"""A Crossplane Operation Function for CNPG recovery."""

import asyncio

import grpc
from crossplane.function import logging, request, resource, response
from crossplane.function.proto.v1 import run_function_pb2 as fnv1
from crossplane.function.proto.v1 import run_function_pb2_grpc as grpcv1

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

CONDITION_SUCCESS = "FunctionSuccess"
REASON_SUCCESS = "Success"
REASON_INVALID = "InvalidRecoveryInput"
REASON_UNRESOLVED = "InvalidRecoveryInput"


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

        try:
            function_input = parse_input(resource.struct_to_dict(req.input))
            postgresql_resources = request.get_required_resources(req, "postgresql")
            if len(postgresql_resources) != 1:
                _set_condition(
                    rsp,
                    fnv1.STATUS_CONDITION_FALSE,
                    REASON_UNRESOLVED,
                    "required PostgreSQL XR is unresolved",
                )
                return rsp
            postgresql = postgresql_resources[0]
            cluster_name = cluster_name_from_postgresql(postgresql)
            function_input.target_name = cluster_name
            validate_input(function_input)
        except InputError as exc:
            _set_condition(rsp, fnv1.STATUS_CONDITION_FALSE, REASON_INVALID, str(exc))
            return rsp
        except Exception as exc:
            response.fatal(rsp, f"cannot read recovery input: {exc}")
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
                OperationContext(function_input, postgresql, req, self.cluster_client),
            )
        except RequiredResourceNotResolved as exc:
            _set_condition(rsp, fnv1.STATUS_CONDITION_FALSE, REASON_UNRESOLVED, str(exc))
        except InvalidOperationInput as exc:
            _set_condition(rsp, fnv1.STATUS_CONDITION_FALSE, REASON_INVALID, str(exc))
        except Exception as exc:
            response.fatal(rsp, f"cannot execute recovery operation: {exc}")

        return rsp
        _set_condition(rsp, fnv1.STATUS_CONDITION_TRUE, REASON_SUCCESS)


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
