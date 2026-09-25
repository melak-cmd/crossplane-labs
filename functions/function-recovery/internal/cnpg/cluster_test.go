package cnpg

import (
	"encoding/json"
	"testing"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/input/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRecoveryPlanCopiesCleanClusterManifest(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target:   v1beta1.ClusterReference{Name: "orders", Namespace: "platform"},
		PlanName: "orders-recovery-plan",
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
	if plan.GetName() != "orders-recovery-plan" || plan.GetNamespace() != "platform" {
		t.Fatalf("unexpected plan identity: %s/%s", plan.GetNamespace(), plan.GetName())
	}
	var saved map[string]interface{}
	encoded := plan.Object["data"].(map[string]interface{})[planDataKey].(string)
	if err := json.Unmarshal([]byte(encoded), &saved); err != nil {
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
	plan := &unstructured.Unstructured{Object: map[string]interface{}{"data": map[string]interface{}{planDataKey: string(encoded)}}}
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
