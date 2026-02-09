# Accesserator

Accesserator is a Kubernetes operator that introduces the `SecurityConfig` CRD and uses it to configure security capabilities and make them available for [Skiperator](https://github.com/kartverket/skiperator) applications.

## 🔍 What Accesserator does
A `SecurityConfig` defines which security capabilities should be created and made available for a Skiperator application referenced by `applicationRef`. 
Accesserator does this by injecting a sidecar container, called **texas** into the application pod that implements the desired security capabilities.
Texas is an API that can perform various token-related activities, and the documentation of the API can be viewed [here](https://editor.swagger.io/?url=https://raw.githubusercontent.com/nais/texas/refs/heads/master/doc/openapi-spec.json).
Texas is configurable through the `SecurityConfig` spec, which can be viewed [here](api-docs.md).
- `spec.tokenx.enabled` indicates whether the token exchange (TokenX) capability should be configured and made available for the application. If this is set to `true`,
the Skiperator application will be able to exchange tokens for the application referred to by `applicationRef` as the intended audience, **as long as the [access policies](https://skip.kartverket.no/docs/applikasjon-utrulling/skiperator/api-docs#applicationspecaccesspolicy) in the Skiperator `Application` manifest allow it**.

> [!IMPORTANT]
> In order for the Skiperator application to get a Texas sidecar container, the `Application` manifest must have the label `skiperator/security: "enabled"`.

## 🔧 Example
See `examples/example.yaml` for a complete example with a namespace, Skiperator `Application`, and corresponding `SecurityConfig`.

A minimal `SecurityConfig` example:

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
  namespace: test
spec:
  tokenx:
    enabled: true
  applicationRef: app
```

## 🧪 Local development

Refer to [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to run and test Accesserator locally.