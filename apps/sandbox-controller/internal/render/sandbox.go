package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultRuntimeClassName = "gvisor"
	WorkloadContainerName   = "workload"
	CompatibilityVersion    = "workload-only-v2"
)

func DesiredState(sandbox *computev1.Sandbox) computev1.SandboxDesiredState {
	if sandbox.Spec.DesiredState == "" {
		return computev1.SandboxDesiredStateRunning
	}
	return sandbox.Spec.DesiredState
}

func Labels(sandbox *computev1.Sandbox) map[string]string {
	return map[string]string{
		computev1.LabelManagedBy:   computev1.ManagedByValue,
		computev1.LabelSandboxName: sandbox.Name,
		"app.kubernetes.io/name":   "daytona-sandbox",
	}
}

func PodName(sandbox *computev1.Sandbox) string {
	return "sandbox-" + sandbox.Name
}

func ServiceName(sandbox *computev1.Sandbox) string {
	return "sandbox-" + sandbox.Name
}

func NetworkPolicyName(sandbox *computev1.Sandbox) string {
	return "sandbox-" + sandbox.Name
}

func RuntimeClassName(sandbox *computev1.Sandbox) string {
	if sandbox.Spec.RuntimeClassName != nil && *sandbox.Spec.RuntimeClassName != "" {
		return *sandbox.Spec.RuntimeClassName
	}
	return DefaultRuntimeClassName
}

func CompatibilityHash(sandbox *computev1.Sandbox) (string, error) {
	spec := sandbox.Spec.DeepCopy()
	spec.DesiredState = ""
	spec.TemplateName = ""
	spec.StopPolicy = computev1.SandboxStopPolicySpec{}
	spec.Restore = nil
	normalizeEnv(spec.Env)
	sort.Slice(spec.Ports, func(i, j int) bool {
		if spec.Ports[i].Name == spec.Ports[j].Name {
			return spec.Ports[i].Port < spec.Ports[j].Port
		}
		return spec.Ports[i].Name < spec.Ports[j].Name
	})
	data, err := json.Marshal(struct {
		Version string                `json:"version"`
		Spec    computev1.SandboxSpec `json:"spec"`
	}{
		Version: CompatibilityVersion,
		Spec:    spec,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func Pod(sandbox *computev1.Sandbox, _ string) (*corev1.Pod, string, error) {
	if sandbox.Spec.Image == "" {
		return nil, "", fmt.Errorf("sandbox image is required")
	}

	specHash, err := CompatibilityHash(sandbox)
	if err != nil {
		return nil, "", err
	}

	labels := Labels(sandbox)
	annotations := map[string]string{computev1.AnnotationSpecHash: specHash}
	if sandbox.Spec.Restore != nil && sandbox.Spec.Restore.ProviderObjectName != "" {
		switch sandbox.Spec.Restore.Provider {
		case computev1.SnapshotProviderLocalRunsc:
			annotations[computev1.LocalRunscRestoreSnapshotAnnotation] = sandbox.Spec.Restore.ProviderObjectName
			if sandbox.Spec.Restore.StorageRef != "" {
				annotations[computev1.LocalRunscRestoreStorageRefAnnotation] = sandbox.Spec.Restore.StorageRef
			}
		default:
			annotations[computev1.GKERestoreSnapshotAnnotation] = sandbox.Spec.Restore.ProviderObjectName
		}
	}

	runtimeClassName := RuntimeClassName(sandbox)
	enableServiceLinks := false

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PodName(sandbox),
			Namespace:   sandbox.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			RuntimeClassName:   &runtimeClassName,
			ServiceAccountName: sandbox.Spec.ServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			EnableServiceLinks: &enableServiceLinks,
			NodeSelector:       copyStringMap(sandbox.Spec.Scheduling.NodeSelector),
			Tolerations:        append([]corev1.Toleration(nil), sandbox.Spec.Scheduling.Tolerations...),
			Affinity:           sandbox.Spec.Scheduling.Affinity,
			Containers: []corev1.Container{
				{
					Name:         WorkloadContainerName,
					Image:        sandbox.Spec.Image,
					Command:      append([]string(nil), sandbox.Spec.Command...),
					Args:         append([]string(nil), sandbox.Spec.Args...),
					Env:          workloadEnv(sandbox),
					EnvFrom:      workloadEnvFrom(sandbox),
					Resources:    *sandbox.Spec.Resources.DeepCopy(),
					Ports:        workloadContainerPorts(sandbox),
					VolumeMounts: workloadVolumeMounts(sandbox),
				},
			},
			Volumes: sandboxVolumes(sandbox),
		},
	}

	return pod, specHash, nil
}

func HasPersistentVolumeClaim(sandbox *computev1.Sandbox) bool {
	for _, volume := range sandbox.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			return true
		}
	}
	return false
}

