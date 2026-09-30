package function

import (
	"context"
	"fmt"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/operations"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type operationVisitor struct {
	ctx           context.Context
	handler       *Handler
	rsp           *fnv1.RunFunctionResponse
	input         *model.Input
	restore       model.RestoreRef
	postgresql    *unstructured.Unstructured
	statusSubject *OperationStatusSubject
}

func (v operationVisitor) VisitPrepare(operations.Prepare) error {
	cluster, err := v.handler.clusterClient.GetCluster(v.ctx, v.input.Spec.Target.Namespace, v.input.Spec.Target.Name)
	if err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot get recovery Cluster")
		return nil
	}
	if err := operations.PersistRecoveryPlan(v.ctx, v.handler.clusterClient, v.input, v.restore, v.postgresql, cluster); err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot persist recovery plan")
		return nil
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitRestore(op operations.Restore) error {
	restore, err := v.handler.clusterClient.GetPostgreSQLRestore(v.ctx, v.restore.Namespace, v.restore.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			InvalidOperation(
				v.rsp,
				v.statusSubject,
				model.Operation(v.input.Spec.Mode),
				fmt.Errorf("PostgreSQLRestore %s/%s was not found", v.restore.Namespace, v.restore.Name),
			)
			return nil
		}
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot get PostgreSQLRestore")
		return nil
	}
	_, cluster, err := op.Run(v.input, restore)
	if err != nil {
		InvalidOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err)
		return nil
	}
	if err := v.handler.clusterClient.CreateRestoredCluster(v.ctx, cluster); err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot restore CNPG Cluster")
		return nil
	}
	if v.handler.log != nil {
		v.handler.log.Info("Restored CNPG Cluster", "namespace", cluster.GetNamespace(), "name", cluster.GetName())
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitDelete(operations.Delete) error {
	if v.handler.log != nil {
		v.handler.log.Info("Deleting CNPG Cluster", "namespace", v.input.Spec.Target.Namespace, "name", v.input.Spec.Target.Name)
	}
	if err := operations.DeleteCluster(v.ctx, v.handler.clusterClient, v.input); err != nil {
		if v.handler.log != nil {
			v.handler.log.Info("Failed to delete CNPG Cluster", "namespace", v.input.Spec.Target.Namespace, "name", v.input.Spec.Target.Name, "error", err)
		}
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot delete CNPG Cluster")
		return nil
	}
	if v.handler.log != nil {
		v.handler.log.Info("Deleted CNPG Cluster", "namespace", v.input.Spec.Target.Namespace, "name", v.input.Spec.Target.Name)
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitCleanup(operations.Cleanup) error {
	if err := operations.CleanupRecovery(v.ctx, v.handler.clusterClient, v.input); err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot remove CNPG recovery bootstrap")
		return nil
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitPrepareDelete(operations.PrepareDelete) error {
	cluster, err := v.handler.clusterClient.GetCluster(v.ctx, v.input.Spec.Target.Namespace, v.input.Spec.Target.Name)
	if err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot get recovery Cluster")
		return nil
	}
	if err := operations.PrepareAndDelete(v.ctx, v.handler.clusterClient, v.input, v.restore, v.postgresql, cluster); err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot prepare and delete CNPG Cluster")
		return nil
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitResume(operations.Resume) error {
	if err := v.handler.clusterClient.ResumePostgreSQL(
		v.ctx, v.input.Spec.Target.Namespace, v.postgresql.GetName(),
	); err != nil {
		FatalOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode), err, "cannot resume PostgreSQL XR")
		return nil
	}
	SucceedOperation(v.rsp, v.statusSubject, model.Operation(v.input.Spec.Mode))
	return nil
}

var _ operations.Visitor = operationVisitor{}
