// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=function-pause.fn.kaonix.com
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares the resource to pause or resume.
	Spec InputSpec `json:"spec"`
}

// InputSpec is the spec of the Function's input.
type InputSpec struct {
	// Target identifies the resource preloaded by the Operation.
	Target TargetReference `json:"target"`

	// Paused controls whether Crossplane reconciliation is paused.
	Paused bool `json:"paused"`
}

// TargetReference identifies the resource to update.
type TargetReference struct {
	// APIVersion is the target resource API version.
	// +kubebuilder:validation:MinLength=1
	APIVersion string `json:"apiVersion"`

	// Kind is the target resource kind.
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`

	// Name is the target resource name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace is the target namespace when the resource is namespaced.
	Namespace string `json:"namespace,omitempty"`
}
