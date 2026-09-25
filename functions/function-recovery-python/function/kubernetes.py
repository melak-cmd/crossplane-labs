"""Kubernetes operations used by the recovery function."""

import time
from collections.abc import Callable
from typing import Any

from kubernetes import client, config, dynamic
from kubernetes.client.exceptions import ApiException

NOT_FOUND = 404
ALREADY_EXISTS = 409


class KubernetesClient:
    """Perform namespaced CNPG and ConfigMap operations using in-cluster auth."""

    def __init__(
        self,
        dynamic_client: Any | None = None,
        interval: float = 1.0,
        timeout: float = 300.0,
    ) -> None:
        """Configure an optional client and the polling interval and timeout."""
        self._dynamic_client = dynamic_client
        self.interval = interval
        self.timeout = timeout

    @property
    def dynamic_client(self) -> Any:
        """Lazily create the in-cluster Kubernetes dynamic client."""
        if self._dynamic_client is None:
            config.load_incluster_config()
            self._dynamic_client = dynamic.DynamicClient(client.ApiClient())
        return self._dynamic_client

    def _resource(self, api_version: str, kind: str) -> Any:
        return self.dynamic_client.resources.get(api_version=api_version, kind=kind)

    def get_cluster(self, namespace: str, name: str) -> dict[str, Any]:
        """Fetch a namespaced CNPG Cluster as a dictionary."""
        return (
            self._resource("postgresql.cnpg.io/v1", "Cluster")
            .get(name=name, namespace=namespace)
            .to_dict()
        )

    def prepare_recovery(
        self, postgresql: dict[str, Any], plan: dict[str, Any]
    ) -> None:
        """Pause the PostgreSQL XR and create or update its recovery plan."""
        metadata = postgresql.get("metadata", {})
        namespace = metadata.get("namespace", "")
        name = metadata.get("name", "")
        postgresqls = self._resource("database.kaonix.inc.fr/v1alpha1", "PostgreSQL")
        postgresqls.get(name=name, namespace=namespace)
        postgresqls.patch(
            name=name,
            namespace=namespace,
            body={"metadata": {"annotations": {"crossplane.io/paused": "true"}}},
            content_type="application/merge-patch+json",
        )

        plan = {**plan, "metadata": {**plan["metadata"], "namespace": namespace}}
        configmaps = self._resource("v1", "ConfigMap")
        try:
            current = configmaps.get(
                name=plan["metadata"]["name"], namespace=namespace
            ).to_dict()
        except ApiException as exc:
            if exc.status != NOT_FOUND:
                raise
            configmaps.create(body=plan, namespace=namespace)
            return
        plan["metadata"]["resourceVersion"] = current["metadata"]["resourceVersion"]
        configmaps.replace(
            name=plan["metadata"]["name"], namespace=namespace, body=plan
        )

    def delete_and_wait(self, namespace: str, name: str) -> None:
        """Delete a Cluster and wait until it is absent from the API."""
        clusters = self._resource("postgresql.cnpg.io/v1", "Cluster")
        try:
            clusters.delete(name=name, namespace=namespace)
        except ApiException as exc:
            if exc.status != NOT_FOUND:
                raise

        def deleted() -> bool:
            try:
                clusters.get(name=name, namespace=namespace)
            except ApiException as exc:
                if exc.status == NOT_FOUND:
                    return True
                raise
            return False

        self._wait(deleted, f"CNPG Cluster {namespace}/{name} was not deleted")

    def create_restored_cluster(self, cluster: dict[str, Any]) -> None:
        """Create a restored Cluster and wait for its Ready condition."""
        metadata = cluster["metadata"]
        namespace = metadata["namespace"]
        name = metadata["name"]
        clusters = self._resource("postgresql.cnpg.io/v1", "Cluster")
        try:
            clusters.create(body=cluster, namespace=namespace)
        except ApiException as exc:
            if exc.status != ALREADY_EXISTS:
                raise
            current = clusters.get(name=name, namespace=namespace).to_dict()
            expected_recovery = _nested(cluster, "spec", "bootstrap", "recovery")
            current_recovery = _nested(current, "spec", "bootstrap", "recovery")
            if current_recovery is None or current_recovery != expected_recovery:
                msg = (
                    f"CNPG Cluster {namespace}/{name} already exists "
                    "without the requested recovery bootstrap"
                )
                raise RuntimeError(msg) from exc

        def ready() -> bool:
            current = clusters.get(name=name, namespace=namespace).to_dict()
            conditions = _nested(current, "status", "conditions") or []
            return any(
                condition.get("type") == "Ready" and condition.get("status") == "True"
                for condition in conditions
                if isinstance(condition, dict)
            )

        self._wait(ready, f"CNPG Cluster {namespace}/{name} did not become Ready")

    def remove_recovery(self, namespace: str, name: str) -> None:
        """Remove only spec.bootstrap.recovery from the selected Cluster."""
        clusters = self._resource("postgresql.cnpg.io/v1", "Cluster")
        cluster = clusters.get(name=name, namespace=namespace).to_dict()
        if _nested(cluster, "spec", "bootstrap", "recovery") is None:
            return
        clusters.patch(
            name=name,
            namespace=namespace,
            body=[{"op": "remove", "path": "/spec/bootstrap/recovery"}],
            content_type="application/json-patch+json",
        )

    def _wait(self, predicate: Callable[[], bool], message: str) -> None:
        deadline = time.monotonic() + self.timeout
        while True:
            if predicate():
                return
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError(message)
            time.sleep(min(self.interval, remaining))


def _nested(value: dict[str, Any], *path: str) -> Any:
    current: Any = value
    for key in path:
        if not isinstance(current, dict):
            return None
        current = current.get(key)
    return current
