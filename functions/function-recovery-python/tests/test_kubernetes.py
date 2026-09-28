import unittest
from unittest.mock import Mock

from kubernetes.client.exceptions import ApiException

from function.kubernetes import KubernetesClient


class TestKubernetesClient(unittest.TestCase):
    def test_get_recovery_plan_fetches_named_configmap(self) -> None:
        configmaps = Mock()
        configmaps.get.return_value.to_dict.return_value = {
            "data": {"manifest.json": "{}"}
        }
        dynamic_client = Mock()
        dynamic_client.resources.get.return_value = configmaps

        plan = KubernetesClient(dynamic_client).get_recovery_plan(
            "platform", "orders-plan"
        )

        self.assertEqual({"data": {"manifest.json": "{}"}}, plan)
        dynamic_client.resources.get.assert_called_once_with(
            api_version="v1", kind="ConfigMap"
        )
        configmaps.get.assert_called_once_with(
            name="orders-plan", namespace="platform"
        )

    def test_prepare_pauses_postgresql_before_creating_plan(self) -> None:
        postgresqls = Mock()
        configmaps = Mock()
        configmaps.get.side_effect = ApiException(status=404)
        dynamic_client = Mock()
        dynamic_client.resources.get.side_effect = [postgresqls, configmaps]
        plan = {
            "metadata": {"name": "orders-plan", "namespace": "platform"},
            "data": {"manifest.json": "{}"},
        }

        KubernetesClient(dynamic_client).prepare_recovery(
            {"metadata": {"name": "orders", "namespace": "platform"}}, plan
        )

        postgresqls.get.assert_called_once_with(name="orders", namespace="platform")
        postgresqls.patch.assert_called_once_with(
            name="orders",
            namespace="platform",
            body={"metadata": {"annotations": {"crossplane.io/paused": "true"}}},
            content_type="application/merge-patch+json",
        )
        configmaps.create.assert_called_once()
        self.assertEqual("platform", configmaps.create.call_args.kwargs["namespace"])

    def test_delete_waits_until_cluster_is_absent(self) -> None:
        clusters = Mock()
        clusters.get.side_effect = ApiException(status=404)
        dynamic_client = Mock()
        dynamic_client.resources.get.return_value = clusters

        KubernetesClient(dynamic_client, interval=0, timeout=0.01).delete_and_wait(
            "platform", "orders-primary"
        )

        clusters.delete.assert_called_once_with(
            name="orders-primary", namespace="platform"
        )

    def test_restore_creates_cluster_and_waits_for_ready(self) -> None:
        clusters = Mock()
        clusters.get.return_value.to_dict.return_value = {
            "status": {
                "conditions": [{"type": "Ready", "status": "True"}],
            }
        }
        dynamic_client = Mock()
        dynamic_client.resources.get.return_value = clusters
        cluster = {
            "metadata": {"name": "orders-primary", "namespace": "platform"},
            "spec": {"bootstrap": {"recovery": {"backup": {"name": "orders-backup"}}}},
        }

        KubernetesClient(dynamic_client).create_restored_cluster(cluster)

        clusters.create.assert_called_once_with(body=cluster, namespace="platform")
        clusters.get.assert_called_once_with(
            name="orders-primary", namespace="platform"
        )

    def test_cleanup_removes_only_recovery_bootstrap(self) -> None:
        clusters = Mock()
        clusters.get.return_value.to_dict.return_value = {
            "spec": {
                "bootstrap": {
                    "initdb": {"database": "orders"},
                    "recovery": {"backup": {"name": "orders-backup"}},
                }
            }
        }
        dynamic_client = Mock()
        dynamic_client.resources.get.return_value = clusters

        KubernetesClient(dynamic_client).remove_recovery("platform", "orders-primary")

        clusters.patch.assert_called_once_with(
            name="orders-primary",
            namespace="platform",
            body=[{"op": "remove", "path": "/spec/bootstrap/recovery"}],
            content_type="application/json-patch+json",
        )


if __name__ == "__main__":
    unittest.main()
