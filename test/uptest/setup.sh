#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

KUBECTL="${KUBECTL:-kubectl}"
CROSSPLANE_NAMESPACE="${CROSSPLANE_NAMESPACE:-crossplane-system}"
MAX_ATTEMPTS=60
RETRY_DELAY=3

retry() {
  local max_attempts=$1; shift
  local delay=$1; shift
  local attempt=1
  while [ "$attempt" -le "$max_attempts" ]; do
    if eval "$@"; then
      return 0
    fi
    echo "  Retry $attempt/$max_attempts in ${delay}s..."
    sleep "$delay"
    ((attempt++))
  done
  echo "  FAILED after $max_attempts attempts: $*" >&2
  return 1
}

echo "==> Installing XRDs and Compositions..."
${KUBECTL} apply -R -f "${ROOT_DIR}/apis/"

echo "==> Installing Functions..."
${KUBECTL} apply -f "${ROOT_DIR}/crossplane/functions/"

echo "==> Waiting for all crossplane-system pods to be ready..."
${KUBECTL} wait --for=condition=Ready pods --all \
  -n "${CROSSPLANE_NAMESPACE}" --timeout=180s

echo "==> Waiting for all providers to be healthy..."
until ${KUBECTL} get providers -o json \
  | jq -e '[.items[].status.conditions[] | select(.type=="Healthy" and .status=="True")] | length == .items | not' \
      >/dev/null 2>&1; do
  # jq expression above: true when number of Healthy=True equals total providers
  # If jq fails or condition is false, not all providers ready
  ready=$(${KUBECTL} get providers -o json \
    | jq -r '[.items[].status.conditions[] | select(.type=="Healthy" and .status=="True")] | length' 2>/dev/null || echo 0)
  total=$(${KUBECTL} get providers -o json \
    | jq -r '.items | length' 2>/dev/null || echo 0)
  if [ "$ready" -eq "$total" ] && [ "$total" -gt 0 ]; then
    echo "  All $total providers are healthy."
    break
  fi
  echo "  Waiting for providers... ($ready/$total healthy)"
  sleep "$RETRY_DELAY"
done

echo "==> Waiting for provider CRDs..."
${KUBECTL} wait --for=condition=Established \
  crd/providerconfigs.kubernetes.m.crossplane.io --timeout=60s

echo "==> Waiting for composition revisions to be synced..."
until [ "$(${KUBECTL} get compositionrevisions -o json 2>/dev/null \
  | jq '[.items[] | select(.status.conditions[]? | select(.type=="Synced" and .status=="True"))] | length')" \
  -gt 0 ] 2>/dev/null; do
  echo "  Waiting for composition revisions..."
  sleep "$RETRY_DELAY"
done
echo "  Composition revisions are synced."

echo "==> Checking provider webhook endpoints..."
if ! curl -sL https://raw.githubusercontent.com/crossplane/uptest/main/hack/check_endpoints.sh \
  -o /tmp/check_endpoints.sh 2>/dev/null; then
  echo "  WARNING: Could not download check_endpoints.sh, skipping webhook check."
else
  chmod +x /tmp/check_endpoints.sh
  /tmp/check_endpoints.sh
fi

echo "==> Installing CloudNativePG..."
helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
helm repo update
helm upgrade --install cnpg cnpg/cloudnative-pg \
  --namespace cnpg-system --create-namespace --wait

echo "==> Waiting for CNPG CRDs..."
${KUBECTL} wait --for=condition=Established \
  crd/clusters.postgresql.cnpg.io --timeout=60s

echo "==> Creating platform namespace..."
${KUBECTL} create namespace platform --dry-run=client -o yaml \
  | ${KUBECTL} apply -f -

echo "==> Creating default ProviderConfig in platform..."
${KUBECTL} apply -f "${ROOT_DIR}/crossplane/providerconfigs/"

echo "==> Setup complete."
