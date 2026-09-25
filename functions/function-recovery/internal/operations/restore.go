package operations

import (
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/cnpg"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Restore struct{}

func (Restore) RequiredResourceName() string { return "recovery-plan" }

func (Restore) Run(in *model.Input, observed *unstructured.Unstructured) (resource.Name, *unstructured.Unstructured, error) {
	cluster, err := cnpg.RestoreCluster(in, observed)
	return resource.Name(in.Spec.Target.Name), cluster, err
}
