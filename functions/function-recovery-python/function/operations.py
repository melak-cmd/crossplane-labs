"""Command objects for the supported recovery operations."""

from dataclasses import dataclass
from typing import Any, Protocol

from crossplane.function import request
from crossplane.function.proto.v1 import run_function_pb2 as fnv1

from function.kubernetes import KubernetesClient
from function.recovery import (
    InputError,
    RecoveryInput,
    build_recovery_plan,
    build_restored_cluster,
)


class RequiredResourceNotResolved(Exception):
    """A required resource has not been resolved by Crossplane."""


class InvalidOperationInput(Exception):
    """An operation-specific input resource is invalid."""


@dataclass(frozen=True)
class OperationContext:
    """Dependencies and request data available to an operation command."""

    function_input: RecoveryInput
    postgresql: dict[str, Any]
    request: fnv1.RunFunctionRequest
    cluster_client: KubernetesClient


class RecoveryCommand(Protocol):
    """Execute one recovery action using the supplied operation context."""

    def execute(self, context: OperationContext) -> None: ...


class PrepareCommand:
    """Pause the PostgreSQL XR and persist the current Cluster manifest."""

    def execute(self, context: OperationContext) -> None:
        value = context.function_input
        cluster = context.cluster_client.get_cluster(value.namespace, value.target_name)
        plan = build_recovery_plan(value, cluster)
        context.cluster_client.prepare_recovery(context.postgresql, plan)


class DeleteCommand:
    """Delete the target CNPG Cluster and wait for it to disappear."""

    def execute(self, context: OperationContext) -> None:
        value = context.function_input
        context.cluster_client.delete_and_wait(value.namespace, value.target_name)


class PrepareDeleteCommand:
    """Persist a recovery plan, then delete the target CNPG Cluster."""

    def execute(self, context: OperationContext) -> None:
        PrepareCommand().execute(context)
        DeleteCommand().execute(context)


class RestoreCommand:
    """Create the restored Cluster from a required recovery-plan ConfigMap."""

    def execute(self, context: OperationContext) -> None:
        plans = request.get_required_resources(context.request, "recovery-plan")
        if len(plans) != 1:
            raise RequiredResourceNotResolved(
                "required recovery resource is not resolved"
            )
        try:
            cluster = build_restored_cluster(context.function_input, plans[0])
        except InputError as exc:
            raise InvalidOperationInput(str(exc)) from exc
        context.cluster_client.create_restored_cluster(cluster)


class CleanupCommand:
    """Remove only spec.bootstrap.recovery from the target Cluster."""

    def execute(self, context: OperationContext) -> None:
        value = context.function_input
        context.cluster_client.remove_recovery(value.namespace, value.target_name)


class OperationDispatcher:
    """Select and execute the command registered for a recovery mode."""

    def __init__(self) -> None:
        self._commands: dict[str, RecoveryCommand] = {
            "prepare": PrepareCommand(),
            "prepare-delete": PrepareDeleteCommand(),
            "delete": DeleteCommand(),
            "restore": RestoreCommand(),
            "cleanup": CleanupCommand(),
        }

    def execute(self, context: OperationContext) -> None:
        """Execute the command corresponding to the validated input mode."""
        self._commands[context.function_input.mode].execute(context)