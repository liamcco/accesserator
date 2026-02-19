# Contributing guides
## Getting started

To run Accesserator locally, you need to have go >= 1.25.7 installed on your machine. 
You also need some other tools to set up and interact with the local Kubernetes cluster. All these 
dependencies are bundled together with [Flox environment](https://flox.dev/), which is a tool that provides a consistent development environment across different machines. 
You can install Flox by following the instructions on their [website](https://flox.dev/docs/install-flox/install).

To activate the Flox environment for this project, run the following command in the terminal:
```bash
flox activate
```
This will set up a local Kubernetes cluster (with [kind](https://kind.sigs.k8s.io/)) as well as give access to useful tools like `kubectl`, `kubectx`, `k9s`, `cloud-provider-kind` and `kubefwd`. 
The local Kubernetes cluster will have the following components installed and configured:
- Istio as service mesh
- Cert-manager to manage TLS certificates for webhook communication
- Skiperator to manage the lifecycle of the workloads deployed in the cluster
- Tokendings to act as an internal token exchange server
- Jwker to manage OAuth2 client registrations for applications that use tokendings
- Ztoperator to handle OAuth2 authorization code flow and JWT verification and authorization for workloads in the cluster
- Mock-OAuth2-Server to mock an external identity provider for testing purposes.

## Branch Workflow Conventions (OPA Fork)

Follow these rules when contributing to the OPA fork:

1. Start all work from `integrate-opa`.
2. Rebase your branch onto `integrate-opa` before merging.
3. Do not commit directly to `main`.

## Run accesserator locally

### Run on your host machine

To run accesserator with the local cluster, you can press `Run` on the run configuration called `Run accesserator` (if you have the project open in a JetBrains IDE), 
or run the following command in the terminal (where you previously activated the [Flox environment](#getting-started)):
```bash
make run-local
```

### Run in the local cluster

To run accesserator in the local cluster, you need to build and deploy a local image of accesserator to the cluster. You can do this by running the following command in the terminal (where you previously activated the [Flox environment](#getting-started)):
```bash
make deploy
```

You can then verify that everything is working correctly by applying the example `SecurityConfig` + Skiperator `Application` from the [examples](examples) folder.

```bash
kubectl apply -f examples/example.yaml
```

## Running tests

We use [envtest](https://book.kubebuilder.io/reference/envtest) and [Ginko](https://onsi.github.io/ginkgo/) for our unit and integration tests, as well as [chainsaw](https://kyverno.github.io/chainsaw/0.2.3/) for end-to-end testing.

To run the Ginko tests locally, run the following command in the terminal:
```bash
make test
```

When running the end-to-end tests locally, you can either run them with accesserator running on your host machine or with an accesserator running in the local cluster. 
To run the end-to-end tests with accesserator running on your host machine, you can either run all tests with
```bash
make chainsaw-test-host
```
or run a single test with
```bash
make chainsaw-test-host-single dir=<TEST FOLDER>
```

To run the end-to-end tests with accesserator running in the local cluster, you first have to deploy accesserator with the command
```bash
make deploy
```

You can then run the end-to-end tests with
```bash
make chainsaw-test-remote
```
