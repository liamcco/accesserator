# What This Fork Adds

This fork extends Accesserator with support for OPA-based authorization in addition to the existing Texas/TokenX integration.

In practice, that means:

- `SecurityConfig` can be used to enable an OPA sidecar for a Skiperator application.
- The sidecar runs an OPA policy engine that the application can call for authorization decisions.
- The setup is designed to support centrally managed policy distribution, updates, and auditing without redeploying the application itself.

# Good To Know
Two feature branches in this fork are especially relevant, as they implement different approaches to loading bundles into the OPA sidecar:

- `use-configmap`: solves the problem by letting Accesserator fetch the OPA bundle ahead of time, store it in a Kubernetes `ConfigMap`, and mount that bundle directly into the application pod for the OPA sidecar to use.
- `discovery-bundle-server`: solves the problem by serving a small discovery bundle from inside the cluster, which tells the OPA sidecar where to fetch the real signed policy bundle directly from OCI/GHCR.
