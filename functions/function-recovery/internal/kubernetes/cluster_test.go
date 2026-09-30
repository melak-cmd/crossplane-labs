package kubernetes

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
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

func TestGetPostgreSQLRestoreUsesNameAndNamespace(t *testing.T) {
	restore := newRestore("orders-restore", nil)
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), restore)
	clusterClient := &ClusterClient{client: client}

	got, err := clusterClient.GetPostgreSQLRestore(context.Background(), "platform", "orders-restore")
	if err != nil {
		t.Fatalf("GetPostgreSQLRestore returned an error: %v", err)
	}
	if got.GetName() != "orders-restore" || got.GetNamespace() != "platform" {
		t.Fatalf("GetPostgreSQLRestore returned %s/%s, want platform/orders-restore", got.GetNamespace(), got.GetName())
	}
}

func TestGetPostgreSQLUsesNameAndNamespace(t *testing.T) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata":   map[string]interface{}{"name": "orders", "namespace": "platform"},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database)
	clusterClient := &ClusterClient{client: client}

	got, err := clusterClient.GetPostgreSQL(context.Background(), "platform", "orders")
	if err != nil {
		t.Fatalf("GetPostgreSQL returned an error: %v", err)
	}
	if got.GetName() != "orders" || got.GetNamespace() != "platform" {
		t.Fatalf("GetPostgreSQL returned %s/%s, want platform/orders", got.GetNamespace(), got.GetName())
	}
}

func TestResumePostgreSQLClearsPausedState(t *testing.T) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata": map[string]interface{}{
			"name": "orders", "namespace": "platform",
			"annotations": map[string]interface{}{"crossplane.io/paused": "true"},
		},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database)
	clusterClient := &ClusterClient{client: client}

	if err := clusterClient.ResumePostgreSQL(context.Background(), "platform", "orders"); err != nil {
		t.Fatalf("ResumePostgreSQL returned an error: %v", err)
	}
	updated, err := client.Resource(postgresqlGVR).Namespace("platform").Get(context.Background(), "orders", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read PostgreSQL XR after resume: %v", err)
	}
	if got := updated.GetAnnotations()["crossplane.io/paused"]; got != "false" {
		t.Fatalf("expected paused annotation to be false, got %q", got)
	}
}

func TestCreateRestoredClusterCreatesMissingCluster(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"spec":       map[string]interface{}{"instances": int64(1)},
		"status":     map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}},
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

func TestCreateRestoredClusterWaitsForReady(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"status":     map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "False"}}},
	}}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	clusterClient := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := clusterClient.CreateRestoredCluster(ctx, cluster); err == nil {
		t.Fatal("expected restoring a Cluster without Ready=True to time out")
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
		"status": map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}},
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

func TestWaitForReadyRequiresReadyTrueCondition(t *testing.T) {
	clusters := fake.NewSimpleDynamicClient(runtime.NewScheme()).Resource(clusterGVR).Namespace("platform")
	client := &ClusterClient{interval: time.Millisecond, timeout: time.Second}
	notReady := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
		"status":     map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "False"}}},
	}}
	if _, err := clusters.Create(context.Background(), notReady, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create test Cluster: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := client.waitForReady(ctx, clusters, "orders-primary"); err == nil {
		t.Fatal("expected a Cluster without Ready=True to time out")
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

var ordersRestore = model.RestoreRef{Namespace: "platform", Name: "orders-restore"}

func newRestore(name string, annotations map[string]interface{}) *unstructured.Unstructured {
	metadata := map[string]interface{}{"name": name, "namespace": "platform"}
	if annotations != nil {
		metadata["annotations"] = annotations
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQLRestore",
		"metadata":   metadata,
		"spec":       map[string]interface{}{"name": "orders-database", "backupName": "orders-backup"},
	}}
}

func prepareFixtures() (*unstructured.Unstructured, *unstructured.Unstructured) {
	database := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "database.nuagik.sncf.fr/v1alpha1",
		"kind":       "PostgreSQL",
		"metadata":   map[string]interface{}{"name": "orders-database", "namespace": "platform"},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]interface{}{"name": "orders-primary", "namespace": "platform"},
	}}
	return database, cluster
}

func TestPrepareRecoveryPausesAndStoresPlanOnRestoreWithoutDeletingCluster(t *testing.T) {
	database, cluster := prepareFixtures()
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster, newRestore("orders-restore", nil))
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.PrepareRecovery(context.Background(), ordersRestore, "platform", "orders-database", `{"kind":"Cluster"}`); err != nil {
		t.Fatalf("PrepareRecovery returned an error: %v", err)
	}
	pausedDatabase, err := client.Resource(postgresqlGVR).Namespace("platform").Get(context.Background(), "orders-database", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read Database: %v", err)
	}
	if pausedDatabase.GetAnnotations()["crossplane.io/paused"] != "true" {
		t.Fatal("expected Database to be paused")
	}
	restore, err := client.Resource(postgresqlRestoreGVR).Namespace("platform").Get(context.Background(), "orders-restore", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to read PostgreSQLRestore: %v", err)
	}
	if got := restore.GetAnnotations()[model.PlanAnnotation]; got != `{"kind":"Cluster"}` {
		t.Fatalf("expected the plan annotation to be stored, got %q", got)
	}
	if got := restore.GetAnnotations()[model.PhaseAnnotation]; got != string(model.PhasePrepared) {
		t.Fatalf("expected phase %q, got %q", model.PhasePrepared, got)
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected Cluster to remain until the next Operation step: %v", err)
	}
}