func workloadEnv(sandbox *computev1.Sandbox) []corev1.EnvVar {
	env := append([]corev1.EnvVar(nil), sandbox.Spec.Env...)
	env = append(env, corev1.EnvVar{Name: "DAYTONA_CONTAINER_NAME", Value: WorkloadContainerName})
	return env
}

func workloadEnvFrom(sandbox *computev1.Sandbox) []corev1.EnvFromSource {
	if sandbox.Spec.Secrets.Provider != "doppler" && sandbox.Spec.Secrets.DopplerProject == "" && sandbox.Spec.Secrets.DopplerConfig == "" {
		return nil
	}
	secretName := sandbox.Spec.Secrets.ManagedSecretName
	if secretName == "" {
		secretName = "sandbox-" + sandbox.Name + "-doppler"
	}
	return []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		},
	}}
}

func Service(sandbox *computev1.Sandbox) *corev1.Service {
	ports := make([]corev1.ServicePort, 0, len(sandbox.Spec.Ports))
	for _, port := range sandbox.Spec.Ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		ports = append(ports, corev1.ServicePort{
			Name:       port.Name,
			Port:       port.Port,
			TargetPort: intstr.FromString(port.Name),
			Protocol:   protocol,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName(sandbox),
			Namespace: sandbox.Namespace,
			Labels:    Labels(sandbox),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: Labels(sandbox),
			Ports:    ports,
		},
	}
}

func NetworkPolicy(sandbox *computev1.Sandbox) *networkingv1.NetworkPolicy {
	labels := Labels(sandbox)
	egressRules := make([]networkingv1.NetworkPolicyEgressRule, 0, len(sandbox.Spec.NetworkPolicy.EgressCIDRs)+1)

	if sandbox.Spec.NetworkPolicy.AllowDNS {
		egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolUDP), Port: intstrPtr(intstr.FromInt32(53))},
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: intstrPtr(intstr.FromInt32(53))},
			},
		})
	}

	for _, cidr := range sandbox.Spec.NetworkPolicy.EgressCIDRs {
		egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				IPBlock: &networkingv1.IPBlock{CIDR: cidr},
			}},
		})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyName(sandbox),
			Namespace: sandbox.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: labels},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: egressRules,
		},
	}
}

func workloadContainerPorts(sandbox *computev1.Sandbox) []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0, len(sandbox.Spec.Ports))
	for _, port := range sandbox.Spec.Ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          port.Name,
			ContainerPort: port.Port,
			Protocol:      protocol,
		})
	}
	return ports
}

func workloadVolumeMounts(sandbox *computev1.Sandbox) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(sandbox.Spec.Volumes))
	for _, volume := range sandbox.Spec.Volumes {
		if volume.Name == "" || volume.MountPath == "" {
			continue
		}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
		})
	}
	return mounts
}

func sandboxVolumes(sandbox *computev1.Sandbox) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(sandbox.Spec.Volumes))
	for _, volume := range sandbox.Spec.Volumes {
		if volume.Name == "" {
			continue
		}
		source := corev1.VolumeSource{}
		if volume.EmptyDir != nil {
			source.EmptyDir = volume.EmptyDir.DeepCopy()
		}
		if volume.PersistentVolumeClaim != nil {
			source.PersistentVolumeClaim = volume.PersistentVolumeClaim.DeepCopy()
		}
		volumes = append(volumes, corev1.Volume{
			Name:         volume.Name,
			VolumeSource: source,
		})
	}
	return volumes
}

func normalizeEnv(values []corev1.EnvVar) {
	sort.Slice(values, func(i, j int) bool {
		return values[i].Name < values[j].Name
	})
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func protocolPtr(value corev1.Protocol) *corev1.Protocol {
	return &value
}

func intstrPtr(value intstr.IntOrString) *intstr.IntOrString {
	return &value
}
