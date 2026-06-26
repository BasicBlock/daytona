package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SnapshotProvider string

const (
	LocalRunscSnapshotFinalizer = "compute.daytona.io/local-runsc-snapshot"
	SandboxSnapshotFinalizer    = "compute.daytona.io/sandbox-snapshot"

	SnapshotProviderGKEPodSnapshot SnapshotProvider = "GKEPodSnapshot"
	SnapshotProviderLocalRunsc     SnapshotProvider = "LocalRunsc"
)

type PostCheckpointAction string

const (
	PostCheckpointResume PostCheckpointAction = "resume"
	PostCheckpointStop   PostCheckpointAction = "stop"
)

type SandboxSnapshotPhase string

const (
	SandboxSnapshotPhasePending    SandboxSnapshotPhase = "Pending"
	SandboxSnapshotPhaseTriggering SandboxSnapshotPhase = "Triggering"
	SandboxSnapshotPhaseReady      SandboxSnapshotPhase = "Ready"
	SandboxSnapshotPhaseFailed     SandboxSnapshotPhase = "Failed"
)

type SandboxSnapshotSourceRef struct {
	SandboxName string `json:"sandboxName"`
}

type GKEPodSnapshotSpec struct {
	StorageConfigName string                `json:"storageConfigName"`
	Storage           GKEPodSnapshotStorage `json:"storage,omitempty"`
	PostCheckpoint    PostCheckpointAction  `json:"postCheckpoint,omitempty"`
	Retention         string                `json:"retention,omitempty"`
}

type GKEPodSnapshotStorage struct {
	Bucket      string `json:"bucket,omitempty"`
	Path        string `json:"path,omitempty"`
	TokenSource string `json:"tokenSource,omitempty"`
}

type LocalRunscProviderSpec struct {
	Storage LocalRunscStorageSpec `json:"storage,omitempty"`
}

type LocalRunscStorageSpec struct {
	Mode     string `json:"mode,omitempty"`
	Path     string `json:"path,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
}

type SandboxSnapshotSpec struct {
	Provider SnapshotProvider         `json:"provider,omitempty"`
	Source   SandboxSnapshotSourceRef `json:"source"`
	GKE      GKEPodSnapshotSpec       `json:"gke,omitempty"`
	Local    LocalRunscProviderSpec   `json:"local,omitempty"`
}

type SandboxSnapshotStatus struct {
	ObservedGeneration int64                `json:"observedGeneration,omitempty"`
	Phase              SandboxSnapshotPhase `json:"phase,omitempty"`
	ProviderObjectName string               `json:"providerObjectName,omitempty"`
	PolicyName         string               `json:"policyName,omitempty"`
	TriggerName        string               `json:"triggerName,omitempty"`
	SourcePodName      string               `json:"sourcePodName,omitempty"`
	TemplateName       string               `json:"templateName,omitempty"`
	CompatibilityHash  string               `json:"compatibilityHash,omitempty"`
	StorageRef         string               `json:"storageRef,omitempty"`
	Error              string               `json:"error,omitempty"`
	Conditions         []metav1.Condition   `json:"conditions,omitempty"`
}

type SandboxSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxSnapshotSpec   `json:"spec,omitempty"`
	Status SandboxSnapshotStatus `json:"status,omitempty"`
}

type SandboxSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SandboxSnapshot `json:"items"`
}

func (in *SandboxSnapshot) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(SandboxSnapshot)
	in.DeepCopyInto(out)
	return out
}

func (in *SandboxSnapshotList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(SandboxSnapshotList)
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]SandboxSnapshot, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

func (in *SandboxSnapshot) DeepCopyInto(out *SandboxSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec
	out.Status = in.Status.DeepCopy()
}

func (in SandboxSnapshotStatus) DeepCopy() SandboxSnapshotStatus {
	out := in
	out.Conditions = make([]metav1.Condition, len(in.Conditions))
	for i := range in.Conditions {
		out.Conditions[i] = in.Conditions[i]
	}
	return out
}

type LocalRunscSnapshotPhase string

const (
	LocalRunscSnapshotPhasePending LocalRunscSnapshotPhase = "Pending"
	LocalRunscSnapshotPhaseRunning LocalRunscSnapshotPhase = "Running"
	LocalRunscSnapshotPhaseReady   LocalRunscSnapshotPhase = "Ready"
	LocalRunscSnapshotPhaseFailed  LocalRunscSnapshotPhase = "Failed"
)

type LocalRunscSnapshotSpec struct {
	SandboxName         string                `json:"sandboxName"`
	SourcePodName       string                `json:"sourcePodName"`
	SourceContainerName string                `json:"sourceContainerName,omitempty"`
	NodeName            string                `json:"nodeName"`
	Storage             LocalRunscStorageSpec `json:"storage,omitempty"`
}

type LocalRunscSnapshotStatus struct {
	ObservedGeneration int64                   `json:"observedGeneration,omitempty"`
	Phase              LocalRunscSnapshotPhase `json:"phase,omitempty"`
	StorageRef         string                  `json:"storageRef,omitempty"`
	NodeName           string                  `json:"nodeName,omitempty"`
	Error              string                  `json:"error,omitempty"`
	Conditions         []metav1.Condition      `json:"conditions,omitempty"`
}

type LocalRunscSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalRunscSnapshotSpec   `json:"spec,omitempty"`
	Status LocalRunscSnapshotStatus `json:"status,omitempty"`
}

type LocalRunscSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LocalRunscSnapshot `json:"items"`
}

func (in *LocalRunscSnapshot) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(LocalRunscSnapshot)
	in.DeepCopyInto(out)
	return out
}

func (in *LocalRunscSnapshotList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(LocalRunscSnapshotList)
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]LocalRunscSnapshot, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

func (in *LocalRunscSnapshot) DeepCopyInto(out *LocalRunscSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec
	out.Status = in.Status.DeepCopy()
}

func (in LocalRunscSnapshotStatus) DeepCopy() LocalRunscSnapshotStatus {
	out := in
	out.Conditions = make([]metav1.Condition, len(in.Conditions))
	for i := range in.Conditions {
		out.Conditions[i] = in.Conditions[i]
	}
	return out
}
