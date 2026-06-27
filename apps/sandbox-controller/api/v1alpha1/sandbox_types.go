package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	SandboxFinalizer = "compute.daytona.io/sandbox"

	LabelSandboxName   = "compute.daytona.io/sandbox"
	LabelManagedBy     = "app.kubernetes.io/managed-by"
	ManagedByValue     = "daytona-sandbox-controller"
	AnnotationSpecHash = "compute.daytona.io/spec-hash"

	GKERestoreSnapshotAnnotation = "podsnapshot.gke.io/ps-name"

	LocalRunscRestoreSnapshotAnnotation   = "compute.daytona.io/local-runsc-snapshot"
	LocalRunscRestoreStorageRefAnnotation = "compute.daytona.io/local-runsc-storage-ref"
)

type SandboxDesiredState string

const (
	SandboxDesiredStateRunning SandboxDesiredState = "Running"
	SandboxDesiredStateStopped SandboxDesiredState = "Stopped"
)

type SandboxPhase string

const (
	SandboxPhasePending   SandboxPhase = "Pending"
	SandboxPhaseStarting  SandboxPhase = "Starting"
	SandboxPhaseRunning   SandboxPhase = "Running"
	SandboxPhaseStopping  SandboxPhase = "Stopping"
	SandboxPhaseStopped   SandboxPhase = "Stopped"
	SandboxPhaseRestoring SandboxPhase = "Restoring"
	SandboxPhaseFailed    SandboxPhase = "Failed"
	SandboxPhaseDeleting  SandboxPhase = "Deleting"
	SandboxPhaseUnknown   SandboxPhase = "Unknown"
)

type SandboxPort struct {
	Name     string          `json:"name"`
	Port     int32           `json:"port"`
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

type SandboxVolumeSpec struct {
	Name                  string                                    `json:"name"`
	MountPath             string                                    `json:"mountPath"`
	ReadOnly              bool                                      `json:"readOnly,omitempty"`
	EmptyDir              *corev1.EmptyDirVolumeSource              `json:"emptyDir,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

type ToolboxSpec struct {
	Image     string                      `json:"image,omitempty"`
	Port      int32                       `json:"port,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type SandboxAccessSpec struct {
	SSHEnabled        bool   `json:"sshEnabled,omitempty"`
	RouteBaseURL      string `json:"routeBaseUrl,omitempty"`
	CredentialVersion string `json:"credentialVersion,omitempty"`
}

type SandboxStopPolicySpec struct {
	SnapshotBeforeStop bool                   `json:"snapshotBeforeStop,omitempty"`
	SnapshotName       string                 `json:"snapshotName,omitempty"`
	AutoStopMinutes    int32                  `json:"autoStopMinutes,omitempty"`
	Provider           SnapshotProvider       `json:"provider,omitempty"`
	GKE                GKEPodSnapshotSpec     `json:"gke,omitempty"`
	Local              LocalRunscProviderSpec `json:"local,omitempty"`
}

type SandboxNetworkPolicySpec struct {
	Enabled     bool     `json:"enabled,omitempty"`
	AllowDNS    bool     `json:"allowDNS,omitempty"`
	EgressCIDRs []string `json:"egressCIDRs,omitempty"`
}

type SandboxSecretsSpec struct {
	Provider          string `json:"provider,omitempty"`
	DopplerProject    string `json:"dopplerProject,omitempty"`
	DopplerConfig     string `json:"dopplerConfig,omitempty"`
	ManagedSecretName string `json:"managedSecretName,omitempty"`
}

type SandboxSnapshotRestoreRef struct {
	Name               string           `json:"name"`
	Provider           SnapshotProvider `json:"provider,omitempty"`
	ProviderObjectName string           `json:"providerObjectName,omitempty"`
	StorageRef         string           `json:"storageRef,omitempty"`
}

type SandboxSchedulingSpec struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
}

