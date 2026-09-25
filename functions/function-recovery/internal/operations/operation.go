package operations

import (
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Operation interface {
	RequiredResourceName() string
	Run(*model.Input, *unstructured.Unstructured) (resource.Name, *unstructured.Unstructured, error)
}

type Element interface {
	Accept(Visitor) error
}

type Visitor interface {
	VisitPrepare(Prepare) error
	VisitRestore(Restore) error
	VisitDelete(Delete) error
	VisitCleanup(Cleanup) error
	VisitPrepareDelete(PrepareDelete) error
}

func (Prepare) Accept(visitor Visitor) error { return visitor.VisitPrepare(Prepare{}) }
func (Restore) Accept(visitor Visitor) error { return visitor.VisitRestore(Restore{}) }
func (Delete) Accept(visitor Visitor) error  { return visitor.VisitDelete(Delete{}) }
func (Cleanup) Accept(visitor Visitor) error { return visitor.VisitCleanup(Cleanup{}) }
func (PrepareDelete) Accept(visitor Visitor) error {
	return visitor.VisitPrepareDelete(PrepareDelete{})
}

type Delete struct{}
type Cleanup struct{}
type PrepareDelete struct{}
