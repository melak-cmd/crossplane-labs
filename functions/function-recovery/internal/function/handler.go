package function

import (
	"context"
	"fmt"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/operations"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/validation"
)

type Handler struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log           logging.Logger
	clusterClient interface {
		operations.ClusterReader
		operations.ClusterRestorer
		operations.ClusterDeleter
		operations.RecoveryCleaner
		operations.PrepareDeleter
		operations.RecoveryPreparer
	}
}

func New(log logging.Logger, clusterClient interface {
	operations.ClusterReader
	operations.ClusterRestorer
	operations.ClusterDeleter
	operations.RecoveryCleaner
	operations.PrepareDeleter
	operations.RecoveryPreparer
}) *Handler {
	return &Handler{log: log, clusterClient: clusterClient}
}

func (h *Handler) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	rsp := response.To(req, response.DefaultTTL)
	in, err := ReadInput(req)
	if err != nil {
		Fatal(rsp, err, "cannot get recovery input")
		return rsp, nil
	}
	database, resolved, err := ReadRequiredResource(req, "postgresql")
	if err != nil {
		Fatal(rsp, err, "cannot get PostgreSQL XR")
		return rsp, nil
	}
	if !resolved {
		Invalid(rsp, fmt.Errorf("required PostgreSQL XR is unresolved"))
		return rsp, nil
	}
	clusterName, err := ReadClusterName(database)
	if err != nil {
		Invalid(rsp, err)
		return rsp, nil
	}
	in.Spec.Target.Name = clusterName
	if err := validation.Input(in); err != nil {
		Invalid(rsp, err)
		return rsp, nil
	}
	op, ok := operations.Lookup(model.Operation(in.Spec.Mode))
	if !ok {
		Invalid(rsp, errUnknownOperation)
		return rsp, nil
	}
	if h.log != nil {
		h.log.Info("Running recovery operation", "mode", in.Spec.Mode, "postgresql", database.GetName(), "cluster", clusterName)
	}
	visitor := operationVisitor{ctx: ctx, handler: h, req: req, rsp: rsp, input: in, postgresql: database}
	if err := op.Accept(visitor); err != nil {
		Fatal(rsp, err, "cannot execute recovery operation")
		return rsp, nil
	}
	return rsp, nil
}
