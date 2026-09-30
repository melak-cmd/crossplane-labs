package model

const (
	ConditionSuccess          = "FunctionSuccess"
	ReasonSuccess             = "Success"
	ReasonInvalid             = "InvalidRecoveryInput"
	OperationStatusSucceeded  = "Succeeded"
	OperationStatusFailed     = "Failed"
	OperationUnknown          = "unknown"
	MessageOperationSucceeded = "%s operation completed successfully"
	MessageOperationFailed    = "%s operation failed: %s"
	MessageOperationSkipped   = "%s operation skipped: recovery phase is %q"
)
