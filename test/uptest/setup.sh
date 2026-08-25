#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Installing XRDs..."
kubectl apply -f "${ROOT_DIR}/crossplane/xrds/"

echo "Installing Compositions..."
kubectl apply -f "${ROOT_DIR}/crossplane/compositions/"

echo "Installing Functions..."
kubectl apply -f "${ROOT_DIR}/crossplane/functions/"

echo "Waiting for all crossplane-system pods to be ready..."
kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

echo "Waiting for providers to be healthy..."
kubectl wait --for=condition=Healthy provider --all --timeout=180s

echo "Waiting for provider-kubernetes CRDs..."
kubectl wait --for=condition=Established crd/providerconfigs.kubernetes.m.crossplane.io --timeout=60s

echo "Creating a-team namespace..."
kubectl create namespace a-team --dry-run=client -o yaml | kubectl apply -f -

echo "Creating default ProviderConfig in a-team..."
cat <<EOF | kubectl apply -f -
apiVersion: kubernetes.m.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
  namespace: a-team
spec:
  credentials:
    source: InjectedIdentity
EOF

echo "Setup complete."
