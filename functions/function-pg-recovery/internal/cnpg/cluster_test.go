package cnpg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/input/v1beta1"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRecoveryPlanCopiesCleanClusterManifest(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": APIVersion,
		"kind":       ClusterKind,
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform", "resourceVersion": "9"},
		"spec":       map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
		"status":     map[string]interface{}{"phase": "healthy"},
	}}
	plan, err := RecoveryPlan(in, cluster)
	if err != nil {
		t.Fatalf("RecoveryPlan returned an error: %v", err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal([]byte(plan), &saved); err != nil {
		t.Fatalf("cannot decode saved manifest: %v", err)
	}
	if _, found := saved["status"]; found {
		t.Fatal("recovery plan must not contain status")
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(saved, "metadata", "resourceVersion"); found {
		t.Fatal("recovery plan must not contain resourceVersion")
	}
}

func TestRecoveryPlanRejectsMismatchedCluster(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"}}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": APIVersion,
		"kind":       ClusterKind,
		"metadata":   map[string]interface{}{"name": "payments", "namespace": "platform"},
	}}
	if _, err := RecoveryPlan(in, cluster); err == nil {
		t.Fatal("expected mismatched target to be rejected")
	}
}

func TestRestoreClusterRemovesInitDBAndAddsBackupRecovery(t *testing.T) {
	manifest := map[string]interface{}{
		"apiVersion": APIVersion,
		"kind":       ClusterKind,
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform"},
		"spec":       map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
	}
	encoded, _ := json.Marshal(manifest)
	plan := restoreWithPlan(string(encoded))
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
		Backup: &v1beta1.BackupReference{Name: "orders-backup", Namespace: "platform"},
	}}
	cluster, err := RestoreCluster(in, plan)
	if err != nil {
		t.Fatalf("RestoreCluster returned an error: %v", err)
	}
	if cluster.GetAPIVersion() != APIVersion || cluster.GetKind() != ClusterKind || cluster.GetName() != "orders" || cluster.GetNamespace() != "platform" {
		t.Fatalf("unexpected restored Cluster identity: %#v", cluster.Object)
	}
	bootstrap, _, _ := unstructured.NestedMap(cluster.Object, "spec", "bootstrap")
	if _, found, _ := unstructured.NestedMap(bootstrap, "initdb"); found {
		t.Fatal("restored Cluster still contains bootstrap.initdb")
	}
	recovery, found, _ := unstructured.NestedMap(bootstrap, "recovery")
	if !found || recovery["backup"].(map[string]interface{})["name"] != "orders-backup" {
		t.Fatalf("unexpected recovery configuration: %#v", recovery)
	}
}

func restoreWithPlan(plan string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQLRestore",
		"metadata": map[string]interface{}{
			"name": "orders-restore", "namespace": "platform",
			"annotations": map[string]interface{}{model.PlanAnnotation: plan},
		},
	}}
}

func TestRestoreClusterRoundTripsTheRecoveryPlan(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
		Backup: &v1beta1.BackupReference{Name: "orders-backup", Namespace: "platform"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": APIVersion,
		"kind":       ClusterKind,
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform", "uid": "abc"},
		"spec":       map[string]interface{}{"instances": int64(3), "bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
	}}
	plan, err := RecoveryPlan(in, cluster)
	if err != nil {
		t.Fatalf("RecoveryPlan returned an error: %v", err)
	}
	restored, err := RestoreCluster(in, restoreWithPlan(plan))
	if err != nil {
		t.Fatalf("RestoreCluster returned an error: %v", err)
	}
	if instances, _, _ := unstructured.NestedFloat64(restored.Object, "spec", "instances"); instances != 3 {
		t.Fatalf("plan did not carry spec.instances: %#v", restored.Object["spec"])
	}
}

func TestRestoreClusterRejectsMissingPlanAnnotation(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
		Backup: &v1beta1.BackupReference{Name: "orders-backup", Namespace: "platform"},
	}}
	restore := restoreWithPlan("")
	unstructured.RemoveNestedField(restore.Object, "metadata", "annotations")
	_, err := RestoreCluster(in, restore)
	if err == nil || !strings.Contains(err.Error(), "no recovery plan found on PostgreSQLRestore platform/orders-restore") {
		t.Fatalf("expected a missing plan error, got %v", err)
	}
}

func TestRestoreClusterRejectsInvalidPlan(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target: v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
		Backup: &v1beta1.BackupReference{Name: "orders-backup", Namespace: "platform"},
	}}
	if _, err := RestoreCluster(in, restoreWithPlan("not json")); err == nil || !strings.Contains(err.Error(), "cannot decode recovery plan") {
		t.Fatalf("expected a decode error, got %v", err)
	}
	if _, err := RestoreCluster(in, restoreWithPlan(`{"kind":"ConfigMap","apiVersion":"v1"}`)); err == nil {
		t.Fatal("expected a non-Cluster plan to be rejected")
	}
}
