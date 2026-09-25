package function

import (
	"errors"
	"reflect"
	"testing"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

func TestSucceedOperationSetsOperationOutput(t *testing.T) {
	rsp := &fnv1.RunFunctionResponse{}

	SucceedOperation(rsp, model.OperationPrepare)

	if got, want := rsp.GetOutput().AsMap(), map[string]interface{}{
		"operation": "prepare",
		"status":    model.OperationStatusSucceeded,
		"message":   "prepare operation completed successfully",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected operation output: got %#v, want %#v", got, want)
	}
}

func TestInvalidOperationSetsFailedOutput(t *testing.T) {
	rsp := &fnv1.RunFunctionResponse{}

	InvalidOperation(rsp, model.OperationRestore, errors.New("invalid recovery plan"))

	if got, want := rsp.GetOutput().AsMap(), map[string]interface{}{
		"operation": "restore",
		"status":    model.OperationStatusFailed,
		"message":   "restore operation failed: invalid recovery plan",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected operation output: got %#v, want %#v", got, want)
	}
}
