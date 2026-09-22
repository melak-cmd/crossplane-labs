#!/usr/bin/env python3
"""Validate the composition read contract for the Crossplane lab.

Derives the downstream resource inventory from the Composition definitions
(kaonix-platform is the single source of truth), then asserts that the
provider identities the Compositions rely on are granted the verbs they need
to observe the resources they manage.

Discovered inventory sources (add here when new Compositions appear):
  - functions/function-app/*.gotmpl   (project function pipeline templates)
  - apis/**/composition.yaml           (inline crossplane-contrib-function-go-templating
                                       templates, and pipeline function refs)

Usage:
  python3 scripts/validate-rbac.py [--render-doc-table] [--strict]
"""

import argparse
import os
import re
import sys
from typing import Dict, List, NoReturn

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOC_PATH = os.path.join(ROOT, "docs", "compositions.md")
COMPOSITION_DIRS = [
    os.path.join(ROOT, "apis", "apps"),
    os.path.join(ROOT, "apis", "networks"),
    os.path.join(ROOT, "apis", "databases"),
]
FUNCTION_TEMPLATE_DIR = os.path.join(ROOT, "functions", "function-app")
PROVIDERS_DIR = os.path.join(ROOT, "providers")
PROVIDERCONFIGS_DIR = os.path.join(ROOT, "providers", "providerconfigs")

# Kind -> (apiGroup, resource) for every downstream resource the Compositions
# manage. Kept explicit (not pluralized) on purpose: unknown Kinds fail loudly
# so this table is extended deliberately, never silently.
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

# ProviderConfig name -> provider package that owns it. Composed Objects refer
# to ProviderConfigs by name; this maps each to the identity that must hold the
# grants the guard checks.
PROVIDERCONFIG_TO_PROVIDER = {"default": "provider-kubernetes"}

IF_DEPTH_RE = re.compile(r"{{\s*(if|range|with)\b")
END_DEPTH_RE = re.compile(r"{{\s*end\s*}}")
OBJECT_START_RE = re.compile(r"^\s*apiVersion:\s*kubernetes\.m\.crossplane\.io/v1alpha1\s*$")
OBJECT_KIND_RE = re.compile(r"^\s*kind:\s*Object\s*$")


def parse_error(msg) -> NoReturn:
    sys.stderr.write("validate-rbac: %s\n" % msg)
    sys.exit(1)


def yaml_available():
    try:
        import yaml  # noqa: F401
        return True
    except ImportError:
        return False


def load_yaml_docs(path):
    import yaml
    with open(path, "r") as f:
        return list(yaml.safe_load_all(f))


def indent_of(line):
    return len(line) - len(line.lstrip())


def composite_type(doc):
    ref = (doc.get("spec") or {}).get("compositeTypeRef") or {}
    if not ref:
        return "?"
    return "%s/%s" % (ref.get("apiVersion", "?"), ref.get("kind", "?"))


def inline_go_template(doc):
    for step in (doc.get("spec") or {}).get("pipeline") or []:
        if (step.get("functionRef") or {}).get("name") == "crossplane-contrib-function-go-templating":
            inline = (step.get("input") or {}).get("inline") or {}
            return inline.get("template")
    return None


def pipeline_function_names(doc):
    return [(step.get("step"), (step.get("functionRef") or {}).get("name"))
            for step in (doc.get("spec") or {}).get("pipeline") or []]


def extract_object(lines, composition, conditional, provider_config):
    manifest_idx = None
    for i, raw in enumerate(lines):
        if raw.strip() == "manifest:":
            manifest_idx = i
            break
    if manifest_idx is None:
        parse_error("could not find manifest block in %s object" % composition)
    base_indent = indent_of(lines[manifest_idx])
    api = kind = None
    for raw in lines[manifest_idx + 1:]:
        stripped = raw.strip()
        if not stripped:
            continue
        if indent_of(raw) <= base_indent:
            break
        if api is None and stripped.startswith("apiVersion:"):
            api = stripped.split(":", 1)[1].strip()
        elif api is not None and stripped.startswith("kind:"):
            kind = stripped.split(":", 1)[1].strip()
            break
    if api is None or kind is None:
        parse_error("could not extract manifest apiVersion/kind from %s object" % composition)
    pc_name = provider_config
    if pc_name is None:
        for i, raw in enumerate(lines):
            if raw.strip() == "providerConfigRef:":
                for j in range(i + 1, len(lines)):
                    sub = lines[j].strip()
                    if sub.startswith("name:"):
                        pc_name = sub.split(":", 1)[1].strip()
                        break
                break
    return {
        "composition": composition,
        "apiVersion": api,
        "kind": kind,
        "conditional": conditional,
        "providerConfig": pc_name or "default",
    }


