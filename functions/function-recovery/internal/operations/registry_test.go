package operations

import (
	"testing"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

func TestLookupReturnsVisitorElementForEachSupportedMode(t *testing.T) {
	modes := []model.Operation{
		model.OperationPrepare,
		model.OperationRestore,
		model.OperationDelete,
		model.OperationCleanup,
		model.OperationPrepareDelete,
	}
	for _, mode := range modes {
		if _, ok := Lookup(mode); !ok {
			t.Errorf("Lookup(%q) did not return an operation", mode)
		}
	}
	if _, ok := Lookup(model.Operation("unknown")); ok {
		t.Fatal("Lookup accepted an unknown operation")
	}
}
