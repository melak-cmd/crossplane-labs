package validation

import (
	"testing"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

func TestInputAcceptsPrepareAndRestore(t *testing.T) {
	inputs := []*model.Input{
		{Spec: model.InputSpec{Mode: string(model.OperationPrepare), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationPrepareDelete), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationRestore), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}, Backup: &model.BackupReference{Name: "backup", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationDelete), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationCleanup), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationResume), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
	}
	for _, in := range inputs {
		if err := Input(in); err != nil {
			t.Fatalf("valid input rejected: %v", err)
		}
	}
}

func TestInputRejectsInvalidRecoverySettings(t *testing.T) {
	inputs := []*model.Input{
		{Spec: model.InputSpec{Mode: "unknown", Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationPrepare), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}, Backup: &model.BackupReference{Name: "backup", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationDelete), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}, Backup: &model.BackupReference{Name: "backup", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationRestore), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationRestore), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}, Backup: &model.BackupReference{Name: "backup"}}},
		{Spec: model.InputSpec{Mode: string(model.OperationRestore), Target: model.ClusterReference{Name: "orders", Namespace: "platform"}, Backup: &model.BackupReference{Name: "backup", Namespace: "other"}}},
	}
	for _, in := range inputs {
		if err := Input(in); err == nil {
			t.Fatal("expected invalid input to be rejected")
		}
	}
}
