#!/bin/bash

KUBECONTEXT=${KUBECONTEXT:-"kind-accesserator"}
ZTOPERATOR_VERSION=${ZTOPERATOR_VERSION:-"latest"}
KUBECTL_BIN="${KUBECTL_BIN:-./bin/kubectl}"

ZTOPERATOR_RESOURCES=(
  https://raw.githubusercontent.com/kartverket/ztoperator/refs/heads/main/config/crd/bases/ztoperator.kartverket.no_authpolicies.yaml
  https://raw.githubusercontent.com/kartverket/ztoperator/refs/heads/main/config/rbac/role.yaml
)

echo "🤞  Creating namespace: $namespace_name"

# Attempt to create the namespace and capture both stdout and stderr
output=$("${KUBECTL_BIN}" create namespace "ztoperator-system" 2>&1)
exit_code=$?

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'ztoperator-system' created successfully"
elif echo "$output" | grep -q "already exists"; then
    echo "✅  Namespace 'ztoperator-system' already exists, continuing..."
else
    echo -e "❌  Error creating 'ztoperator-system' namespace."
    exit 1
fi

# Install required ztoperator resources
for resource in "${ZTOPERATOR_RESOURCES[@]}"; do
  "${KUBECTL_BIN}" apply --context "$KUBECONTEXT" -f "$resource"
done

# Install ztoperator
ZTOPERATOR_MANIFESTS="$(cat <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: "ztoperator-system"
  name: "ztoperator"
automountServiceAccountToken: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ztoperator
roleRef:
  apiGroup: "rbac.authorization.k8s.io"
  kind: "ClusterRole"
  name: "ztoperator"
subjects:
  - kind: "ServiceAccount"
    namespace: "ztoperator-system"
    name: "ztoperator"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: ztoperator
  name: ztoperator
  namespace: ztoperator-system
spec:
  replicas: 2
  revisionHistoryLimit: 0
  selector:
    matchLabels:
      app: ztoperator
  template:
    metadata:
      labels:
        app: ztoperator
    spec:
      automountServiceAccountToken: true
      containers:
        - args:
            - -d
            - -leader-elect
            - -metrics-bind-address=0.0.0.0:8181
            - -metrics-secure=false
          image: "ghcr.io/kartverket/ztoperator:${ZTOPERATOR_VERSION}"
          livenessProbe:
            httpGet:
              path: /healthz
              port: probes
          name: ztoperator
          ports:
            - containerPort: 8181
              name: metrics
            - containerPort: 8081
              name: probes
          readinessProbe:
            httpGet:
              path: /readyz
              port: probes
          resources:
            limits:
              memory: 256Mi
            requests:
              cpu: 10m
              memory: 32Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              add:
                - NET_BIND_SERVICE
              drop:
                - ALL
            privileged: false
            readOnlyRootFilesystem: true
            runAsGroup: 65532
            runAsNonRoot: true
            runAsUser: 65532
            seccompProfile:
              type: RuntimeDefault
      imagePullSecrets:
        - name: github-auth
      priorityClassName: skip-critical
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: ztoperator
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app.kubernetes.io/name: ztoperator
  name: leader-election-role
  namespace: ztoperator-system
rules:
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
      - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app.kubernetes.io/name: ztoperator
  name: ztoperator-leader-election
  namespace: ztoperator-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: leader-election-role
subjects:
  - kind: ServiceAccount
    name: ztoperator
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ztoperator-netpol
  namespace: ztoperator-system
spec:
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: obo
          podSelector:
            matchLabels:
              app: tokendings
  podSelector:
    matchLabels:
      app: ztoperator
  policyTypes:
    - Egress
---
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: ztoperator-egress
  namespace: ztoperator-system
spec:
  exportTo:
    - .
    - istio-system
    - istio-gateways
  hosts:
    - login.microsoftonline.com
    - idporten.no
    - test.idporten.no
    - maskinporten.no
    - test.maskinporten.no
    - test.ansattporten.no
    - ansattporten.no
  ports:
    - name: https
      number: 443
      protocol: HTTPS
  resolution: DNS
EOF
)"

"${KUBECTL_BIN}" apply -f <(echo "$ZTOPERATOR_MANIFESTS") --context "$KUBECONTEXT"
