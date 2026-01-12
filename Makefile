# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

##@ Variables

KUBERNETES_VERSION			= 1.35.0
KIND_IMAGE					= kindest/node:v$(KUBERNETES_VERSION)
KIND_CLUSTER_NAME          ?= accesserator
KUBECONTEXT                ?= kind-$(KIND_CLUSTER_NAME)
ISTIO_VERSION 				= 1.28.0
CERT_MANAGER_VERSION		= 1.19.2

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: local
local: ensureflox cluster cert-manager istio skiperator tokendings jwker mock-oauth2 generate install ## Set up entire local development environment with external dependencies

.PHONY: clean
clean: ensureflox
	@kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= accesserator-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ensureflox ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: ensureflox setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ensureflox ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name accesserator-builder
	$(CONTAINER_TOOL) buildx use accesserator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm accesserator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: webhooks
webhooks: kustomize ## Install mutating and validating webhook into the K8s cluster
	@/bin/bash ./scripts/create-webhook-certs.sh
	$(KUBECTL) --context $(KUBECONTEXT) apply -f config/webhook/local-webhooks.yaml
	@CABUNDLE=$$(tr -d '\n' < webhook-certs/caBundle); \
	$(KUBECTL) --context $(KUBECONTEXT) patch mutatingwebhookconfiguration accesserator-mutating-webhook-configuration --type='json' -p="[{\"op\":\"replace\",\"path\":\"/webhooks/0/clientConfig/caBundle\",\"value\":\"$$CABUNDLE\"}]"; \
	$(KUBECTL) --context $(KUBECONTEXT) patch validatingwebhookconfiguration accesserator-validating-webhook-configuration --type='json' -p="[{\"op\":\"replace\",\"path\":\"/webhooks/0/clientConfig/caBundle\",\"value\":\"$$CABUNDLE\"}]"

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: ensureflox manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: ensureflox manifests kustomize docker-build ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	kind load docker-image ${IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: ensureflox kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Cluster

.PHONY: cluster
cluster: ensureflox ## Create Kind cluster with kube context kind-accesserator
	@echo Create kind cluster... >&2
	@kind create cluster --image $(KIND_IMAGE) --name ${KIND_CLUSTER_NAME}

##@ Operators

.PHONY: install-jwker-crds
install-jwker-crds: ensureflox ## Installing Jwker CRDs
	@echo -e "🤞  Installing jwker crds..."
	@kubectl apply -f https://raw.githubusercontent.com/nais/liberator/main/config/crd/bases/nais.io_jwkers.yaml --context $(KUBECONTEXT)

.PHONY: jwker
jwker: ensureflox install-jwker-crds ## Installing Jwker on k8s cluster
	@echo -e "🤞  Installing Jwker..."
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash scripts/install-jwker.sh
	@kubectl wait pod --for=create --timeout=60s -n obo -l app=jwker --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Jwker." && exit 1; }
	@kubectl wait pod --for=condition=Ready --timeout=60s -n obo -l app=jwker --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Jwker." && exit 1; }
	@echo -e "✅  Jwker installed in namespace 'obo'!"

.PHONY: skiperator
skiperator: ensureflox ## Install Skiperator on k8s cluster
	@echo -e "🤞  Installing Skiperator..."
	@kubectl create namespace skiperator-system || (echo -e "❌  Error creating 'skiperator-system' namespace." && exit 1)
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/install-skiperator.sh
	@kubectl wait pod --for=condition=ready --timeout=30s -n skiperator-system -l app=skiperator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying Skiperator." && exit 1)
	@echo -e "✅  Skiperator installed in namespace 'skiperator-system'!"

.PHONY: install-istio
install-istio: ensureflox ## Install istio
	@echo "⬇️ Downloading Istio..."
	@curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) TARGET_ARCH=$(ARCH) sh -
	@echo "⛵️  Installing Istio on Kubernetes cluster..."
	@./istio-$(ISTIO_VERSION)/bin/istioctl install --context $(KUBECONTEXT) -y --set meshConfig.accessLogFile=/dev/stdout --set profile=minimal &> /dev/null
	@echo "✅  Istio installation complete."

.PHONY: istio
istio: ensureflox helm install-istio ## Install istio gateways
	@echo "⛵️ Creating istio-gateways namespace..."
	@kubectl create namespace istio-gateways --context $(KUBECONTEXT) &> /dev/null || true
	@echo "⬇️  Installing istio-gateways"
	@helm install istio-ingressgateway istio/gateway --version v$(ISTIO_VERSION) -n istio-gateways --kube-context $(KUBECONTEXT) --set labels.app=istio-ingress-external --set labels.istio=ingressgateway
	@echo "✅  Istio gateways installed."

