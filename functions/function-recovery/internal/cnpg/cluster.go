package cnpg

import (
	"encoding/json"
	"fmt"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	APIVersion  = "postgresql.cnpg.io/v1"
	ClusterKind = "Cluster"
	planDataKey = "manifest.json"
)

func RecoveryPlan(in *model.Input, cluster *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if cluster.GetAPIVersion() != APIVersion || cluster.GetKind() != ClusterKind {
		return nil, fmt.Errorf("required resource is not a CNPG Cluster")
	}
	if cluster.GetName() != in.Spec.Target.Name || cluster.GetNamespace() != in.Spec.Target.Namespace {
		return nil, fmt.Errorf("required CNPG Cluster does not match target")
	}
	manifest := cluster.DeepCopy().Object
	delete(manifest, "status")
	metadata, found, err := unstructured.NestedMap(manifest, "metadata")
	if err != nil || !found {
		return nil, fmt.Errorf("CNPG Cluster does not contain metadata")
	}
	for _, field := range []string{"creationTimestamp", "deletionGracePeriodSeconds", "deletionTimestamp", "generation", "managedFields", "resourceVersion", "selfLink", "uid"} {
		delete(metadata, field)
	}
	if err := unstructured.SetNestedMap(manifest, metadata, "metadata"); err != nil {
		return nil, fmt.Errorf("cannot clean CNPG Cluster metadata: %w", err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot encode CNPG Cluster: %w", err)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      in.Spec.PlanName,
			"namespace": in.Spec.Target.Namespace,
		},
		"data": map[string]interface{}{planDataKey: string(encoded)},
	}}, nil
}

func RestoreCluster(in *model.Input, plan *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	data, found, err := unstructured.NestedStringMap(plan.Object, "data")
	if err != nil || !found || data[planDataKey] == "" {
		return nil, fmt.Errorf("recovery plan does not contain manifest.json")
	}
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(data[planDataKey]), &manifest); err != nil {
		return nil, fmt.Errorf("cannot decode recovery plan: %w", err)
	}
	if manifest["kind"] != ClusterKind || manifest["apiVersion"] != APIVersion {
		return nil, fmt.Errorf("recovery plan does not contain a CNPG Cluster manifest")
	}
	unstructured.RemoveNestedField(manifest, "spec", "bootstrap", "initdb")
	bootstrap, _, _ := unstructured.NestedMap(manifest, "spec", "bootstrap")
	if bootstrap == nil {
		bootstrap = map[string]interface{}{}
	}
	if in.Spec.Backup != nil {
		bootstrap["recovery"] = map[string]interface{}{"backup": map[string]interface{}{"name": in.Spec.Backup.Name}}
	} else {
		bootstrap["recovery"] = map[string]interface{}{"volumeSnapshots": map[string]interface{}{
			"storage":    map[string]interface{}{"storageClass": in.Spec.VolumeSnapshots.StorageClass, "volumeSnapshot": map[string]interface{}{"name": in.Spec.VolumeSnapshots.Data}},
			"walStorage": map[string]interface{}{"volumeSnapshot": map[string]interface{}{"name": in.Spec.VolumeSnapshots.Wal}},
		}}
	}
	if err := unstructured.SetNestedMap(manifest, bootstrap, "spec", "bootstrap"); err != nil {
		return nil, fmt.Errorf("cannot set recovery bootstrap: %w", err)
	}
	cluster := &unstructured.Unstructured{Object: manifest}
	cluster.SetAPIVersion(APIVersion)
	cluster.SetKind(ClusterKind)
	cluster.SetName(in.Spec.Target.Name)
	cluster.SetNamespace(in.Spec.Target.Namespace)
	return cluster, nil
}