type SandboxSpec struct {
	DesiredState       SandboxDesiredState         `json:"desiredState,omitempty"`
	TemplateName       string                      `json:"templateName,omitempty"`
	Image              string                      `json:"image"`
	Command            []string                    `json:"command,omitempty"`
	Args               []string                    `json:"args,omitempty"`
	Env                []corev1.EnvVar             `json:"env,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	Ports              []SandboxPort               `json:"ports,omitempty"`
	Volumes            []SandboxVolumeSpec         `json:"volumes,omitempty"`
	RuntimeClassName   *string                     `json:"runtimeClassName,omitempty"`
	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	Toolbox            ToolboxSpec                 `json:"toolbox,omitempty"`
	Access             SandboxAccessSpec           `json:"access,omitempty"`
	StopPolicy         SandboxStopPolicySpec       `json:"stopPolicy,omitempty"`
	NetworkPolicy      SandboxNetworkPolicySpec    `json:"networkPolicy,omitempty"`
	Secrets            SandboxSecretsSpec          `json:"secrets,omitempty"`
	Scheduling         SandboxSchedulingSpec       `json:"scheduling,omitempty"`
	Restore            *SandboxSnapshotRestoreRef  `json:"restore,omitempty"`
}

type SandboxStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Phase              SandboxPhase       `json:"phase,omitempty"`
	PodName            string             `json:"podName,omitempty"`
	ServiceName        string             `json:"serviceName,omitempty"`
	SpecHash           string             `json:"specHash,omitempty"`
	RestoredSnapshot   string             `json:"restoredSnapshot,omitempty"`
	SleepSnapshotName  string             `json:"sleepSnapshotName,omitempty"`
	LastActivityTime   *metav1.Time       `json:"lastActivityTime,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

type Sandbox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxSpec   `json:"spec,omitempty"`
	Status SandboxStatus `json:"status,omitempty"`
}

type SandboxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sandbox `json:"items"`
}

func (in *Sandbox) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(Sandbox)
	in.DeepCopyInto(out)
	return out
}

func (in *SandboxList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(SandboxList)
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]Sandbox, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

func (in *Sandbox) DeepCopyInto(out *Sandbox) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec.DeepCopy()
	out.Status = in.Status.DeepCopy()
}

func (in SandboxSpec) DeepCopy() SandboxSpec {
	out := in
	out.Command = append([]string(nil), in.Command...)
	out.Args = append([]string(nil), in.Args...)
	out.Resources = *in.Resources.DeepCopy()
	out.Env = make([]corev1.EnvVar, len(in.Env))
	for i := range in.Env {
		in.Env[i].DeepCopyInto(&out.Env[i])
	}
	out.Ports = append([]SandboxPort(nil), in.Ports...)
	out.Volumes = make([]SandboxVolumeSpec, len(in.Volumes))
	for i := range in.Volumes {
		out.Volumes[i] = in.Volumes[i].DeepCopy()
	}
	if in.RuntimeClassName != nil {
		value := *in.RuntimeClassName
		out.RuntimeClassName = &value
	}
	out.Toolbox.Resources = *in.Toolbox.Resources.DeepCopy()
	out.NetworkPolicy.EgressCIDRs = append([]string(nil), in.NetworkPolicy.EgressCIDRs...)
	out.Scheduling.NodeSelector = copyStringMap(in.Scheduling.NodeSelector)
	out.Scheduling.Tolerations = make([]corev1.Toleration, len(in.Scheduling.Tolerations))
	for i := range in.Scheduling.Tolerations {
		in.Scheduling.Tolerations[i].DeepCopyInto(&out.Scheduling.Tolerations[i])
	}
	if in.Scheduling.Affinity != nil {
		out.Scheduling.Affinity = in.Scheduling.Affinity.DeepCopy()
	}
	if in.Restore != nil {
		restore := *in.Restore
		out.Restore = &restore
	}
	return out
}

func (in SandboxVolumeSpec) DeepCopy() SandboxVolumeSpec {
	out := in
	if in.EmptyDir != nil {
		out.EmptyDir = in.EmptyDir.DeepCopy()
	}
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = in.PersistentVolumeClaim.DeepCopy()
	}
	return out
}

func (in SandboxStatus) DeepCopy() SandboxStatus {
	out := in
	if in.LastActivityTime != nil {
		out.LastActivityTime = in.LastActivityTime.DeepCopy()
	}
	out.Conditions = make([]metav1.Condition, len(in.Conditions))
	for i := range in.Conditions {
		out.Conditions[i] = in.Conditions[i]
	}
	return out
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
