package operations

import (
	"context"
	"fmt"

	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/cnpg"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ClusterDeleter interface {
	DeleteAndWait(context.Context, string, string) error
}

type ClusterReader interface {
	GetCluster(context.Context, string, string) (*unstructured.Unstructured, error)
}

type RestoreReader interface {
	GetPostgreSQLRestore(context.Context, string, string) (*unstructured.Unstructured, error)
}

type RestorePhaseSetter interface {
	SetRestorePhase(context.Context, model.RestoreRef, model.Phase) error
}

type PostgreSQLReader interface {
	GetPostgreSQL(context.Context, string, string) (*unstructured.Unstructured, error)
}

type PostgreSQLResumer interface {
	ResumePostgreSQL(context.Context, string, string) error
}

type ClusterRestorer interface {
	CreateRestoredCluster(context.Context, *unstructured.Unstructured) error
}

type RecoveryCleaner interface {
	RemoveRecovery(context.Context, string, string) error
}

type PrepareDeleter interface {
	PrepareAndDelete(context.Context, model.RestoreRef, string, string, string, string, string) error
}

type RecoveryPreparer interface {
	PrepareRecovery(context.Context, model.RestoreRef, string, string, string) error
}

func DeleteCluster(ctx context.Context, deleter ClusterDeleter, in *model.Input) error {
	if deleter == nil {
		return fmt.Errorf("CNPG Cluster deletion client is not configured")
	}
	return deleter.DeleteAndWait(ctx, in.Spec.Target.Namespace, in.Spec.Target.Name)
}

func CleanupRecovery(ctx context.Context, cleaner RecoveryCleaner, in *model.Input) error {
	if cleaner == nil {
		return fmt.Errorf("CNPG recovery cleanup client is not configured")
	}
	return cleaner.RemoveRecovery(ctx, in.Spec.Target.Namespace, in.Spec.Target.Name)
}

func PrepareAndDelete(ctx context.Context, client PrepareDeleter, in *model.Input, restore model.RestoreRef, postgresql, cluster *unstructured.Unstructured) error {
	if client == nil {
		return fmt.Errorf("CNPG recovery client is not configured")
	}
	plan, err := cnpg.RecoveryPlan(in, cluster)
	if err != nil {
		return err
	}
	return client.PrepareAndDelete(ctx, restore, postgresql.GetNamespace(), postgresql.GetName(), in.Spec.Target.Namespace, in.Spec.Target.Name, plan)
}

func PersistRecoveryPlan(ctx context.Context, client RecoveryPreparer, in *model.Input, restore model.RestoreRef, postgresql, cluster *unstructured.Unstructured) error {
	if client == nil {
		return fmt.Errorf("CNPG recovery client is not configured")
	}
	plan, err := cnpg.RecoveryPlan(in, cluster)
	if err != nil {
		return err
	}
	return client.PrepareRecovery(ctx, restore, postgresql.GetNamespace(), postgresql.GetName(), plan)
}