func TestPrepareRecoveryDoesNotCreateAConfigMap(t *testing.T) {
	database, cluster := prepareFixtures()
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster, newRestore("orders-restore", nil))
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.PrepareRecovery(context.Background(), ordersRestore, "platform", "orders-database", `{"kind":"Cluster"}`); err != nil {
		t.Fatalf("PrepareRecovery returned an error: %v", err)
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "configmaps" {
			t.Fatalf("unexpected ConfigMap access: %s", action.GetVerb())
		}
	}
}

func TestPrepareRecoveryRejectsOversizedPlanBeforeChangingAnything(t *testing.T) {
	database, cluster := prepareFixtures()
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster, newRestore("orders-restore", nil))
	clusterClient := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	oversized := strings.Repeat("x", maxAnnotationBytes)
	err := clusterClient.PrepareRecovery(context.Background(), ordersRestore, "platform", "orders-database", oversized)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected a too large error, got %v", err)
	}
	pausedDatabase, getErr := client.Resource(postgresqlGVR).Namespace("platform").Get(context.Background(), "orders-database", metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("failed to read Database: %v", getErr)
	}
	if _, paused := pausedDatabase.GetAnnotations()["crossplane.io/paused"]; paused {
		t.Fatal("Database must not be paused when the plan is rejected")
	}
	restore, _ := client.Resource(postgresqlRestoreGVR).Namespace("platform").Get(context.Background(), "orders-restore", metav1.GetOptions{})
	if _, found := restore.GetAnnotations()[model.PlanAnnotation]; found {
		t.Fatal("PostgreSQLRestore must not be modified when the plan is rejected")
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); err != nil {
		t.Fatalf("Cluster must remain when the plan is rejected: %v", err)
	}
}

func TestCheckAnnotationBudgetCountsExistingAnnotations(t *testing.T) {
	existing := map[string]string{"kubectl.kubernetes.io/last-applied-configuration": strings.Repeat("y", maxAnnotationBytes-100)}
	if err := checkAnnotationBudget(existing, map[string]string{"k": "small"}); err != nil {
		t.Fatalf("expected a small addition to fit: %v", err)
	}
	if err := checkAnnotationBudget(existing, map[string]string{"k": strings.Repeat("z", 200)}); err == nil {
		t.Fatal("expected existing annotations to count against the budget")
	}
}

func TestSetRestorePhaseOnlyPatchesThePhaseAnnotation(t *testing.T) {
	restore := newRestore("orders-restore", map[string]interface{}{
		model.PlanAnnotation:  `{"kind":"Cluster"}`,
		model.PhaseAnnotation: string(model.PhasePrepared),
	})
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), restore)
	clusterClient := &ClusterClient{client: client}
	if err := clusterClient.SetRestorePhase(context.Background(), ordersRestore, model.PhaseDeleted); err != nil {
		t.Fatalf("SetRestorePhase returned an error: %v", err)
	}
	updated, _ := client.Resource(postgresqlRestoreGVR).Namespace("platform").Get(context.Background(), "orders-restore", metav1.GetOptions{})
	if got := updated.GetAnnotations()[model.PhaseAnnotation]; got != string(model.PhaseDeleted) {
		t.Fatalf("expected phase %q, got %q", model.PhaseDeleted, got)
	}
	if got := updated.GetAnnotations()[model.PlanAnnotation]; got != `{"kind":"Cluster"}` {
		t.Fatalf("plan annotation must be preserved, got %q", got)
	}
}

func TestPrepareAndDeleteStoresPlanAndDeletesCluster(t *testing.T) {
	database, cluster := prepareFixtures()
	unstructured.SetNestedField(cluster.Object, map[string]interface{}{"initdb": map[string]interface{}{"database": "orders"}}, "spec", "bootstrap")
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), database, cluster, newRestore("orders-restore", nil))
	clusterClient := &ClusterClient{client: client, interval: time.Millisecond, timeout: time.Second}
	if err := clusterClient.PrepareAndDelete(context.Background(), ordersRestore, "platform", "orders-database", "platform", "orders-primary", `{"kind":"Cluster"}`); err != nil {
		t.Fatalf("PrepareAndDelete returned an error: %v", err)
	}
	restore, _ := client.Resource(postgresqlRestoreGVR).Namespace("platform").Get(context.Background(), "orders-restore", metav1.GetOptions{})
	if restore.GetAnnotations()[model.PlanAnnotation] == "" {
		t.Fatal("expected the plan to be stored before deletion")
	}
	if _, err := client.Resource(clusterGVR).Namespace("platform").Get(context.Background(), "orders-primary", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected Cluster to be deleted, got error: %v", err)
	}
}
