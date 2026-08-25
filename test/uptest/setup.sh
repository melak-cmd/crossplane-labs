#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Installing XRDs..."
kubectl apply -R -f "${ROOT_DIR}/apis/"

echo "Installing Functions..."
kubectl apply -f "${ROOT_DIR}/crossplane/functions/"

echo "Waiting for all crossplane-system pods to be ready..."
kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

echo "Waiting for providers to be healthy..."
for i in $(seq 1 60); do
  if kubectl get providers -o jsonpath='{range .items[*]}{.status.conditions[?(@.type=="Healthy")].status}{"\n"}{end}' 2>/dev/null | grep -q "True"; then
    echo "Providers are healthy."
    break
  fi
  echo "Waiting for providers... ($i)"
  sleep 3
done

echo "Waiting for provider-kubernetes CRDs..."
kubectl wait --for=condition=Established crd/providerconfigs.kubernetes.m.crossplane.io --timeout=60s

echo "Installing CloudNativePG..."
helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
helm repo update
helm upgrade --install cnpg cnpg/cloudnative-pg \
    --namespace cnpg-system --create-namespace --wait

echo "Waiting for CNPG CRDs..."
kubectl wait --for=condition=Established crd/clusters.postgresql.cnpg.io --timeout=60s

echo "Creating platform namespace..."
kubectl create namespace platform --dry-run=client -o yaml | kubectl apply -f -

echo "Creating default ProviderConfig in platform..."
kubectl apply -f "${ROOT_DIR}/crossplane/providerconfigs/"

echo "Setup complete."
