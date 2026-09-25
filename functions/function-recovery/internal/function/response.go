package function

import (
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
