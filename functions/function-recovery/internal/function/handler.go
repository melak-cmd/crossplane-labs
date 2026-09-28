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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Handler struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log           logging.Logger
	clusterClient interface {
		operations.ClusterReader
		operations.RecoveryPlanReader
		operations.PostgreSQLReader
		operations.PostgreSQLResumer
		operations.RestoreRequestAcknowledger
		operations.ClusterRestorer
		operations.ClusterDeleter
		operations.RecoveryCleaner
		operations.PrepareDeleter
		operations.RecoveryPreparer
	}
}

func New(log logging.Logger, clusterClient interface {
	operations.ClusterReader
	operations.RecoveryPlanReader
	operations.PostgreSQLReader
	operations.PostgreSQLResumer
	operations.RestoreRequestAcknowledger
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
	statusSubject := newOperationStatusSubject(rsp)
	in, err := ReadInput(req)
	if err != nil {
		FatalOperation(rsp, statusSubject, model.OperationUnknown, err, "cannot get recovery input")
		return rsp, nil
	}
	database, resolved, err := h.databaseForRequest(ctx, req, in)
	if err != nil {
		FatalOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), err, "cannot resolve restore request")
		return rsp, nil
	}
	if !resolved {
		if in.Spec.WatchedRequest {
			SucceedOperation(rsp, statusSubject, model.Operation(in.Spec.Mode))
			return rsp, nil
		}
		InvalidOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), fmt.Errorf("required PostgreSQL XR is unresolved"))
		return rsp, nil
	}
	clusterName, err := ReadClusterName(database)
	if err != nil {
		InvalidOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), err)
		return rsp, nil
	}
	in.Spec.Target.Name = clusterName
	if err := validation.Input(in); err != nil {
		InvalidOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), err)
		return rsp, nil
	}
	op, ok := operations.Lookup(model.Operation(in.Spec.Mode))
	if !ok {
		InvalidOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), errUnknownOperation)
		return rsp, nil
	}
	if h.log != nil {
		h.log.Info("Running recovery operation", "mode", in.Spec.Mode, "postgresql", database.GetName(), "cluster", clusterName)
	}
	visitor := operationVisitor{
		ctx: ctx, handler: h, rsp: rsp, input: in,
		postgresql: database, statusSubject: statusSubject,
	}
	if err := op.Accept(visitor); err != nil {
		FatalOperation(rsp, statusSubject, model.Operation(in.Spec.Mode), err, "cannot execute recovery operation")
		return rsp, nil
	}
	return rsp, nil
}

func (h *Handler) databaseForRequest(ctx context.Context, req *fnv1.RunFunctionRequest, in *model.Input) (*unstructured.Unstructured, bool, error) {
	request, resolved, err := ReadRequiredResource(req, "ops.crossplane.io/watched-resource")
	if err != nil {
		return nil, false, err
	}
	if resolved {
		if request.GetAPIVersion() != "database.kaonix.inc.fr/v1alpha1" || request.GetKind() != "DatabaseRestore" {
			return nil, true, fmt.Errorf("watched resource is not a DatabaseRestore request")
		}
		if in.Spec.Mode != "restore" {
			if in.Spec.Mode != "prepare" && in.Spec.Mode != "delete" && in.Spec.Mode != "cleanup" && in.Spec.Mode != "resume" {
				return nil, true, fmt.Errorf("watched DatabaseRestore has unsupported mode %q", in.Spec.Mode)
			}
		}
		name := request.GetName()
		backupName, found, err := unstructured.NestedString(request.Object, "spec", "backupName")
		if err != nil || !found || backupName == "" {
			return nil, true, fmt.Errorf("watched DatabaseRestore has no spec.backupName")
		}
		targetNamespace, found, err := unstructured.NestedString(request.Object, "spec", "target", "namespace")
		if err != nil || !found || targetNamespace == "" {
			return nil, true, fmt.Errorf("watched DatabaseRestore has no spec.target.namespace")
		}
		if err := h.clusterClient.AcknowledgeRestoreRequest(ctx, name); err != nil {
			return nil, true, fmt.Errorf("cannot acknowledge DatabaseRestore %q: %w", name, err)
		}
		in.Spec.Target.Namespace = targetNamespace
		in.Spec.PlanName = name + "-recovery-plan"
		if in.Spec.Mode == "restore" {
			in.Spec.Backup = &model.BackupReference{Name: backupName, Namespace: targetNamespace}
		}
		database, err := h.clusterClient.GetPostgreSQL(ctx, targetNamespace, name)
		return database, true, err
	}

	database, resolved, err := ReadRequiredResource(req, "postgresql")
	if err != nil || !resolved {
		return nil, resolved, err
	}
	return database, true, nil
}
