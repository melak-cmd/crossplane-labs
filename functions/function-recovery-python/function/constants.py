"""Shared condition and Operation status constants."""

CONDITION_SUCCESS = "FunctionSuccess"
REASON_SUCCESS = "Success"
REASON_INVALID = "InvalidRecoveryInput"
REASON_UNRESOLVED = "InvalidRecoveryInput"

OPERATION_STATUS_SUCCEEDED = "Succeeded"
OPERATION_STATUS_FAILED = "Failed"
OPERATION_UNKNOWN = "unknown"

MESSAGE_POSTGRESQL_UNRESOLVED = "required PostgreSQL XR is unresolved"
MESSAGE_INPUT_READ_FAILURE = "cannot read recovery input: {error}"
MESSAGE_OPERATION_FAILURE = "{operation} operation: {message}"
MESSAGE_OPERATION_EXECUTION_FAILURE = "cannot execute {operation} operation: {error}"
MESSAGE_OPERATION_SUCCESS = "{operation} operation completed successfully"