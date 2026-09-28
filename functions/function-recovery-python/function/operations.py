"""Visitor-based execution for the supported recovery operations."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Protocol

from kubernetes.client.exceptions import ApiException

from function.kubernetes import KubernetesClient
from function.recovery import (
    InputError,
    RecoveryInput,
    build_recovery_plan,
    build_restored_cluster,
)


class RequiredResourceNotResolved(Exception):
    """A required recovery-plan ConfigMap was not found."""


class InvalidOperationInput(Exception):
    """An operation-specific input resource is invalid."""


@dataclass(frozen=True)
class OperationContext:
    """Dependencies and request data available to an operation visitor."""

    function_input: RecoveryInput
    postgresql: dict[str, Any]
    cluster_client: KubernetesClient


class OperationVisitor(Protocol):
    """Visit each supported recovery operation."""

    def visit_prepare(
        self, operation: PrepareOperation, context: OperationContext
    ) -> None: ...
    def visit_prepare_delete(
        self, operation: PrepareDeleteOperation, context: OperationContext
    ) -> None: ...
    def visit_delete(
        self, operation: DeleteOperation, context: OperationContext
    ) -> None: ...
    def visit_restore(
        self, operation: RestoreOperation, context: OperationContext
    ) -> None: ...
    def visit_cleanup(
        self, operation: CleanupOperation, context: OperationContext
    ) -> None: ...


class RecoveryOperation(Protocol):
    """Accept a visitor that performs the operation."""

    def accept(
        self, visitor: OperationVisitor, context: OperationContext
    ) -> None: ...


class PrepareOperation:
    """Visitor element for preparing recovery."""

    def accept(self, visitor: OperationVisitor, context: OperationContext) -> None:
        visitor.visit_prepare(self, context)


class PrepareDeleteOperation:
    """Visitor element for preparing recovery and deleting the Cluster."""

    def accept(self, visitor: OperationVisitor, context: OperationContext) -> None:
        visitor.visit_prepare_delete(self, context)


class DeleteOperation:
    """Visitor element for deleting the target Cluster."""

    def accept(self, visitor: OperationVisitor, context: OperationContext) -> None:
        visitor.visit_delete(self, context)


class RestoreOperation:
    """Visitor element for restoring the target Cluster."""

    def accept(self, visitor: OperationVisitor, context: OperationContext) -> None:
        visitor.visit_restore(self, context)


class CleanupOperation:
    """Visitor element for removing the recovery bootstrap."""

    def accept(self, visitor: OperationVisitor, context: OperationContext) -> None:
        visitor.visit_cleanup(self, context)


class RecoveryOperationVisitor:
    """Perform recovery actions for each operation element."""

    def visit_prepare(
        self, _operation: PrepareOperation, context: OperationContext
    ) -> None:
        value = context.function_input
        cluster = context.cluster_client.get_cluster(value.namespace, value.target_name)
        plan = build_recovery_plan(value, cluster)
        context.cluster_client.prepare_recovery(context.postgresql, plan)

    def visit_prepare_delete(
        self, _operation: PrepareDeleteOperation, context: OperationContext
    ) -> None:
        self.visit_prepare(PrepareOperation(), context)
        self.visit_delete(DeleteOperation(), context)

    def visit_delete(
        self, _operation: DeleteOperation, context: OperationContext
    ) -> None:
        value = context.function_input
        context.cluster_client.delete_and_wait(value.namespace, value.target_name)

    def visit_restore(
        self, _operation: RestoreOperation, context: OperationContext
    ) -> None:
        value = context.function_input
        if not value.plan_name:
            raise InvalidOperationInput("planName is required")
        try:
            plan = context.cluster_client.get_recovery_plan(
                value.namespace, value.plan_name
            )
        except ApiException as exc:
            if exc.status == 404:
                raise RequiredResourceNotResolved(
                    "required recovery plan ConfigMap is not found"
                ) from exc
            raise
        try:
            cluster = build_restored_cluster(value, plan)
        except InputError as exc:
            raise InvalidOperationInput(str(exc)) from exc
        context.cluster_client.create_restored_cluster(cluster)

    def visit_cleanup(
        self, _operation: CleanupOperation, context: OperationContext
    ) -> None:
        value = context.function_input
        context.cluster_client.remove_recovery(value.namespace, value.target_name)


class OperationDispatcher:
    """Look up an operation and pass it to the recovery visitor."""

    def __init__(self, visitor: OperationVisitor | None = None) -> None:
        self._operations: dict[str, RecoveryOperation] = {
            "prepare": PrepareOperation(),
            "prepare-delete": PrepareDeleteOperation(),
            "delete": DeleteOperation(),
            "restore": RestoreOperation(),
            "cleanup": CleanupOperation(),
        }
        self._visitor = visitor or RecoveryOperationVisitor()

    def execute(self, context: OperationContext) -> None:
        """Visit the operation corresponding to the validated input mode."""
        self._operations[context.function_input.mode].accept(self._visitor, context)