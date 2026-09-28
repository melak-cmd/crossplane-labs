"""Input validation and CNPG recovery manifest transformations."""

import json
from copy import deepcopy
from dataclasses import dataclass
from typing import Any

CNPG_API_VERSION = "postgresql.cnpg.io/v1"
DATABASE_API_VERSION = "database.kaonix.inc.fr/v1alpha1"
PLAN_DATA_KEY = "manifest.json"
MODES = {"prepare", "delete", "restore", "cleanup", "resume"}


class InputError(ValueError):
    """The function input or a supplied recovery resource is invalid."""


@dataclass
class RecoveryInput:
    """Validated input fields used by a recovery operation."""

    mode: str
    namespace: str
    plan_name: str | None = None
    backup_name: str | None = None
    backup_namespace: str | None = None
    target_name: str = ""
    watched_request: bool = False


def parse_input(value: dict[str, Any]) -> RecoveryInput:
    """Parse the function's versioned input structure."""
    spec = value.get("spec")
    if not isinstance(spec, dict):
        msg = "spec is required"
        raise InputError(msg)
    target = spec.get("target")
    if not isinstance(target, dict):
        msg = "target is required"
        raise InputError(msg)
    backup = spec.get("backup")
    if backup is not None and not isinstance(backup, dict):
        msg = "backup must be an object"
        raise InputError(msg)
    watched_request = spec.get("watchedRequest", False)
    if not isinstance(watched_request, bool):
        msg = "watchedRequest must be a boolean"
        raise InputError(msg)
    return RecoveryInput(
        mode=spec.get("mode", ""),
        namespace=target.get("namespace", ""),
        plan_name=spec.get("planName"),
        backup_name=backup.get("name") if backup else None,
        backup_namespace=backup.get("namespace") if backup else None,
        watched_request=watched_request,
    )


def cluster_name_from_postgresql(postgresql: dict[str, Any]) -> str:
    """Find the sole CNPG Cluster reference in the required PostgreSQL XR."""
    if (
        postgresql.get("apiVersion") != DATABASE_API_VERSION
        or postgresql.get("kind") != "PostgreSQL"
    ):
        msg = "required resource is not a PostgreSQL XR"
        raise InputError(msg)
    refs = postgresql.get("spec", {}).get("crossplane", {}).get("resourceRefs")
    if not isinstance(refs, list):
        msg = "PostgreSQL XR has no resourceRefs"
        raise InputError(msg)

    names = []
    for ref in refs:
        if not isinstance(ref, dict):
            msg = "PostgreSQL XR contains an invalid resourceRef"
            raise InputError(msg)
        if ref.get("apiVersion") == CNPG_API_VERSION and ref.get("kind") == "Cluster":
            name = ref.get("name")
            if not isinstance(name, str) or not name:
                msg = "PostgreSQL XR CNPG Cluster resourceRef has no name"
                raise InputError(msg)
            names.append(name)
    if not names:
        msg = "PostgreSQL XR has no CNPG Cluster resourceRef"
        raise InputError(msg)
    if len(names) != 1:
        msg = "PostgreSQL XR has multiple CNPG Cluster resourceRefs"
        raise InputError(msg)
    return names[0]


def validate_input(value: RecoveryInput) -> None:
    """Apply the same mode-specific validation as the Go recovery function."""
    if not value.target_name or not value.namespace:
        msg = "target name and namespace are required"
        raise InputError(msg)
    if value.mode not in MODES:
        msg = "mode must be prepare, restore, delete, cleanup, or resume"
        raise InputError(msg)
    if value.mode in {"delete", "cleanup", "resume"}:
        if value.backup_name is not None or value.backup_namespace is not None:
            msg = "delete and cleanup modes do not accept a Backup reference"
            raise InputError(msg)
        return
    if not value.plan_name:
        msg = "planName is required"
        raise InputError(msg)
    if value.mode == "prepare":
        if value.backup_name is not None or value.backup_namespace is not None:
            msg = "prepare modes do not accept a Backup reference"
            raise InputError(msg)
        return
    if not value.backup_name or not value.backup_namespace:
        msg = "backup name and namespace are required"
        raise InputError(msg)
    if value.backup_namespace != value.namespace:
        msg = "backup and target must use the same namespace"
        raise InputError(msg)


def build_recovery_plan(
    value: RecoveryInput, cluster: dict[str, Any]
) -> dict[str, Any]:
    """Create a ConfigMap containing a server-field-free Cluster manifest."""
    metadata = cluster.get("metadata")
    if (
        cluster.get("apiVersion") != CNPG_API_VERSION
        or cluster.get("kind") != "Cluster"
    ):
        msg = "required resource is not a CNPG Cluster"
        raise InputError(msg)
    if (
        not isinstance(metadata, dict)
        or metadata.get("name") != value.target_name
        or metadata.get("namespace") != value.namespace
    ):
        msg = "required CNPG Cluster does not match target"
        raise InputError(msg)

    manifest = deepcopy(cluster)
    manifest.pop("status", None)
    for field in (
        "creationTimestamp",
        "deletionGracePeriodSeconds",
        "deletionTimestamp",
        "generation",
        "managedFields",
        "resourceVersion",
        "selfLink",
        "uid",
    ):
        manifest["metadata"].pop(field, None)
    return {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {"name": value.plan_name, "namespace": value.namespace},
        "data": {PLAN_DATA_KEY: json.dumps(manifest, separators=(",", ":"))},
    }


def build_restored_cluster(
    value: RecoveryInput, plan: dict[str, Any]
) -> dict[str, Any]:
    """Build a CNPG Cluster with recovery bootstrap from a saved plan."""
    if not value.backup_name:
        msg = "restore mode requires a Backup reference"
        raise InputError(msg)
    data = plan.get("data")
    if not isinstance(data, dict) or not isinstance(data.get(PLAN_DATA_KEY), str):
        msg = "recovery plan does not contain manifest.json"
        raise InputError(msg)
    try:
        manifest = json.loads(data[PLAN_DATA_KEY])
    except json.JSONDecodeError as exc:
        msg = f"cannot decode recovery plan: {exc}"
        raise InputError(msg) from exc
    if (
        not isinstance(manifest, dict)
        or manifest.get("kind") != "Cluster"
        or manifest.get("apiVersion") != CNPG_API_VERSION
    ):
        msg = "recovery plan does not contain a CNPG Cluster manifest"
        raise InputError(msg)

    spec = manifest.setdefault("spec", {})
    if not isinstance(spec, dict):
        msg = "recovery plan contains an invalid Cluster spec"
        raise InputError(msg)
    bootstrap = spec.setdefault("bootstrap", {})
    if not isinstance(bootstrap, dict):
        msg = "recovery plan contains an invalid bootstrap"
        raise InputError(msg)
    bootstrap.pop("initdb", None)
    bootstrap["recovery"] = {"backup": {"name": value.backup_name}}
    metadata = manifest.setdefault("metadata", {})
    if not isinstance(metadata, dict):
        msg = "recovery plan contains invalid metadata"
        raise InputError(msg)
    metadata["name"] = value.target_name
    metadata["namespace"] = value.namespace
    return manifest
