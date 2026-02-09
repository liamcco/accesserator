#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

echo "🤞  Creating namespace: obo"

# Attempt to create the namespace and capture both stdout and stderr
# NOTE: `set -e` would abort the script on a non-zero exit code here (e.g. AlreadyExists),
# so we temporarily disable it to handle the error explicitly.
set +e
output=$("${KUBECTL_BIN}" create namespace "obo" --context "$KUBECONTEXT" 2>&1)
exit_code=$?
set -e

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'obo' created successfully"
elif echo "$output" | grep -qiE "already exists|AlreadyExists"; then
    echo "✅  Namespace 'obo' already exists, continuing..."
else
    echo -e "❌  Error creating 'obo' namespace:"
    echo "$output"
    exit 1
fi

# Install Jwker
JWKER_MANIFESTS="$(cat <<EOF
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: jwker
  namespace: obo
spec:
  image: ghcr.io/nais/jwker:2025-01-07-145102-876d62d@sha256:8f6db2eff60db7c24c8d9df510e5f3aa3cee4df4b168ef8084094975f468b549
  port: 8080
  replicas: 1
  env:
    - name: JWKER_CLIENT_ID
      value: dfb2cec9-3b6d-456b-a14f-649236247e3d
    - name: TOKENDINGS_URL
      value: http://tokendings.obo:7456
    - name: CLUSTER_NAME
      value: kind-accesserator
    - name: JWKER_PRIVATE_JWK
      value: |
        {"d":"EeEtuQrs5k_kRasM-tOWuTe_mEjXtGJsjfTZId0v8lZ43r-LasZHq07OVERiWLv6grlUVKkxQ45dRh4yMK3YHGsCJKapBuRXKNfiwNYq9IrzHR04k8ADe9SfLS3Bu1_ig15SFEytzxYVn2Dswh6mDF1dtbL8z5xwLmOJhdL0UloBleYRvThkG_oQR8DzURUucS8newhTDE6xO5O5uAPmkDAEdkekWf93UQKipPv-QGA_dFf7Z5xB90qX8mW5qyAUcnaajPw6FufuP_VrGhfuTMPsJ0Aw1JBfxrazZFWPwGRuFFUaxbRN-OS7GLTpN9zd3DfX4gDsMs4vpJT7kkk10Q","dp":"OVbxZhOnW5rV_l5eKke1MfM7WuKRjiEjd0eL4Q8fQtFE9zNVhac3MimJQSUv4teTNJGFic7f9TTIpY25I9dQcDiqdp6Kob-7gqeOC8eAGNGtPi6ogO80WIri7XZ-mW4hBMp6UaXqC9KC7AoW0zbQMmkNWDkXg8KfnquCu29EQvE","dq":"ZgpZZ-mBBeMYHLBq4rmQyQA8aqJGbcMWWNGyzzlgsusYFYg9pKYWLwFVZPoQjzwAD4QTIGIUow3TC5kUkEwP7IiwjyuQwpexM3csSeK6Ag0MctmECehbIKfYN8TyOPM_TjxMJejOMs7BHwY2jnZ7Iakhx5yLvV2mfA2EhGz_cbU","e":"AQAB","kid":"1234","kty":"RSA","n":"vtHS1ulZpqxYPJZQmZrdgUPfMT4AR8sYL1VfLHwZc7bNo4fm1VRS0XwaRFYyOBBk1SXTCc7ojM8aO10NOHVYUWxMpr8mcxjWlFnn3xq0D_Z7DAKHt2cjIG2EPRzloH76qfqUCqmIHULTRuIavsOBmbE7dhTC63KKoO10i6KqP6iDeuqEds7-PyaqjB4F8-kgL9ukdVfo9AmWEnIm0bUvnNAhdyozoyGcmdI1bbtGWOo0RSb1t94LgVvWrpLzQulEfgHZpoW-TbrLlKhEA311BORSs55RhY8xDyijHNZQ-aShpQkROI7VELHyS6p54339g0z-FKp9Uwa8KQ2uijPp7Q","p":"6zv2JRlLYswnDmmMqYLFnYRW9CgBa-Aq9Cx41qfJi8NLLlVL_ZN1wg8WE9Bj3la6cAbUEIthzVc40G12Tk49HK-91lGmzy1zf6gQ2QD5vUSi4FEhNp_rGWtgE2GLn_1Y1268Gpr1DPkjHqPalh3krk7vd3ENbTZ9wNC7ikew75E","q":"z6oj5v91e1q6erKnvrjCj_Fqdai88heklEaW16xespfoibBrEmau6kUd4ojbKgCE4ZP8gX4ANzisW6TFoyNafqQ7HKSux87euW9EpyfdwmpGGTc7k9NMwtoIJzduqkaL6IebWBzXhpHn5Sguhri-qeJoQa6TSfdFKraK5Pm-Hp0","qi":"XjFAhzFySRwuXHhVtGTwt0UEnEwkfLHW606S4UD5bylSMSdroKAtb2KEnWuXf9tkixCW35JZAfPqBaAeqYn3Wz9hScZgV3qqJm_aJt1wN7Aih7mG__4dSOqWOp95MMZQs9HVfUqyTtz8Vv9O9PDzZf60CSHVQfVvPCFkHukLlcM"}
  accessPolicy:
    outbound:
      rules:
        - application: tokendings
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: jwker
  name: jwker
  namespace: obo
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: jwker
  name: jwker
  namespace: obo
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - get
      - create
      - update
      - delete
      - patch
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app: jwker
  name: jwker
  namespace: obo
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: jwker
subjects:
  - kind: ServiceAccount
    name: jwker
    namespace: obo
EOF
)"

"${KUBECTL_BIN}" apply -f <(echo "$JWKER_MANIFESTS") --context "$KUBECONTEXT"
