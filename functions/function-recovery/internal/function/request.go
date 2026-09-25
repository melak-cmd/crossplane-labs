package function

import (
	"fmt"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ReadInput(req *fnv1.RunFunctionRequest) (*model.Input, error) {
	in := &model.Input{}
	if err := request.GetInput(req, in); err != nil {
		return nil, err
	}
	return in, nil
}

func ReadRequiredResource(req *fnv1.RunFunctionRequest, name string) (*unstructured.Unstructured, bool, error) {
	resources, resolved, err := request.GetRequiredResource(req, name)
	if err != nil || !resolved || len(resources) != 1 {
		return nil, resolved && len(resources) == 1, err
	}
	return resources[0].Resource, true, nil
}

func ReadClusterName(database *unstructured.Unstructured) (string, error) {
	if database.GetAPIVersion() != "database.kaonix.inc.fr/v1alpha1" || database.GetKind() != "PostgreSQL" {
		return "", fmt.Errorf("required resource is not a PostgreSQL XR")
	}
	refs, found, err := unstructured.NestedSlice(database.Object, "spec", "crossplane", "resourceRefs")
	if err != nil {
		return "", fmt.Errorf("cannot read PostgreSQL XR resourceRefs: %w", err)
	}
	if !found {
		return "", fmt.Errorf("PostgreSQL XR has no resourceRefs")
	}

	var clusterName string
	for _, ref := range refs {
		resourceRef, ok := ref.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("PostgreSQL XR contains an invalid resourceRef")
		}
		if resourceRef["apiVersion"] != "postgresql.cnpg.io/v1" || resourceRef["kind"] != "Cluster" {
			continue
		}
		name, ok := resourceRef["name"].(string)
		if !ok || name == "" {
			return "", fmt.Errorf("PostgreSQL XR CNPG Cluster resourceRef has no name")
		}
		if clusterName != "" {
			return "", fmt.Errorf("PostgreSQL XR has multiple CNPG Cluster resourceRefs")
		}
		clusterName = name
	}
	if clusterName == "" {
		return "", fmt.Errorf("PostgreSQL XR has no CNPG Cluster resourceRef")
	}
	return clusterName, nil
}
