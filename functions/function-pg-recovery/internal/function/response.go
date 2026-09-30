package function

import (
	"fmt"

	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/model"
)

func Invalid(rsp *fnv1.RunFunctionResponse, err error) {
	response.ConditionFalse(rsp, model.ConditionSuccess, model.ReasonInvalid).WithMessage(err.Error())
}

func Fatal(rsp *fnv1.RunFunctionResponse, err error, message string) {
	response.Fatal(rsp, errors.Wrap(err, message))
}

func Succeed(rsp *fnv1.RunFunctionResponse) {
	response.ConditionTrue(rsp, model.ConditionSuccess, model.ReasonSuccess)
}

func SucceedOperation(
	rsp *fnv1.RunFunctionResponse,
	statusSubject *OperationStatusSubject,
	operation model.Operation,
) {
	Succeed(rsp)
	statusSubject.Notify(OperationStatusEvent{
		Operation: operation,
		Status:    model.OperationStatusSucceeded,
		Message:   fmt.Sprintf(model.MessageOperationSucceeded, operation),
	})
}

func InvalidOperation(
	rsp *fnv1.RunFunctionResponse,
	statusSubject *OperationStatusSubject,
	operation model.Operation,
	err error,
) {
	Invalid(rsp, err)
	statusSubject.Notify(OperationStatusEvent{
		Operation: operation,
		Status:    model.OperationStatusFailed,
		Message:   fmt.Sprintf(model.MessageOperationFailed, operation, err),
	})
}

func FatalOperation(
	rsp *fnv1.RunFunctionResponse,
	statusSubject *OperationStatusSubject,
	operation model.Operation,
	err error,
	message string,
) {
	Fatal(rsp, err, message)
	statusSubject.Notify(OperationStatusEvent{
		Operation: operation,
		Status:    model.OperationStatusFailed,
		Message:   fmt.Sprintf(model.MessageOperationFailed, operation, fmt.Sprintf("%s: %v", message, err)),
	})
}

// SkipOperation reports success for a step that was invoked in a recovery phase
// it must not run in, for example by an additional Operation that was started
// when the PostgreSQLRestore was updated.
func SkipOperation(
	rsp *fnv1.RunFunctionResponse,
	statusSubject *OperationStatusSubject,
	operation model.Operation,
	phase model.Phase,
) {
	Succeed(rsp)
	statusSubject.Notify(OperationStatusEvent{
		Operation: operation,
		Status:    model.OperationStatusSucceeded,
		Message:   fmt.Sprintf(model.MessageOperationSkipped, operation, phase),
	})
}

// Failed reports whether the response already carries a failure.
func Failed(rsp *fnv1.RunFunctionResponse) bool {
	for _, result := range rsp.GetResults() {
		if result.GetSeverity() == fnv1.Severity_SEVERITY_FATAL {
			return true
		}
	}
	for _, condition := range rsp.GetConditions() {
		if condition.GetStatus() == fnv1.Status_STATUS_CONDITION_FALSE {
			return true
		}
	}
	return false
}