def scan_template(text, composition, provider_config=None):
    """Extract kubernetes.m.crossplane.io Object manifests from a template body.

    Tracks Go-template conditional depth across the whole body so Objects
    wrapped in `{{ if ... }}` blocks (e.g. DNS Service, Ingress, ScheduledBackup)
    are marked conditional.
    """
    objects = []
    block = []
    depth = 0
    in_object = False
    pending_kind = False
    block_open_depth = 0
    for raw in text.splitlines():
        depth += len(IF_DEPTH_RE.findall(raw))
        depth -= len(END_DEPTH_RE.findall(raw))
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if stripped == "---":
            if in_object and block:
                objects.append(extract_object(block, composition, block_open_depth > 0, provider_config))
            block = []
            in_object = False
            pending_kind = False
            block_open_depth = depth
            continue
        if in_object:
            block.append(raw)
            continue
        if pending_kind:
            if OBJECT_KIND_RE.match(raw):
                in_object = True
                block.append(raw)
            pending_kind = False
            continue
        if OBJECT_START_RE.match(raw):
            pending_kind = True
            block.append(raw)
    if in_object and block:
        objects.append(extract_object(block, composition, block_open_depth > 0, provider_config))
    return objects


def discover_inventory():
    inventory = []
    fn_comps = {}
    for directory in COMPOSITION_DIRS:
        if not yaml_available():
            parse_error("PyYAML is required (pip install pyyaml) to parse %s" % directory)
        for fname in sorted(os.listdir(directory)):
            if not (fname.endswith(".yaml") and "composition" in fname):
                continue
            path = os.path.join(directory, fname)
            docs = load_yaml_docs(path)
            doc = next((d for d in docs if d and d.get("kind") == "Composition"), None)
            if doc is None:
                continue
            comp_name = (doc.get("metadata") or {}).get("name", fname)
            template = inline_go_template(doc)
            if template is not None:
                for obj in scan_template(template, comp_name):
                    obj["xr"] = composite_type(doc)
                    obj["source"] = os.path.relpath(path, ROOT)
                    inventory.append(obj)
            for step, fn in pipeline_function_names(doc):
                if fn and "function-app" in fn:
                    fn_comps.setdefault(comp_name, []).append(step)
    if not yaml_available():
        parse_error("PyYAML is required (pip install pyyaml) to inspect function templates")
    if os.path.isdir(FUNCTION_TEMPLATE_DIR):
        for fname in sorted(os.listdir(FUNCTION_TEMPLATE_DIR)):
            if not (fname.endswith(".gotmpl") and fname != "00-prelude.yaml.gotmpl"):
                continue
            path = os.path.join(FUNCTION_TEMPLATE_DIR, fname)
            with open(path, "r") as f:
                text = f.read()
            for comp_name, steps in fn_comps.items():
                for directory in COMPOSITION_DIRS:
                    for cfname in sorted(os.listdir(directory)):
                        if not (cfname.endswith(".yaml") and "composition" in cfname):
                            continue
                        cpath = os.path.join(directory, cfname)
                        comp = next((d for d in load_yaml_docs(cpath)
                                     if d and d.get("kind") == "Composition"), None)
                        if comp is None or (comp.get("metadata") or {}).get("name") != comp_name:
                            continue
                        for obj in scan_template(text, comp_name):
                            obj["xr"] = composite_type(comp)
                            obj["source"] = os.path.relpath(path, ROOT)
                            inventory.append(obj)
                        break
    return inventory


def classifiable(inventory):
    """Return (entry, group, resource) for every inventory entry that is mappable."""
    for obj in inventory:
        mapping = KIND_TO_RESOURCE.get(obj["kind"])
        if mapping is None:
            parse_error("unknown Kind %r in %s - add it to KIND_TO_RESOURCE" % (obj["kind"], obj["composition"]))
        group, resource = mapping
        yield obj, group, resource


def inventory_table(inventory):
    header = "| Composition | XR | Downstream | apiVersion/kind | Resource | Conditional | ProviderConfig |"
    sep = "| --- | --- | --- | --- | --- | --- | --- |"
    rows = []
    for obj, group, resource in sortable(classifiable(inventory)):
        rows.append("| %s | %s | %s | %s | %s | %s | %s |" % (
            obj["composition"], obj["xr"], obj["kind"], obj["apiVersion"],
            resource, "yes" if obj["conditional"] else "no", obj["providerConfig"]))
    return "\n".join([header, sep] + rows)


def sortable(items):
    return sorted(items, key=lambda item: (item[0]["composition"], item[0]["apiVersion"], item[0]["kind"]))


def load_provider_rules() -> Dict[str, List[dict]]:
    rules: Dict[str, List[dict]] = {}
    for fname in sorted(os.listdir(PROVIDERS_DIR)):
        if not fname.endswith(".yaml"):
            continue
        path = os.path.join(PROVIDERS_DIR, fname)
        if not os.path.isfile(path):
            continue
        if not yaml_available():
            parse_error("PyYAML is required (pip install pyyaml) to parse %s" % path)
        for doc in load_yaml_docs(path):
            if not doc or doc.get("kind") != "ClusterRole":
                continue
            name = (doc.get("metadata") or {}).get("name")
            if not name:
                continue
            for rule in doc.get("rules") or []:
                rules.setdefault(str(name), []).append(rule)
    return rules


