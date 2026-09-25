package function

import (
	"fmt"

	"github.com/crossplane/function-sdk-go/errors"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
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

func SucceedOperation(rsp *fnv1.RunFunctionResponse, operation model.Operation) {
	Succeed(rsp)
	setOperationOutput(
		rsp,
		operation,
		model.OperationStatusSucceeded,
		fmt.Sprintf(model.MessageOperationSucceeded, operation),
	)
}

func InvalidOperation(rsp *fnv1.RunFunctionResponse, operation model.Operation, err error) {
	Invalid(rsp, err)
	setOperationOutput(
		rsp,
		operation,
		model.OperationStatusFailed,
		fmt.Sprintf(model.MessageOperationFailed, operation, err),
	)
}

func FatalOperation(
	rsp *fnv1.RunFunctionResponse,
	operation model.Operation,
	err error,
	message string,
) {
	Fatal(rsp, err, message)
	setOperationOutput(
		rsp,
		operation,
		model.OperationStatusFailed,
		fmt.Sprintf(model.MessageOperationFailed, operation, fmt.Sprintf("%s: %v", message, err)),
	)
}

func setOperationOutput(
	rsp *fnv1.RunFunctionResponse,
	operation model.Operation,
	status string,
	message string,
) {
	if err := response.SetOutput(rsp, map[string]string{
		"operation": string(operation),
		"status":    status,
		"message":   message,
	}); err != nil {
		Fatal(rsp, err, "cannot set operation status output")
	}
}
