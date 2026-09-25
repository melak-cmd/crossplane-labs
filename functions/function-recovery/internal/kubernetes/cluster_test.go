package kubernetes

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestDeleteAndWaitDeletesNamespacedCluster(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform"},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster)
	deleter := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	if err := deleter.DeleteAndWait(context.Background(), "platform", "orders"); err != nil {
		t.Fatalf("DeleteAndWait returned an error: %v", err)
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected Cluster to be deleted, got error: %v", err)
	}
}

func TestGetClusterUsesReferencedName(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster)
	clusterClient := &ClusterClient{client: client}
	got, err := clusterClient.GetCluster(context.Background(), "platform", "orders-primary")
	if err != nil {
		t.Fatalf("GetCluster returned an error: %v", err)
	}
	if got.GetName() != "orders-primary" {
		t.Fatalf("GetCluster returned %q, want %q", got.GetName(), "orders-primary")
	}
}

func TestCreateRestoredClusterCreatesMissingCluster(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"spec":       map[string]interface{}{"instances": int64(1)},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.CreateRestoredCluster(context.Background(), cluster); err != nil {
		t.Fatalf("CreateRestoredCluster returned an error: %v", err)
	}
	created, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected restored Cluster to be created: %v", err)
	}
	if instances, found, _ := unstructured.NestedInt64(created.Object, "spec", "instances"); !found || instances != 1 {
		t.Fatalf("unexpected restored Cluster spec: %#v", created.Object["spec"])
	}
}

func TestCreateRestoredClusterAcceptsMatchingRetry(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"spec": map[string]interface{}{"bootstrap": map[string]interface{}{
			"recovery": map[string]interface{}{"backup": map[string]interface{}{"name": "orders-backup"}},
		}},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster.DeepCopy())
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.CreateRestoredCluster(context.Background(), cluster); err != nil {
		t.Fatalf("CreateRestoredCluster returned an error for an identical retry: %v", err)
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected restored Cluster to remain present: %v", err)
	}
}

func TestCreateRestoredClusterRejectsExistingNonRecoveryCluster(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"spec":       map[string]interface{}{"instances": int64(1)},
	}}
	desired := existing.DeepCopy()
	if err := unstructured.SetNestedField(desired.Object, map[string]interface{}{"backup": map[string]interface{}{"name": "orders-backup"}}, "spec", "bootstrap", "recovery"); err != nil {
		t.Fatalf("failed to set desired recovery bootstrap: %v", err)
	}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), existing)
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.CreateRestoredCluster(context.Background(), desired); err == nil {
		t.Fatal("expected an existing Cluster without matching recovery bootstrap to be rejected")
	}
	unchanged, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read existing Cluster: %v", err)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(unchanged.Object, "spec", "bootstrap", "recovery"); found {
		t.Fatal("existing Cluster must not be updated with recovery bootstrap")
	}
}

func TestDeleteAndWaitIsIdempotent(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	deleter := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	if err := deleter.DeleteAndWait(context.Background(), "platform", "missing"); err != nil {
		t.Fatalf("DeleteAndWait returned an error for an absent Cluster: %v", err)
	}
}

func TestRemoveRecoveryDeletesOnlyRecoveryBootstrap(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform"},
		"spec": map[string]interface{}{
			"bootstrap": map[string]interface{}{
				"initdb":   map[string]interface{}{"database": "orders"},
				"recovery": map[string]interface{}{"backup": map[string]interface{}{"name": "orders-backup"}},
			},
		},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster)
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.RemoveRecovery(context.Background(), "platform", "orders"); err != nil {
		t.Fatalf("RemoveRecovery returned an error: %v", err)
	}
	updated, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read Cluster after cleanup: %v", err)
	}
	if _, found, err := unstructured.NestedFieldNoCopy(updated.Object, "spec", "bootstrap", "recovery"); err != nil || found {
		t.Fatalf("expected bootstrap.recovery to be removed, found=%t err=%v", found, err)
	}
	if _, found, err := unstructured.NestedFieldNoCopy(updated.Object, "spec", "bootstrap", "initdb"); err != nil || !found {
		t.Fatalf("expected bootstrap.initdb to remain, found=%t err=%v", found, err)
	}
}

func TestRemoveRecoveryIsIdempotent(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform"},
		"spec":       map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster)
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.RemoveRecovery(context.Background(), "platform", "orders"); err != nil {
		t.Fatalf("RemoveRecovery returned an error when recovery was already absent: %v", err)
	}
}

func TestPrepareRecoveryPausesAndPersistsPlanWithoutDeletingCluster(t *testing.T) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.kaonix.inc.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata":   map[string]interface{}{"name": "orders-database", "namespace": "platform"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
	}}
	plan := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "orders-recovery-plan", "namespace": "platform"},
		"data":       map[string]interface{}{"manifest.json": `{"kind":"Cluster"}`},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster)
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.PrepareRecovery(context.Background(), "platform", "orders-database", plan); err != nil {
		t.Fatalf("PrepareRecovery returned an error: %v", err)
	}
	pausedDatabase, err := client.Resource(postgresqlGVR).Namespace("platform").Get(context.Background(), "orders-database", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read Database: %v", err)
	}
	if pausedDatabase.GetAnnotations()["crossplane.io/paused"] != "true" {
		t.Fatal("expected Database to be paused")
	}
	if _, err := client.Resource(configMapGVR).Namespace("platform").Get(context.Background(), "orders-recovery-plan", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected recovery plan to be persisted: %v", err)
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected Cluster to remain until the next Operation step: %v", err)
	}
}

func TestPrepareAndDeletePausesPersistsPlanAndDeletesCluster(t *testing.T) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.kaonix.inc.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata":   map[string]interface{}{"name": "orders-database", "namespace": "platform"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"spec":       map[string]interface{}{"bootstrap": map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}},
	}}
	plan := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "orders-recovery-plan", "namespace": "platform"},
		"data":       map[string]interface{}{"manifest.json": `{"kind":"Cluster"}`},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster)
	clusterClient := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	if err := clusterClient.PrepareAndDelete(context.Background(), "platform", "orders-database", "platform", "orders-primary", plan); err != nil {
		t.Fatalf("PrepareAndDelete returned an error: %v", err)
	}
	pausedDatabase, err := client.Resource(postgresqlGVR).Namespace("platform").Get(context.Background(), "orders-database", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read Database: %v", err)
	}
	if pausedDatabase.GetAnnotations()["crossplane.io/paused"] != "true" {
		t.Fatal("expected Database to be paused")
	}
	if _, err := client.Resource(configMapGVR).Namespace("platform").Get(context.Background(), "orders-recovery-plan", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected recovery plan to be persisted before deletion: %v", err)
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected Cluster to be deleted, got error: %v", err)
	}
}
