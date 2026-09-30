package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

var clusterGVR = schema.GroupVersionResource{
	Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
}

var postgresqlGVR = schema.GroupVersionResource{Group: "database.nuagik.sncf.fr", Version: "v1alpha1", Resource: "postgresqls"}
var postgresqlRestoreGVR = schema.GroupVersionResource{Group: "database.nuagik.sncf.fr", Version: "v1alpha1", Resource: "postgresqlrestores"}

// maxAnnotationBytes is the Kubernetes limit on the total size of all
// annotations of an object (keys plus values).
const maxAnnotationBytes = 256 * (1 << 10)

type ClusterClient struct {
	client   dynamic.Interface
	interval time.Duration
	timeout  time.Duration
	once     sync.Once
	initErr  error
}

func New() *ClusterClient {
	return &ClusterClient{interval: time.Second, timeout: 5 * time.Minute}
}

func (c *ClusterClient) initialize() {
	c.once.Do(func() {
		if c.client != nil {
			return
		}
		config, err := rest.InClusterConfig()
		if err != nil {
			c.initErr = err
			return
		}
		c.client, c.initErr = dynamic.NewForConfig(config)
	})
}

func (c *ClusterClient) DeleteAndWait(ctx context.Context, namespace, name string) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	clusters := c.client.Resource(clusterGVR).Namespace(namespace)
	policy := metav1.DeletePropagationBackground
	if err := clusters.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, c.interval, c.timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clusters.Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return apierrors.IsNotFound(err), nil
	})
}

func (c *ClusterClient) GetCluster(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	c.initialize()
	if c.initErr != nil {
		return nil, c.initErr
	}
	return c.client.Resource(clusterGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetPostgreSQLRestore reads the live PostgreSQLRestore that carries the
// recovery plan and phase.
func (c *ClusterClient) GetPostgreSQLRestore(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	c.initialize()
	if c.initErr != nil {
		return nil, c.initErr
	}
	return c.client.Resource(postgresqlRestoreGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *ClusterClient) GetPostgreSQL(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	c.initialize()
	if c.initErr != nil {
		return nil, c.initErr
	}
	return c.client.Resource(postgresqlGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *ClusterClient) CreateRestoredCluster(ctx context.Context, cluster *unstructured.Unstructured) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	clusters := c.client.Resource(clusterGVR).Namespace(cluster.GetNamespace())
	if _, err := clusters.Create(ctx, cluster, metav1.CreateOptions{}); err == nil {
		return c.waitForReady(ctx, clusters, cluster.GetName())
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	current, err := clusters.Get(ctx, cluster.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	currentRecovery, currentFound, err := unstructured.NestedFieldNoCopy(current.Object, "spec", "bootstrap", "recovery")
	if err != nil {
		return err
	}
	desiredRecovery, desiredFound, err := unstructured.NestedFieldNoCopy(cluster.Object, "spec", "bootstrap", "recovery")
	if err != nil {
		return err
	}
	if !currentFound || !desiredFound || !apiequality.Semantic.DeepEqual(currentRecovery, desiredRecovery) {
		return fmt.Errorf("CNPG Cluster %s/%s already exists without the requested recovery bootstrap", cluster.GetNamespace(), cluster.GetName())
	}
	return c.waitForReady(ctx, clusters, cluster.GetName())
}

func (c *ClusterClient) waitForReady(ctx context.Context, clusters dynamic.ResourceInterface, name string) error {
	return wait.PollUntilContextTimeout(ctx, c.interval, c.timeout, true, func(ctx context.Context) (bool, error) {
		cluster, err := clusters.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		conditions, found, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
		if err != nil || !found {
			return false, err
		}
		for _, value := range conditions {
			condition, ok := value.(map[string]interface{})
			if ok && condition["type"] == "Ready" && condition["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

func (c *ClusterClient) PrepareAndDelete(ctx context.Context, restore model.RestoreRef, postgresqlNamespace, postgresqlName, clusterNamespace, clusterName, plan string) error {
	if err := c.PrepareRecovery(ctx, restore, postgresqlNamespace, postgresqlName, plan); err != nil {
		return err
	}
	return c.DeleteAndWait(ctx, clusterNamespace, clusterName)
}

// PrepareRecovery pauses the PostgreSQL XR and records the plan and the
// prepared phase on the PostgreSQLRestore in one patch. It fails before
// changing anything when the plan does not fit in the annotation budget.
func (c *ClusterClient) PrepareRecovery(ctx context.Context, restore model.RestoreRef, postgresqlNamespace, postgresqlName, plan string) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	restores := c.client.Resource(postgresqlRestoreGVR).Namespace(restore.Namespace)
	current, err := restores.Get(ctx, restore.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	annotations := map[string]string{
		model.PlanAnnotation:  plan,
		model.PhaseAnnotation: string(model.PhasePrepared),
	}
	if err := checkAnnotationBudget(current.GetAnnotations(), annotations); err != nil {
		return err
	}
	postgresqls := c.client.Resource(postgresqlGVR).Namespace(postgresqlNamespace)
	if _, err := postgresqls.Get(ctx, postgresqlName, metav1.GetOptions{}); err != nil {
		return err
	}
	pausePatch := []byte(`{"metadata":{"annotations":{"crossplane.io/paused":"true"}}}`)
	if _, err := postgresqls.Patch(ctx, postgresqlName, types.MergePatchType, pausePatch, metav1.PatchOptions{}); err != nil {
		return err
	}
	return c.patchRestoreAnnotations(ctx, restore, annotations)
}

// SetRestorePhase records the recovery phase on the PostgreSQLRestore.
func (c *ClusterClient) SetRestorePhase(ctx context.Context, restore model.RestoreRef, phase model.Phase) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	return c.patchRestoreAnnotations(ctx, restore, map[string]string{model.PhaseAnnotation: string(phase)})
}

func (c *ClusterClient) patchRestoreAnnotations(ctx context.Context, restore model.RestoreRef, annotations map[string]string) error {
	patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"annotations": annotations}})
	if err != nil {
		return fmt.Errorf("cannot encode PostgreSQLRestore annotation patch: %w", err)
	}
	_, err = c.client.Resource(postgresqlRestoreGVR).Namespace(restore.Namespace).Patch(ctx, restore.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// checkAnnotationBudget verifies that adding the annotations keeps the total
// annotation size within the Kubernetes limit.
func checkAnnotationBudget(existing, add map[string]string) error {
	merged := map[string]string{}
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range add {
		merged[key] = value
	}
	total := 0
	for key, value := range merged {
		total += len(key) + len(value)
	}
	if total > maxAnnotationBytes {
		return fmt.Errorf("recovery plan is too large: annotations would be %d bytes, the Kubernetes limit is %d bytes", total, maxAnnotationBytes)
	}
	return nil
}

func (c *ClusterClient) ResumePostgreSQL(ctx context.Context, namespace, name string) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	postgresqls := c.client.Resource(postgresqlGVR).Namespace(namespace)
	patch := []byte(`{"metadata":{"annotations":{"crossplane.io/paused":"false"}}}`)
	_, err := postgresqls.Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (c *ClusterClient) RemoveRecovery(ctx context.Context, namespace, name string) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	clusters := c.client.Resource(clusterGVR).Namespace(namespace)
	cluster, err := clusters.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, found, err := unstructured.NestedFieldNoCopy(cluster.Object, "spec", "bootstrap", "recovery")
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	patch := []byte(`[{"op":"remove","path":"/spec/bootstrap/recovery"}]`)
	_, err = clusters.Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
	return err
}
