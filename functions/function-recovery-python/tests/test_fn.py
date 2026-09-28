import json
import unittest
from unittest.mock import Mock

from crossplane.function import logging, resource
from crossplane.function.proto.v1 import run_function_pb2 as fnv1

from function import fn
from function.operations import OperationContext, OperationDispatcher
from function.recovery import RecoveryInput


class TestFunctionRunner(unittest.IsolatedAsyncioTestCase):
    def setUp(self) -> None:
        logging.configure(level=logging.Level.DISABLED)

    async def test_prepare_persists_plan_and_returns_operation_condition(self) -> None:
        client = Mock()
        client.get_cluster.return_value = {
            "apiVersion": "postgresql.cnpg.io/v1",
            "kind": "Cluster",
            "metadata": {"name": "orders-primary", "namespace": "platform"},
            "spec": {"bootstrap": {"initdb": {"database": "orders"}}},
            "status": {"conditions": []},
        }
        postgresql = {
            "apiVersion": "database.kaonix.inc.fr/v1alpha1",
            "kind": "PostgreSQL",
            "metadata": {"name": "orders", "namespace": "platform"},
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
        req = fnv1.RunFunctionRequest(
            input=resource.dict_to_struct(
                {
                    "apiVersion": "function-recovery-python.fn.kaonix.com/v1beta1",
                    "kind": "Input",
                    "spec": {
                        "mode": "prepare",
                        "target": {"namespace": "platform"},
                        "planName": "orders-plan",
                    },
                }
            ),
        )
        req.required_resources["postgresql"].items.add(
            resource=resource.dict_to_struct(postgresql)
        )

        rsp = await fn.FunctionRunner(client).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_TRUE, rsp.conditions[0].status)
        self.assertEqual("FunctionSuccess", rsp.conditions[0].type)
        self.assertEqual("Success", rsp.conditions[0].reason)
        self.assertEqual(
            "prepare operation completed successfully", rsp.conditions[0].message
        )
        self.assertEqual(
            {
                "operation": "prepare",
                "status": "Succeeded",
                "message": "prepare operation completed successfully",
            },
            resource.struct_to_dict(rsp.output),
        )
        client.prepare_recovery.assert_called_once()
        persisted_plan = client.prepare_recovery.call_args.args[1]
        self.assertEqual("orders-plan", persisted_plan["metadata"]["name"])
        self.assertNotIn("status", json.loads(persisted_plan["data"]["manifest.json"]))

    async def test_unresolved_postgresql_returns_false_condition(self) -> None:
        req = fnv1.RunFunctionRequest(
            input=resource.dict_to_struct(
                {
                    "spec": {
                        "mode": "delete",
                        "target": {"namespace": "platform"},
                    }
                }
            )
        )

        rsp = await fn.FunctionRunner(Mock()).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_FALSE, rsp.conditions[0].status)
        self.assertEqual("InvalidRecoveryInput", rsp.conditions[0].reason)
        self.assertIn("PostgreSQL XR is unresolved", rsp.conditions[0].message)
        self.assertEqual(
            {
                "operation": "delete",
                "status": "Failed",
                "message": "required PostgreSQL XR is unresolved",
            },
            resource.struct_to_dict(rsp.output),
        )

    async def test_watched_restore_request_resumes_dynamic_postgresql(self) -> None:
        postgresql = {
            "apiVersion": "database.kaonix.inc.fr/v1alpha1",
            "kind": "PostgreSQL",
            "metadata": {"name": "orders", "namespace": "platform"},
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
        restore_request = {
            "apiVersion": "database.kaonix.inc.fr/v1alpha1",
            "kind": "DatabaseRestore",
            "metadata": {"name": "orders"},
            "spec": {
                "backupName": "orders-backup",
                "target": {"namespace": "platform"},
            },
        }
        req = fnv1.RunFunctionRequest(
            input=resource.dict_to_struct(
                {
                    "spec": {
                        "mode": "resume",
                        "target": {},
                        "watchedRequest": True,
                    }
                }
            )
        )
        req.required_resources["ops.crossplane.io/watched-resource"].items.add(
            resource=resource.dict_to_struct(restore_request)
        )
        client = Mock()
        client.get_postgresql.return_value = postgresql

        rsp = await fn.FunctionRunner(client).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_TRUE, rsp.conditions[0].status)
        client.acknowledge_restore_request.assert_called_once_with("orders")
        client.get_postgresql.assert_called_once_with("platform", "orders")
        client.resume_postgresql.assert_called_once_with("platform", "orders")

    async def test_watched_tombstone_is_successful_noop(self) -> None:
        req = fnv1.RunFunctionRequest(
            input=resource.dict_to_struct(
                {
                    "spec": {
                        "mode": "resume",
                        "target": {},
                        "watchedRequest": True,
                    }
                }
            )
        )
        client = Mock()

        rsp = await fn.FunctionRunner(client).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_TRUE, rsp.conditions[0].status)
        self.assertEqual("Succeeded", resource.struct_to_dict(rsp.output)["status"])
        client.get_postgresql.assert_not_called()
        client.resume_postgresql.assert_not_called()

    async def test_invalid_recovery_plan_returns_false_condition(self) -> None:
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
        req = fnv1.RunFunctionRequest(
            input=resource.dict_to_struct(
                {
                    "spec": {
                        "mode": "restore",
                        "target": {"namespace": "platform"},
                        "planName": "orders-plan",
                        "backup": {"name": "orders-backup", "namespace": "platform"},
                    }
                }
            )
        )
        req.required_resources["postgresql"].items.add(
            resource=resource.dict_to_struct(postgresql)
        )
        client = Mock()
        client.get_recovery_plan.return_value = {"data": {}}

        rsp = await fn.FunctionRunner(client).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_FALSE, rsp.conditions[0].status)
        self.assertEqual("InvalidRecoveryInput", rsp.conditions[0].reason)
        client.get_recovery_plan.assert_called_once_with("platform", "orders-plan")
        client.create_restored_cluster.assert_not_called()


class TestOperationDispatcher(unittest.TestCase):
    def test_prepare_delete_persists_plan_before_deleting_cluster(self) -> None:
        client = Mock()
        client.get_cluster.return_value = {
            "apiVersion": "postgresql.cnpg.io/v1",
            "kind": "Cluster",
            "metadata": {"name": "orders-primary", "namespace": "platform"},
        }
        context = OperationContext(
            RecoveryInput(
                mode="prepare-delete",
                namespace="platform",
                plan_name="orders-plan",
                target_name="orders-primary",
            ),
            postgresql={},
            cluster_client=client,
        )

        OperationDispatcher().execute(context)

        self.assertEqual(
            ["get_cluster", "prepare_recovery", "delete_and_wait"],
            [call[0] for call in client.method_calls],
        )

    def test_delete_and_cleanup_dispatch_to_their_commands(self) -> None:
        for mode, method in (
            ("delete", "delete_and_wait"),
            ("cleanup", "remove_recovery"),
            ("resume", "resume_postgresql"),
        ):
            with self.subTest(mode=mode):
                client = Mock()
                context = OperationContext(
                    RecoveryInput(
                        mode=mode,
                        namespace="platform",
                        target_name="orders-primary",
                    ),
                    postgresql={"metadata": {"name": "orders"}},
                    cluster_client=client,
                )

                OperationDispatcher().execute(context)

                getattr(client, method).assert_called_once_with(
                    "platform", "orders" if mode == "resume" else "orders-primary"
                )


if __name__ == "__main__":
    unittest.main()
