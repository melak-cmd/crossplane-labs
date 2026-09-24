// Package v1beta1 contains the input type for this Function.
// +kubebuilder:object:generate=true
// +groupName=function-recovery.fn.kaonix.com
// +versionName=v1beta1
package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InputSpec `json:"spec"`
}

type InputSpec struct {
	Mode            string                  `json:"mode"`
	Target          ClusterReference        `json:"target"`
	PlanName        string                  `json:"planName,omitempty"`
	Backup          *BackupReference        `json:"backup,omitempty"`
	VolumeSnapshots *VolumeSnapshotRecovery `json:"volumeSnapshots,omitempty"`
}

type ClusterReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
}

type BackupReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace,omitempty"`
}

type VolumeSnapshotRecovery struct {
	// +kubebuilder:validation:MinLength=1
	Data string `json:"data"`
	// +kubebuilder:validation:MinLength=1
	Wal string `json:"wal"`
	// +kubebuilder:validation:MinLength=1
	StorageClass string `json:"storageClass"`
}
