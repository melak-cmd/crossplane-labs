package model

const (
	// PlanAnnotation holds the RecoveryPlan, a sanitized copy of the CNPG
	// Cluster manifest, on the PostgreSQLRestore.
	PlanAnnotation = "recovery.database.nuagik.sncf.fr/plan"
	// PhaseAnnotation records how far the recovery has progressed on the
	// PostgreSQLRestore.
	PhaseAnnotation = "recovery.database.nuagik.sncf.fr/phase"
)

// Phase is the recovery progress recorded on the PostgreSQLRestore.
type Phase string

const (
	PhaseNone     Phase = ""
	PhasePrepared Phase = "prepared"
	PhaseDeleted  Phase = "deleted"
	PhaseRestored Phase = "restored"
	PhaseCleaned  Phase = "cleaned"
	PhaseResumed  Phase = "resumed"
)

// RestoreRef identifies the PostgreSQLRestore that carries the plan and phase.
type RestoreRef struct {
	Namespace string
	Name      string
}

// PhaseGate returns the phase an operation requires before it may run and the
// phase that is recorded once it succeeds. Operations that are invoked in any
// other phase are duplicates or out of order and must not run.
func PhaseGate(operation Operation) (required, next Phase, ok bool) {
	switch operation {
	case OperationPrepare:
		return PhaseNone, PhasePrepared, true
	case OperationDelete:
		return PhasePrepared, PhaseDeleted, true
	case OperationRestore:
		return PhaseDeleted, PhaseRestored, true
	case OperationCleanup:
		return PhaseRestored, PhaseCleaned, true
	case OperationResume:
		return PhaseCleaned, PhaseResumed, true
	case OperationPrepareDelete:
		return PhaseNone, PhaseDeleted, true
	}
	return PhaseNone, PhaseNone, false
}