def rule_covers(rule, group, resource, verb):
    groups = rule.get("apiGroups") or []
    resources = rule.get("resources") or []
    verbs = rule.get("verbs") or []
    return (("*" in groups or group in groups) and
            ("*" in resources or resource in resources) and
            ("*" in verbs or verb in verbs))


def check_grants(inventory, rules):
    failures = []
    for obj, group, resource in classifiable(inventory):
        provider = PROVIDERCONFIG_TO_PROVIDER.get(obj["providerConfig"])
        if provider is None:
            failures.append(("providerconfig", obj["providerConfig"], "?", "get"))
            failures.append(("providerconfig", obj["providerConfig"], "?", "list"))
            continue
        provider_rules = rules.get(provider, [])
        for verb in ("get", "list"):
            if not any(rule_covers(rule, group, resource, verb) for rule in provider_rules):
                failures.append((provider, group, resource, verb))
    referenced = sorted({obj["providerConfig"] for obj in inventory})
    missing_pcs = [pc for pc in referenced
                   if not os.path.exists(os.path.join(PROVIDERCONFIGS_DIR, "%s.yaml" % pc))]
    return failures, missing_pcs


def parse_doc_inventory():
    if not os.path.exists(DOC_PATH):
        return None
    entries = []
    in_table = False
    with open(DOC_PATH, "r") as f:
        for line in f:
            stripped = line.strip()
            if stripped.startswith("#"):
                if in_table:
                    break
                if stripped.startswith("### Downstream resource inventory"):
                    in_table = True
                continue
            if not in_table or not stripped.startswith("|"):
                continue
            cols = [c.strip() for c in stripped.strip("|").split("|")]
            if len(cols) != 7 or cols[0] == "---" or cols[3] == "apiVersion/kind":
                continue
            entries.append({
                "composition": cols[0],
                "apiVersion": cols[3],
                "kind": cols[2],
                "conditional": cols[5] == "yes",
                "providerConfig": cols[6],
            })
    return entries


def canonical(inventory):
    return sorted(
        (o["composition"], o["apiVersion"], o["kind"], o["conditional"], o["providerConfig"])
        for o in inventory)


def check_doc_drift(inventory, strict):
    doc = parse_doc_inventory()
    if not doc:
        sys.stderr.write("validate-rbac: warning: %s not found - doc-drift check skipped\n" % DOC_PATH)
        return False
    derived = canonical(inventory)
    documented = canonical(doc)
    if derived == documented:
        return False
    sys.stderr.write("validate-rbac: %s: inventory table differs from derived inventory\n" % DOC_PATH)
    d_set, doc_set = set(derived), set(documented)
    for entry in sorted(d_set - doc_set):
        sys.stderr.write("  missing from doc: %s\n" % (entry,))
    for entry in sorted(doc_set - d_set):
        sys.stderr.write("  not in code: %s\n" % (entry,))
    if strict:
        sys.stderr.write("validate-rbac: FAIL: doc table must match derived inventory in --strict\n")
        return True
    sys.stderr.write("validate-rbac: warning: re-run --render-doc-table and update docs/compositions.md (or use --strict)\n")
    return False


def main():
    parser = argparse.ArgumentParser(description="Validate the composition read contract.")
    parser.add_argument("--render-doc-table", action="store_true",
                        help="print the canonical inventory table for docs/compositions.md")
    parser.add_argument("--strict", action="store_true",
                        help="fail when docs/compositions.md drifts from the derived inventory")
    args = parser.parse_args()

    inventory = discover_inventory()
    if args.render_doc_table:
        print("# Downstream resource inventory (generated - do not hand-edit)\n")
        print(inventory_table(inventory))
        return 0

    rules = load_provider_rules()
    failures, missing_pcs = check_grants(inventory, rules)

    print("Composition read contract validation")
    print("=" * 60)
    for obj, group, resource in sortable(classifiable(inventory)):
        print("  %-26s %-16s %-28s %s/%s (%s)" % (
            obj["composition"], obj["kind"], obj["apiVersion"], group, resource,
            obj["providerConfig"]))

    status = 0
    if failures:
        status = 1
        print("\nMissing grants:")
        for provider, group, resource, verb in sorted(failures):
            print("  %-22s %s/%s %s" % (provider, group or "core", resource, verb))
    if missing_pcs:
        status = 1
        print("\nMissing ProviderConfig files under %s:" % PROVIDERCONFIGS_DIR)
        for pc in sorted(missing_pcs):
            print("  %s.yaml" % pc)

    if check_doc_drift(inventory, args.strict):
        status = 1

    print()
    print("FAIL" if status else
          "OK - grants cover every downstream resource (%d); doc matches derived inventory" % len(inventory))
    return status


if __name__ == "__main__":
    sys.exit(main())