package validation

import (
	"fmt"

	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/model"
)

func Input(in *model.Input) error {
	if in.Spec.Target.Name == "" || in.Spec.Target.Namespace == "" {
		return fmt.Errorf("target name and namespace are required")
	}
	if in.Spec.Mode != string(model.OperationPrepare) && in.Spec.Mode != string(model.OperationRestore) && in.Spec.Mode != string(model.OperationDelete) && in.Spec.Mode != string(model.OperationCleanup) && in.Spec.Mode != string(model.OperationPrepareDelete) && in.Spec.Mode != string(model.OperationResume) {
		return fmt.Errorf("mode must be prepare, restore, delete, cleanup, prepare-delete, or resume")
	}
	if in.Spec.Mode == string(model.OperationDelete) || in.Spec.Mode == string(model.OperationCleanup) || in.Spec.Mode == string(model.OperationResume) {
		if in.Spec.Backup != nil {
			return fmt.Errorf("delete and cleanup modes do not accept a Backup reference")
		}
		return nil
	}
	if in.Spec.PlanName == "" {
		return fmt.Errorf("planName is required")
	}
	if in.Spec.Mode == string(model.OperationPrepare) || in.Spec.Mode == string(model.OperationPrepareDelete) {
		if in.Spec.Backup != nil {
			return fmt.Errorf("prepare modes do not accept a Backup reference")
		}
		return nil
	}
	if in.Spec.Backup == nil {
		return fmt.Errorf("restore mode requires a Backup reference")
	}
	if in.Spec.Backup.Name == "" || in.Spec.Backup.Namespace == "" {
		return fmt.Errorf("backup name and namespace are required")
	}
	if in.Spec.Backup.Namespace != in.Spec.Target.Namespace {
		return fmt.Errorf("backup and target must use the same namespace")
	}
	return nil
}
