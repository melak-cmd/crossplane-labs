import json
import unittest

from function.recovery import (
    InputError,
    RecoveryInput,
    build_recovery_plan,
    build_restored_cluster,
    cluster_name_from_postgresql,
    validate_input,
)


class TestRecovery(unittest.TestCase):
    def test_resume_requires_only_a_target(self) -> None:
        validate_input(
            RecoveryInput(
                mode="resume",
                namespace="platform",
                target_name="orders",
            )
        )

    def test_recovery_plan_removes_server_fields_without_mutating_source(self) -> None:
        cluster = {
            "apiVersion": "postgresql.cnpg.io/v1",
            "kind": "Cluster",
            "metadata": {
                "name": "orders-primary",
                "namespace": "platform",
                "resourceVersion": "42",
            },
            "spec": {"instances": 2},
            "status": {"phase": "Healthy"},
        }
        value = RecoveryInput(
            mode="prepare",
            namespace="platform",
            plan_name="orders-plan",
            target_name="orders-primary",
        )

        plan = build_recovery_plan(value, cluster)

        manifest = json.loads(plan["data"]["manifest.json"])
        self.assertNotIn("status", manifest)
        self.assertNotIn("resourceVersion", manifest["metadata"])
        self.assertEqual("42", cluster["metadata"]["resourceVersion"])

    def test_restore_replaces_initdb_with_recovery_bootstrap(self) -> None:
        manifest = {
            "apiVersion": "postgresql.cnpg.io/v1",
            "kind": "Cluster",
            "metadata": {"name": "old", "namespace": "platform"},
            "spec": {
                "bootstrap": {"initdb": {"database": "orders"}},
                "instances": 2,
            },
        }
        value = RecoveryInput(
            mode="restore",
            namespace="platform",
            plan_name="orders-plan",
            backup_name="orders-backup",
            backup_namespace="platform",
            target_name="orders-primary",
        )

        restored = build_restored_cluster(
            value, {"data": {"manifest.json": json.dumps(manifest)}}
        )

        self.assertEqual("orders-primary", restored["metadata"]["name"])
        self.assertEqual(2, restored["spec"]["instances"])
        self.assertNotIn("initdb", restored["spec"]["bootstrap"])
        self.assertEqual(
            {"backup": {"name": "orders-backup"}},
            restored["spec"]["bootstrap"]["recovery"],
        )

    def test_cluster_name_requires_exactly_one_cnpg_cluster_ref(self) -> None:
        postgresql = {
            "apiVersion": "database.kaonix.inc.fr/v1alpha1",
            "kind": "PostgreSQL",
            "spec": {
                "crossplane": {
                    "resourceRefs": [
                        {
                            "apiVersion": "postgresql.cnpg.io/v1",
                            "kind": "Cluster",
                            "name": "orders-primary",
                        }
                    ]
                }
            },
        }

        self.assertEqual("orders-primary", cluster_name_from_postgresql(postgresql))
        postgresql["spec"]["crossplane"]["resourceRefs"].append(
            {
                "apiVersion": "postgresql.cnpg.io/v1",
                "kind": "Cluster",
                "name": "orders-replica",
            }
        )
        with self.assertRaisesRegex(InputError, "multiple CNPG Cluster"):
            cluster_name_from_postgresql(postgresql)

    def test_restore_requires_backup_and_same_namespace(self) -> None:
        with self.assertRaisesRegex(InputError, "same namespace"):
            validate_input(
                RecoveryInput(
                    mode="restore",
                    namespace="platform",
                    plan_name="orders-plan",
                    backup_name="orders-backup",
                    backup_namespace="other",
                    target_name="orders-primary",
                )
            )


if __name__ == "__main__":
    unittest.main()
