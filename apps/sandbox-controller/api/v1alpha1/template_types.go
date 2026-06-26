package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SandboxTemplateSpec struct {
	Template SandboxSpec `json:"template"`
}

type SandboxTemplateStatus struct {
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	CompatibilityHash  string `json:"compatibilityHash,omitempty"`
}

type SandboxTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxTemplateSpec   `json:"spec,omitempty"`
	Status SandboxTemplateStatus `json:"status,omitempty"`
}

type SandboxTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SandboxTemplate `json:"items"`
}

func (in *SandboxTemplate) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(SandboxTemplate)
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec.Template = in.Spec.Template.DeepCopy()
	return out
}

func (in *SandboxTemplateList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(SandboxTemplateList)
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]SandboxTemplate, len(in.Items))
		copy(out.Items, in.Items)
	}
	return out
}
