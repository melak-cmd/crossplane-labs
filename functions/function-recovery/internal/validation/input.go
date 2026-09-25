package validation

import (
	"fmt"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

func Input(in *model.Input) error {
	if in.Spec.Target.Name == "" || in.Spec.Target.Namespace == "" {
		return fmt.Errorf("target name and namespace are required")
	}
	if in.Spec.Mode != string(model.OperationPrepare) && in.Spec.Mode != string(model.OperationRestore) && in.Spec.Mode != string(model.OperationDelete) && in.Spec.Mode != string(model.OperationCleanup) && in.Spec.Mode != string(model.OperationPrepareDelete) {
		return fmt.Errorf("mode must be prepare, restore, delete, cleanup, or prepare-delete")
	}
	if in.Spec.Mode == string(model.OperationDelete) || in.Spec.Mode == string(model.OperationCleanup) {
		if in.Spec.Backup != nil || in.Spec.VolumeSnapshots != nil {
			return fmt.Errorf("delete mode does not accept recovery sources")
		}
		return nil
	}
	if in.Spec.PlanName == "" {
		return fmt.Errorf("planName is required")
	}
	if in.Spec.Mode == string(model.OperationPrepare) || in.Spec.Mode == string(model.OperationPrepareDelete) {
		if in.Spec.Backup != nil || in.Spec.VolumeSnapshots != nil {
			return fmt.Errorf("prepare modes do not accept recovery sources")
		}
		return nil
	}
	backup := in.Spec.Backup != nil
	snapshots := in.Spec.VolumeSnapshots != nil
	if backup == snapshots {
		return fmt.Errorf("exactly one of backup or volumeSnapshots is required")
	}
	if backup && (in.Spec.Backup.Name == "" || in.Spec.Backup.Namespace == "") {
		return fmt.Errorf("backup name and namespace are required")
	}
	if backup && in.Spec.Backup.Namespace != in.Spec.Target.Namespace {
		return fmt.Errorf("backup and target must use the same namespace")
	}
	if snapshots && (in.Spec.VolumeSnapshots.Data == "" || in.Spec.VolumeSnapshots.Wal == "" || in.Spec.VolumeSnapshots.StorageClass == "") {
		return fmt.Errorf("volumeSnapshots data, wal, and storageClass are required")
	}
	return nil
}
