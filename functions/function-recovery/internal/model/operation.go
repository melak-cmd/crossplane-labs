package model

type Operation string

const (
	OperationPrepare       Operation = "prepare"
	OperationRestore       Operation = "restore"
	OperationDelete        Operation = "delete"
	OperationCleanup       Operation = "cleanup"
	OperationPrepareDelete Operation = "prepare-delete"
)
