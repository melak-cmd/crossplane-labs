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
)

// RecoveryPlan returns the JSON encoding of a sanitized copy of the CNPG
// Cluster manifest. It is stored as an annotation on the PostgreSQLRestore.
func RecoveryPlan(in *model.Input, cluster *unstructured.Unstructured) (string, error) {
	if cluster.GetAPIVersion() != APIVersion || cluster.GetKind() != ClusterKind {
		return "", fmt.Errorf("required resource is not a CNPG Cluster")
	}
	if cluster.GetName() != in.Spec.Target.Name || cluster.GetNamespace() != in.Spec.Target.Namespace {
		return "", fmt.Errorf("required CNPG Cluster does not match target")
	}
	manifest := cluster.DeepCopy().Object
	delete(manifest, "status")
	metadata, found, err := unstructured.NestedMap(manifest, "metadata")
	if err != nil || !found {
		return "", fmt.Errorf("CNPG Cluster does not contain metadata")
	}
	for _, field := range []string{"creationTimestamp", "deletionGracePeriodSeconds", "deletionTimestamp", "generation", "managedFields", "resourceVersion", "selfLink", "uid"} {
		delete(metadata, field)
	}
	if err := unstructured.SetNestedMap(manifest, metadata, "metadata"); err != nil {
		return "", fmt.Errorf("cannot clean CNPG Cluster metadata: %w", err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("cannot encode CNPG Cluster: %w", err)
	}
	return string(encoded), nil
}

// RestoreCluster rebuilds the CNPG Cluster from the plan stored on the
// PostgreSQLRestore.
func RestoreCluster(in *model.Input, restore *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if in.Spec.Backup == nil {
		return nil, fmt.Errorf("restore mode requires a Backup reference")
	}
	plan := restore.GetAnnotations()[model.PlanAnnotation]
	if plan == "" {
		return nil, fmt.Errorf("no recovery plan found on PostgreSQLRestore %s/%s", restore.GetNamespace(), restore.GetName())
	}
	manifest := map[string]interface{}{}
	if err := json.Unmarshal([]byte(plan), &manifest); err != nil {
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
	bootstrap["recovery"] = map[string]interface{}{"backup": map[string]interface{}{"name": in.Spec.Backup.Name}}
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
