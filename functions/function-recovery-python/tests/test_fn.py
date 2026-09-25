import json
import unittest
from unittest.mock import Mock

from crossplane.function import logging, resource
from crossplane.function.proto.v1 import run_function_pb2 as fnv1

from function import fn


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
        req.required_resources["recovery-plan"].items.add(
            resource=resource.dict_to_struct({"data": {}})
        )
        client = Mock()

        rsp = await fn.FunctionRunner(client).RunFunction(req, None)

        self.assertEqual(fnv1.STATUS_CONDITION_FALSE, rsp.conditions[0].status)
        self.assertEqual("InvalidRecoveryInput", rsp.conditions[0].reason)
        client.create_restored_cluster.assert_not_called()


if __name__ == "__main__":
    unittest.main()
