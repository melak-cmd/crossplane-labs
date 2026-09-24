package main

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/input/v1beta1"
)

func TestRenderPlanCopiesClusterManifest(t *testing.T) {
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Target:   v1beta1.ClusterReference{Name: "orders-cluster", Namespace: "platform"},
		PlanName: "orders-recovery-plan",
		Mode:     modePrepare,
	}}
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"forProvider": map[string]interface{}{"manifest": map[string]interface{}{
			"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
			"spec": map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
		}}},
	}}
	plan, err := renderPlan(in, object)
	if err != nil {
		t.Fatalf("renderPlan returned an error: %v", err)
	}
	if plan.GetName() != "orders-recovery-plan" || plan.GetNamespace() != "platform" {
		t.Fatalf("unexpected plan identity: %s/%s", plan.GetNamespace(), plan.GetName())
	}
	if plan.Object["data"].(map[string]interface{})[planDataKey] == "" {
		t.Fatal("plan does not contain the copied manifest")
	}
}

func TestRenderRestoredObjectRemovesInitDBAndAddsBackupRecovery(t *testing.T) {
	manifest := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]interface{}{"name": "orders", "namespace": "platform"},
		"spec":     map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
	}
	encoded, _ := json.Marshal(manifest)
	plan := &unstructured.Unstructured{Object: map[string]interface{}{"data": map[string]interface{}{planDataKey: string(encoded)}}}
	in := &v1beta1.Input{Spec: v1beta1.InputSpec{
		Mode: modeRestore, PlanName: "orders-recovery-plan",
		Target: v1beta1.ClusterReference{Name: "orders-cluster", Namespace: "platform"},
		Backup: &v1beta1.BackupReference{Name: "orders-backup", Namespace: "platform"},
	}}
	object, err := renderRestoredObject(in, plan)
	if err != nil {
		t.Fatalf("renderRestoredObject returned an error: %v", err)
	}
	if object.GetAPIVersion() != "kubernetes.m.crossplane.io/v1alpha1" || object.GetName() != "orders-cluster" {
		t.Fatalf("unexpected provider Object identity: %#v", object.Object)
	}
	if object.GetAnnotations()["crossplane.io/composition-resource-name"] != "cluster" {
		t.Fatal("restored provider Object is missing the composition resource annotation")
	}
	restored, _, _ := unstructured.NestedMap(object.Object, "spec", "forProvider", "manifest")
	bootstrap, _, _ := unstructured.NestedMap(restored, "spec", "bootstrap")
	if _, found, _ := unstructured.NestedMap(bootstrap, "initdb"); found {
		t.Fatal("restored manifest still contains bootstrap.initdb")
	}
	recovery, found, _ := unstructured.NestedMap(bootstrap, "recovery")
	if !found || recovery["backup"].(map[string]interface{})["name"] != "orders-backup" {
		t.Fatalf("unexpected recovery configuration: %#v", recovery)
	}
}

func TestValidatePrepareAndRestoreInputs(t *testing.T) {
	validPrepare := &v1beta1.Input{Spec: v1beta1.InputSpec{Mode: modePrepare, PlanName: "plan", Target: v1beta1.ClusterReference{Name: "object", Namespace: "platform"}}}
	if err := validate(validPrepare); err != nil {
		t.Fatalf("valid prepare input rejected: %v", err)
	}
	validRestore := &v1beta1.Input{Spec: v1beta1.InputSpec{Mode: modeRestore, PlanName: "plan", Target: v1beta1.ClusterReference{Name: "object", Namespace: "platform"}, Backup: &v1beta1.BackupReference{Name: "backup", Namespace: "platform"}}}
	if err := validate(validRestore); err != nil {
		t.Fatalf("valid restore input rejected: %v", err)
	}
	invalid := []*v1beta1.Input{
		{Spec: v1beta1.InputSpec{Mode: "unknown", PlanName: "plan", Target: v1beta1.ClusterReference{Name: "object", Namespace: "platform"}}},
		{Spec: v1beta1.InputSpec{Mode: modePrepare, Target: v1beta1.ClusterReference{Name: "object", Namespace: "platform"}}},
		{Spec: v1beta1.InputSpec{Mode: modeRestore, PlanName: "plan", Target: v1beta1.ClusterReference{Name: "object", Namespace: "platform"}}},
	}
	for _, input := range invalid {
		if err := validate(input); err == nil {
			t.Fatal("expected invalid recovery input to be rejected")
		}
	}
}
