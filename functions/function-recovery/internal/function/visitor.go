package function

import (
	"context"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/operations"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type operationVisitor struct {
	ctx        context.Context
	handler    *Handler
	req        *fnv1.RunFunctionRequest
	rsp        *fnv1.RunFunctionResponse
	input      *model.Input
	postgresql *unstructured.Unstructured
}

func (v operationVisitor) VisitPrepare(operations.Prepare) error {
	cluster, err := v.handler.clusterClient.GetCluster(v.ctx, v.input.Spec.Target.Namespace, v.input.Spec.Target.Name)
	if err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot get recovery Cluster")
		return nil
	}
	if err := operations.PersistRecoveryPlan(v.ctx, v.handler.clusterClient, v.input, v.postgresql, cluster); err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot persist recovery plan")
		return nil
	}
	SucceedOperation(v.rsp, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitRestore(op operations.Restore) error {
	plan, resolved, err := ReadRequiredResource(v.req, op.RequiredResourceName())
	if err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot get recovery plan")
		return nil
	}
	if !resolved {
		InvalidOperation(v.rsp, model.Operation(v.input.Spec.Mode), errUnresolvedResource)
		return nil
	}
	_, cluster, err := op.Run(v.input, plan)
	if err != nil {
		InvalidOperation(v.rsp, model.Operation(v.input.Spec.Mode), err)
		return nil
	}
	if err := v.handler.clusterClient.CreateRestoredCluster(v.ctx, cluster); err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot restore CNPG Cluster")
		return nil
	}
	if v.handler.log != nil {
		v.handler.log.Info("Restored CNPG Cluster", "namespace", cluster.GetNamespace(), "name", cluster.GetName())
	}
	SucceedOperation(v.rsp, model.Operation(v.input.Spec.Mode))
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
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot delete CNPG Cluster")
		return nil
	}
	if v.handler.log != nil {
		v.handler.log.Info("Deleted CNPG Cluster", "namespace", v.input.Spec.Target.Namespace, "name", v.input.Spec.Target.Name)
	}
	SucceedOperation(v.rsp, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitCleanup(operations.Cleanup) error {
	if err := operations.CleanupRecovery(v.ctx, v.handler.clusterClient, v.input); err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot remove CNPG recovery bootstrap")
		return nil
	}
	SucceedOperation(v.rsp, model.Operation(v.input.Spec.Mode))
	return nil
}

func (v operationVisitor) VisitPrepareDelete(operations.PrepareDelete) error {
	cluster, err := v.handler.clusterClient.GetCluster(v.ctx, v.input.Spec.Target.Namespace, v.input.Spec.Target.Name)
	if err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot get recovery Cluster")
		return nil
	}
	if err := operations.PrepareAndDelete(v.ctx, v.handler.clusterClient, v.input, v.postgresql, cluster); err != nil {
		FatalOperation(v.rsp, model.Operation(v.input.Spec.Mode), err, "cannot prepare and delete CNPG Cluster")
		return nil
	}
	SucceedOperation(v.rsp, model.Operation(v.input.Spec.Mode))
	return nil
}

var _ operations.Visitor = operationVisitor{}
