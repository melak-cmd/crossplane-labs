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
${KUBECTL} apply -f "${ROOT_DIR}/functions/"

echo "==> Waiting for all crossplane-system pods to be ready..."
${KUBECTL} wait --for=condition=Ready pods --all \
  -n "${CROSSPLANE_NAMESPACE}" --timeout=180s

echo "==> Waiting for composition revisions to be synced..."
until [ "$(${KUBECTL} get compositionrevisions -o json 2>/dev/null \
  | jq '[.items[] | select(.status.conditions[]? | select(.type=="Synced" and .status=="True"))] | length')" \
  -gt 0 ] 2>/dev/null; do
  echo "  Waiting for composition revisions..."
  sleep "$RETRY_DELAY"
done
echo "  Composition revisions are synced."

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

echo "==> Setup complete."