.PHONY: cert-manager
cert-manager: ensureflox
	@echo -e "🤞  Installing cert-manager..."
	@kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "🕑  Waiting for cert-manager to be ready..."
	@kubectl -n cert-manager wait deploy --all --for=condition=Available --timeout=60s
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/create-cluster-issuer.sh
	@echo -e "✅  Cert-manager installed!"

##@ Helper services

.PHONY: tokendings
tokendings: ensureflox ## Deploying tokendings oauth authorization server
	@echo -e "🤞  Setting up Tokendings..."
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash scripts/install-tokendings.sh
	@kubectl wait pod --for=create --timeout=60s -n obo -l app=tokendings --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Tokendings." && exit 1; }
	@kubectl wait pod --for=condition=Ready --timeout=60s -n obo -l app=tokendings --context $(KUBECONTEXT) &> /dev/null || { echo -e "❌  Error deploying Tokendings." && exit 1; }
	@echo -e "✅  Tokendings installed in namespace 'obo'!"

.PHONY: mock-oauth2
mock-oauth2: ensureflox ## Deployinh Mock-OAuth service in auth namespace
	@echo -e "🤞  Deploying 'mock-oauth2'..."
	@KUBECONTEXT=$(KUBECONTEXT) MOCK_OAUTH2_CONFIG=scripts/mock-oauth2-server-config.json /bin/bash ./scripts/install-mock-oauth2.sh
	@echo -e "✅  'mock-oauth2' is ready and running"

##@ Helpers

.PHONY: mock-token
mock-token: ensureflox ensurekubefwd ## Retrieves a JWT issued by mock-oauth2
	@command -v jq >/dev/null 2>&1 || { echo -e "❌  jq is required (used to parse JSON). Please install jq and try again."; exit 1; }
	@token=$$(curl -s -X POST "http://mock-oauth2.auth:8080/accesserator/token" \
		-d "grant_type=authorization_code" \
		-d "code=code" \
		-d "client_id=something" | jq -r '.access_token // empty'); \
	if [ -z "$$token" ]; then \
		echo -e "❌  No access_token found in response"; \
		exit 1; \
	fi; \
	echo "$$token"

.PHONY: ensurelocal
ensurelocal: kind kubectl
	@/bin/bash ./scripts/ensure-local-setup.sh

##@ Dependencies

.PHONY: helm
helm: ensureflox ## Fetch helm charts for Istio
	# Ensure istio helm repo exists
	@helm repo list | grep -q '^istio\s' || (echo "Adding istio helm repo..." && helm repo add istio https://istio-release.storage.googleapis.com/charts)
	# Make sure the requested ISTIO_VERSION is available; update index if not
	@helm search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "Updating Helm repos to fetch Istio charts..." && helm repo update)
	@helm search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "❌ Istio Helm chart version $(ISTIO_VERSION) not found in repo index." && echo "   Tip: check available versions with: helm search repo istio/gateway --versions" && exit 1)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0
KUBECTL_VERSION ?= v1.34.2
KIND_VERSION ?= v0.30.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.5.0
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary.
$(KUBECTL): $(LOCALBIN)
	@set -e; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		armv7l) arch=arm ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	url="https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$${os}/$${arch}/kubectl"; \
	echo "Downloading kubectl $(KUBECTL_VERSION) from $$url"; \
	curl -L -o "$(KUBECTL)" "$$url"; \
	chmod +x "$(KUBECTL)"; \
	echo "✅ kubectl installed at $(KUBECTL)"

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

### CUSTOM TARGETS ###
ensureflox:
	@if ! command -v "flox" >/dev/null 2>&1; then \
		echo -e "❌  Flox is not installed. Please install Flox (https://flox.dev/docs/install-flox/) and try again."; \
		exit 1; \
	fi
ifndef FLOX_ENV
	echo -e "❌  Flox is not activated. Please activate flox with 'flox activate' and try again." && exit 1
endif

ensurekubefwd:
	@pgrep -f "kubefwd( |$$)" >/dev/null 2>&1 || { \
		echo -e "❌  kubefwd is not running."; \
		echo -e "    Start it in another terminal with:"; \
		echo -e "      sudo kubefwd svc -n <namespace> --context $(KUBECONTEXT)"; \
		exit 1; \
	}