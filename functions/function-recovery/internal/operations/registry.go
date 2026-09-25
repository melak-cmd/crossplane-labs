package operations

import "github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"

var registry = map[model.Operation]Element{
	model.OperationPrepare:       Prepare{},
	model.OperationRestore:       Restore{},
	model.OperationDelete:        Delete{},
	model.OperationCleanup:       Cleanup{},
	model.OperationPrepareDelete: PrepareDelete{},
}

func Lookup(operation model.Operation) (Element, bool) {
	value, ok := registry[operation]
	return value, ok
}
