#!/usr/bin/env bash
set -euo pipefail

# Allow overriding via env, but default to your Makefile defaults
KUBECONTEXT="${KUBECONTEXT:-kind-accesserator}}"
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

NAMESPACE="accesserator-system"
DEPLOYMENT="accesserator"

MUTATING_CFG="accesserator-mutating-webhook-configuration"
VALIDATING_CFG="accesserator-validating-webhook-configuration"

EXPECTED_SVC="webhook-service"
EXPECTED_SELECTOR_KEY="accesserator-webhooks"
EXPECTED_SELECTOR_VALUE="enabled"

check_deployment() {
  echo "🔎 Checking Deployment ${NAMESPACE}/${DEPLOYMENT}..."

  ${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" >/dev/null

  READY=$(${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.status.readyReplicas}')

  DESIRED=$(${KUBECTL_BIN} get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.spec.replicas}')

  if [[ "${READY:-0}" != "$DESIRED" ]]; then
    echo "❌ Deployment not ready (ready=${READY:-0}, desired=${DESIRED})"
    exit 1
  fi

  echo "✅ Deployment is healthy (replicas=${DESIRED})."
}

check_webhook_config() {
  local KIND="$1"   # mutatingwebhookconfiguration | validatingwebhookconfiguration
  local NAME="$2"
  local LABEL="$3"  # Mutating | Validating

  echo "🔎 Checking ${LABEL}WebhookConfiguration ${NAME}..."

  ${KUBECTL_BIN} get "$KIND" "$NAME" >/dev/null

  # Ensure at least one webhook exists
  WEBHOOK_NAME=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].name}')

  if [[ -z "$WEBHOOK_NAME" ]]; then
    echo "❌ No webhooks defined in ${NAME}"
    exit 1
  fi

  # clientConfig.service
  SVC_NAME=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].clientConfig.service.name}')

  SVC_NS=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].clientConfig.service.namespace}')

  if [[ "$SVC_NAME" != "$EXPECTED_SVC" || "$SVC_NS" != "$NAMESPACE" ]]; then
    echo "❌ ${NAME} webhooks[0]: clientConfig.service must be ${NAMESPACE}/${EXPECTED_SVC}"
    echo "   got ${SVC_NS}/${SVC_NAME}"
    exit 1
  fi

  # CA bundle
  CA_BUNDLE=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].clientConfig.caBundle}')

  if [[ -z "$CA_BUNDLE" ]]; then
    echo "❌ ${NAME} webhooks[0]: clientConfig.caBundle is empty"
    exit 1
  fi

  # namespaceSelector (order-independent, portable)
  SELECTOR_KEY=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].namespaceSelector.matchExpressions[0].key}')

  SELECTOR_VALUE=$(${KUBECTL_BIN} get "$KIND" "$NAME" \
    -o jsonpath='{.webhooks[0].namespaceSelector.matchExpressions[0].values[0]}')

  if [[ "$SELECTOR_KEY" != "$EXPECTED_SELECTOR_KEY" || "$SELECTOR_VALUE" != "$EXPECTED_SELECTOR_VALUE" ]]; then
    echo "❌ ${NAME} webhooks[0]: namespaceSelector must include ${EXPECTED_SELECTOR_KEY}=In(${EXPECTED_SELECTOR_VALUE})"
    echo "   Found: ${SELECTOR_KEY}=In(${SELECTOR_VALUE:-<empty>})"
    exit 1
  fi

  echo "✅ ${LABEL}WebhookConfiguration ${NAME} is valid."
}

# ---- execution ----

check_deployment

check_webhook_config \
  "mutatingwebhookconfiguration" \
  "$MUTATING_CFG" \
  "Mutating"

check_webhook_config \
  "validatingwebhookconfiguration" \
  "$VALIDATING_CFG" \
  "Validating"

echo "🎉 Accesserator is fully deployed and all webhooks are healthy."