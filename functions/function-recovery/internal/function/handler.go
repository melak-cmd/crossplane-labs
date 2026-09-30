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

const (
	restoreAPIVersion = "database.nuagik.sncf.fr/v1alpha1"
	restoreKind       = "PostgreSQLRestore"
)

// ClusterClient is everything the handler needs from the Kubernetes API.
type ClusterClient interface {
	operations.ClusterReader
	operations.RestoreReader
	operations.RestorePhaseSetter
	operations.PostgreSQLReader
	operations.PostgreSQLResumer
	operations.ClusterRestorer
	operations.ClusterDeleter
	operations.RecoveryCleaner
	operations.PrepareDeleter
	operations.RecoveryPreparer
}

type Handler struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	log           logging.Logger
	clusterClient ClusterClient
}

func New(log logging.Logger, clusterClient ClusterClient) *Handler {
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
	database, restore, resolved, err := h.databaseForRequest(ctx, req, in)
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
	operation := model.Operation(in.Spec.Mode)
	op, ok := operations.Lookup(operation)
	if !ok {
		InvalidOperation(rsp, statusSubject, operation, errUnknownOperation)
		return rsp, nil
	}
	required, next, _ := model.PhaseGate(operation)
	current, err := h.clusterClient.GetPostgreSQLRestore(ctx, restore.Namespace, restore.Name)
	if err != nil {
		FatalOperation(rsp, statusSubject, operation, err, "cannot get PostgreSQLRestore")
		return rsp, nil
	}
	phase := model.Phase(current.GetAnnotations()[model.PhaseAnnotation])
	if phase != required {
		// Every update of the watched PostgreSQLRestore starts another
		// Operation, and a step cannot tell which Operation it belongs to. The
		// recorded phase is what keeps completed work from being repeated.
		if h.log != nil {
			h.log.Info("Skipping recovery operation in this phase", "mode", in.Spec.Mode, "phase", string(phase), "required", string(required))
		}
		SkipOperation(rsp, statusSubject, operation, phase)
		return rsp, nil
	}
	if h.log != nil {
		h.log.Info("Running recovery operation", "mode", in.Spec.Mode, "postgresql", database.GetName(), "cluster", clusterName)
	}
	visitor := operationVisitor{
		ctx: ctx, handler: h, rsp: rsp, input: in, restore: restore,
		postgresql: database, statusSubject: statusSubject,
	}
	if err := op.Accept(visitor); err != nil {
		FatalOperation(rsp, statusSubject, operation, err, "cannot execute recovery operation")
		return rsp, nil
	}
	if Failed(rsp) {
		return rsp, nil
	}
	// prepare records its phase together with the plan in a single update.
	if operation != model.OperationPrepare {
		if err := h.clusterClient.SetRestorePhase(ctx, restore, next); err != nil {
			FatalOperation(rsp, statusSubject, operation, err, "cannot record recovery phase")
			return rsp, nil
		}
	}
	return rsp, nil
}

func (h *Handler) databaseForRequest(ctx context.Context, req *fnv1.RunFunctionRequest, in *model.Input) (*unstructured.Unstructured, model.RestoreRef, bool, error) {
	request, resolved, err := ReadRequiredResource(req, "ops.crossplane.io/watched-resource")
	if err != nil {
		return nil, model.RestoreRef{}, false, err
	}
	if resolved {
		if request.GetAPIVersion() != restoreAPIVersion || request.GetKind() != restoreKind {
			return nil, model.RestoreRef{}, true, fmt.Errorf("watched resource is not a PostgreSQLRestore request")
		}
		if in.Spec.Mode != "restore" {
			if in.Spec.Mode != "prepare" && in.Spec.Mode != "delete" && in.Spec.Mode != "cleanup" && in.Spec.Mode != "resume" {
				return nil, model.RestoreRef{}, true, fmt.Errorf("watched PostgreSQLRestore has unsupported mode %q", in.Spec.Mode)
			}
		}
		name, found, err := unstructured.NestedString(request.Object, "spec", "name")
		if err != nil || !found || name == "" {
			return nil, model.RestoreRef{}, true, fmt.Errorf("watched PostgreSQLRestore has no spec.name")
		}
		backupName, found, err := unstructured.NestedString(request.Object, "spec", "backupName")
		if err != nil || !found || backupName == "" {
			return nil, model.RestoreRef{}, true, fmt.Errorf("watched PostgreSQLRestore has no spec.backupName")
		}
		targetNamespace := request.GetNamespace()
		if targetNamespace == "" {
			return nil, model.RestoreRef{}, true, fmt.Errorf("watched PostgreSQLRestore has no namespace")
		}
		in.Spec.Target.Namespace = targetNamespace
		if in.Spec.Mode == "restore" {
			in.Spec.Backup = &model.BackupReference{Name: backupName, Namespace: targetNamespace}
		}
		database, err := h.clusterClient.GetPostgreSQL(ctx, targetNamespace, name)
		return database, model.RestoreRef{Namespace: targetNamespace, Name: request.GetName()}, true, err
	}

	database, resolved, err := ReadRequiredResource(req, "postgresql")
	if err != nil || !resolved {
		return nil, model.RestoreRef{}, resolved, err
	}
	restore, restoreResolved, err := ReadRequiredResource(req, "postgresqlrestore")
	if err != nil {
		return nil, model.RestoreRef{}, true, err
	}
	if !restoreResolved {
		return nil, model.RestoreRef{}, true, fmt.Errorf("a PostgreSQLRestore required resource named %q is required", "postgresqlrestore")
	}
	if restore.GetAPIVersion() != restoreAPIVersion || restore.GetKind() != restoreKind {
		return nil, model.RestoreRef{}, true, fmt.Errorf("required resource %q is not a PostgreSQLRestore", "postgresqlrestore")
	}
	return database, model.RestoreRef{Namespace: restore.GetNamespace(), Name: restore.GetName()}, true, nil
}
