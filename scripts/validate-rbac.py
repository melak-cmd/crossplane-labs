#!/usr/bin/env python3
"""Validate Crossplane RBAC for native Kubernetes resources in Compositions."""

import argparse
import os
import re
import sys

import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
COMPOSITION_DIR = os.path.join(ROOT, "apis")
RBAC_PATH = os.path.join(ROOT, "operations", "rbac.yaml")
COMPOSED_RESOURCE_RE = re.compile(
    r"(?m)^\s*apiVersion:\s*([A-Za-z0-9.-]+(?:/[A-Za-z0-9.]+)?)\s*\n"
    r"\s*kind:\s*([A-Za-z][A-Za-z0-9]*)\s*$"
)
KIND_TO_RESOURCE = {
    "Deployment": ("apps", "deployments"),
    "HorizontalPodAutoscaler": ("autoscaling", "horizontalpodautoscalers"),
    "NetworkPolicy": ("networking.k8s.io", "networkpolicies"),
    "Ingress": ("networking.k8s.io", "ingresses"),
    "Service": ("", "services"),
    "Secret": ("", "secrets"),
    "Cluster": ("postgresql.cnpg.io", "clusters"),
    "ScheduledBackup": ("postgresql.cnpg.io", "scheduledbackups"),
    "Backup": ("postgresql.cnpg.io", "backups"),
}
REQUIRED_VERBS = {"get", "list", "watch", "create", "update", "patch", "delete"}
AGGREGATION_LABEL = "rbac.crossplane.io/aggregate-to-crossplane"


def composition_templates():
    templates = []
    for directory, _, filenames in os.walk(COMPOSITION_DIR):
        for filename in sorted(filenames):
            if not filename.endswith((".yaml", ".yml")) or "composition" not in filename:
                continue
            path = os.path.join(directory, filename)
            with open(path, encoding="utf-8") as stream:
                for doc in yaml.safe_load_all(stream):
                    if not doc or doc.get("kind") != "Composition":
                        continue
                    name = doc.get("metadata", {}).get("name", filename)
                    for step in doc.get("spec", {}).get("pipeline", []):
                        function = step.get("functionRef", {}).get("name")
                        if function != "function-go-templating":
                            continue
                        template = step.get("input", {}).get("inline", {}).get("template", "")
                        templates.append((name, template))
    return templates


def composed_resources():
    resources = set()
    for composition, template in composition_templates():
        for api_version, kind in COMPOSED_RESOURCE_RE.findall(template):
            if kind in ("PostgreSQL", "DatabaseBackup"):
                continue
            mapping = KIND_TO_RESOURCE.get(kind)
            if mapping is None:
                raise ValueError("unknown composed Kind %r in %s" % (kind, composition))
            resources.add(mapping)
            if kind == "Cluster":
                resources.add((mapping[0], mapping[1] + "/status"))
    return resources


def aggregated_rules():
    with open(RBAC_PATH, encoding="utf-8") as stream:
        docs = yaml.safe_load_all(stream)
        return [
            rule
            for doc in docs
            if doc and doc.get("kind") == "ClusterRole"
            and doc.get("metadata", {}).get("labels", {}).get(AGGREGATION_LABEL) == "true"
            for rule in doc.get("rules", [])
        ]


def rule_covers(rule, group, resource, verb):
    return (
        ("*" in rule.get("apiGroups", []) or group in rule.get("apiGroups", []))
        and ("*" in rule.get("resources", []) or resource in rule.get("resources", []))
        and ("*" in rule.get("verbs", []) or verb in rule.get("verbs", []))
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--render-doc-table", action="store_true")
    args = parser.parse_args()

    try:
        resources = sorted(composed_resources())
        rules = aggregated_rules()
    except (OSError, yaml.YAMLError, ValueError) as error:
        print("validate-rbac: %s" % error, file=sys.stderr)
        return 1

    if args.render_doc_table:
        print("| apiGroup | Resource |")
        print("| --- | --- |")
        for group, resource in resources:
            print("| %s | %s |" % (group or "(core)", resource))
        return 0

    missing = [
        (group, resource, verb)
        for group, resource in resources
        for verb in sorted(REQUIRED_VERBS)
        if not any(rule_covers(rule, group, resource, verb) for rule in rules)
    ]
    if missing:
        print("Missing Crossplane aggregated RBAC grants:", file=sys.stderr)
        for group, resource, verb in missing:
            print("  %s/%s %s" % (group or "core", resource, verb), file=sys.stderr)
        return 1

    print("OK - Crossplane aggregated RBAC covers %d native resource types" % len(resources))
    return 0


if __name__ == "__main__":
    sys.exit(main())