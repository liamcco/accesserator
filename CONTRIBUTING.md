# Contributing guides

## Run accesserator locally
 
We use a `Makefile` to simplify all the steps involved in running accesserator locally.

To run accesserator locally, you need to set up a local Kubernetes cluster and install the necessary dependencies. 
This can be done with the command 
```bash
make local
```

To run accesserator with the local cluster, you can press `Run` if you have the project open in a JetBrains IDE, or run the following command in the terminal:
```bash
make run-local
```

You can then verify that accesserator is running by applying the example `SecurityConfig` + Skiperator `Application` from the [examples](examples) folder.

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