// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=function-scale.fn.kaonix.com
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

	// Spec declares the scale targets for this run.
	Spec InputSpec `json:"spec"`
}

// InputSpec is the spec of the Function's input.
type InputSpec struct {
	// ScaleTargets lists the apps (by desired resource name) and the replica
	// counts to apply to each.
	ScaleTargets []ScaleTarget `json:"scaleTargets,omitempty"`
}

// ScaleTarget pairs an app name with the replica count to scale it to.
type ScaleTarget struct {
	// Name is the metadata.name of the desired composed resource to scale.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Replicas is the value to set on the matching resource's spec.replicas.
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas"`
}
