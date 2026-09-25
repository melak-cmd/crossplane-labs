package kubernetes

import (
	"context"
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
)

var clusterGVR = schema.GroupVersionResource{
	Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
}

var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
var postgresqlGVR = schema.GroupVersionResource{Group: "database.kaonix.inc.fr", Version: "v1alpha1", Resource: "postgresqls"}

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

func (c *ClusterClient) CreateRestoredCluster(ctx context.Context, cluster *unstructured.Unstructured) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	clusters := c.client.Resource(clusterGVR).Namespace(cluster.GetNamespace())
	if _, err := clusters.Create(ctx, cluster, metav1.CreateOptions{}); err == nil {
		return nil
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
	return nil
}

func (c *ClusterClient) PrepareAndDelete(ctx context.Context, postgresqlNamespace, postgresqlName, clusterNamespace, clusterName string, plan *unstructured.Unstructured) error {
	if err := c.PrepareRecovery(ctx, postgresqlNamespace, postgresqlName, plan); err != nil {
		return err
	}
	return c.DeleteAndWait(ctx, clusterNamespace, clusterName)
}

func (c *ClusterClient) PrepareRecovery(ctx context.Context, namespace, name string, plan *unstructured.Unstructured) error {
	c.initialize()
	if c.initErr != nil {
		return c.initErr
	}
	postgresqls := c.client.Resource(postgresqlGVR).Namespace(namespace)
	if _, err := postgresqls.Get(ctx, name, metav1.GetOptions{}); err != nil {
		return err
	}
	pausePatch := []byte(`{"metadata":{"annotations":{"crossplane.io/paused":"true"}}}`)
	if _, err := postgresqls.Patch(ctx, name, types.MergePatchType, pausePatch, metav1.PatchOptions{}); err != nil {
		return err
	}
	configMaps := c.client.Resource(configMapGVR).Namespace(namespace)
	plan = plan.DeepCopy()
	plan.SetNamespace(namespace)
	current, err := configMaps.Get(ctx, plan.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := configMaps.Create(ctx, plan, metav1.CreateOptions{}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		plan.SetResourceVersion(current.GetResourceVersion())
		if _, err := configMaps.Update(ctx, plan, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
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
