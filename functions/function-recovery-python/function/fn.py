"""A Crossplane Operation Function for CNPG recovery."""

import asyncio

import grpc
from crossplane.function import logging, request, resource, response
from crossplane.function.proto.v1 import run_function_pb2 as fnv1
from crossplane.function.proto.v1 import run_function_pb2_grpc as grpcv1

from function.constants import (
    CONDITION_SUCCESS,
    MESSAGE_INPUT_READ_FAILURE,
    MESSAGE_OPERATION_EXECUTION_FAILURE,
    MESSAGE_OPERATION_FAILURE,
    MESSAGE_OPERATION_SUCCESS,
    MESSAGE_POSTGRESQL_UNRESOLVED,
    OPERATION_STATUS_FAILED,
    OPERATION_STATUS_SUCCEEDED,
    OPERATION_UNKNOWN,
    REASON_INVALID,
    REASON_SUCCESS,
    REASON_UNRESOLVED,
)
from function.kubernetes import KubernetesClient
from function.operations import (
    InvalidOperationInput,
    OperationContext,
    OperationDispatcher,
    RequiredResourceNotResolved,
)
from function.recovery import (
    InputError,
    cluster_name_from_postgresql,
    parse_input,
    validate_input,
)
from function.status import (
    FunctionResponseStatusObserver,
    OperationStatusEvent,
    OperationStatusSubject,
)

class FunctionRunner(grpcv1.FunctionRunnerService):
    """Handle recovery operations over the Crossplane Function protocol."""

    def __init__(self, cluster_client: KubernetesClient | None = None):
        """Create a runner, optionally injecting a Kubernetes client for tests."""
        self.log = logging.get_logger()
        self.cluster_client = cluster_client or KubernetesClient()
        self.operation_dispatcher = OperationDispatcher()

    async def RunFunction(
        self, req: fnv1.RunFunctionRequest, _: grpc.aio.ServicerContext
    ) -> fnv1.RunFunctionResponse:
        """Run the requested recovery operation."""
        log = self.log.bind(tag=req.meta.tag)
        log.info("Running recovery function")

        rsp = response.to(req)
        status_subject = OperationStatusSubject()
        status_subject.attach(FunctionResponseStatusObserver(rsp))
        input_data = resource.struct_to_dict(req.input)
        operation = _operation_mode(input_data)

        try:
            function_input = parse_input(input_data)
            operation = function_input.mode
            postgresql_resources = request.get_required_resources(req, "postgresql")
            if len(postgresql_resources) != 1:
                message = MESSAGE_POSTGRESQL_UNRESOLVED
                _set_condition(
                    rsp,
                    fnv1.STATUS_CONDITION_FALSE,
                    REASON_UNRESOLVED,
                    MESSAGE_OPERATION_FAILURE.format(
                        operation=operation, message=message
                    ),
                )
                status_subject.notify(
                    OperationStatusEvent(operation, OPERATION_STATUS_FAILED, message)
                )
                return rsp
            postgresql = postgresql_resources[0]
            cluster_name = cluster_name_from_postgresql(postgresql)
            function_input.target_name = cluster_name
            validate_input(function_input)
        except InputError as exc:
            _set_condition(rsp, fnv1.STATUS_CONDITION_FALSE, REASON_INVALID, str(exc))
            status_subject.notify(
                OperationStatusEvent(operation, OPERATION_STATUS_FAILED, str(exc))
            )
            return rsp
        except Exception as exc:
            message = MESSAGE_INPUT_READ_FAILURE.format(error=exc)
            response.fatal(rsp, message)
            status_subject.notify(
                OperationStatusEvent(operation, OPERATION_STATUS_FAILED, message)
            )
            return rsp

        log.info(
            "Running recovery operation",
            mode=function_input.mode,
            postgresql=postgresql.get("metadata", {}).get("name", ""),
            cluster=cluster_name,
        )
        try:
            await asyncio.to_thread(
                self.operation_dispatcher.execute,
                OperationContext(function_input, postgresql, self.cluster_client),
            )
        except RequiredResourceNotResolved as exc:
            message = str(exc)
            _set_condition(
                rsp,
                fnv1.STATUS_CONDITION_FALSE,
                REASON_UNRESOLVED,
                MESSAGE_OPERATION_FAILURE.format(
                    operation=operation, message=message
                ),
            )
            status_subject.notify(
                OperationStatusEvent(operation, OPERATION_STATUS_FAILED, message)
            )
        except InvalidOperationInput as exc:
            message = str(exc)
            _set_condition(
                rsp,
                fnv1.STATUS_CONDITION_FALSE,
                REASON_INVALID,
                MESSAGE_OPERATION_FAILURE.format(
                    operation=operation, message=message
                ),
            )
            status_subject.notify(
                OperationStatusEvent(operation, OPERATION_STATUS_FAILED, message)
            )
        except Exception as exc:
            message = MESSAGE_OPERATION_EXECUTION_FAILURE.format(
                operation=operation, error=exc
            )
            response.fatal(rsp, message)
            status_subject.notify(
                OperationStatusEvent(operation, OPERATION_STATUS_FAILED, message)
            )
        else:
            message = MESSAGE_OPERATION_SUCCESS.format(operation=operation)
            _set_condition(
                rsp,
                fnv1.STATUS_CONDITION_TRUE,
                REASON_SUCCESS,
                message,
            )
            status_subject.notify(
                OperationStatusEvent(
                    operation, OPERATION_STATUS_SUCCEEDED, message
                )
            )

        return rsp


def _operation_mode(value: dict) -> str:
    spec = value.get("spec")
    if not isinstance(spec, dict):
        return OPERATION_UNKNOWN
    mode = spec.get("mode")
    return mode if isinstance(mode, str) and mode else OPERATION_UNKNOWN


def _set_condition(
    rsp: fnv1.RunFunctionResponse,
    status: int,
    reason: str,
    message: str = "",
) -> None:
    condition = rsp.conditions.add(
        type=CONDITION_SUCCESS,
        status=status,
        reason=reason,
        target=fnv1.TARGET_COMPOSITE,
    )
    if message:
        condition.message = message
