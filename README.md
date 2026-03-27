# Accesserator

Accesserator is a Kubernetes operator that introduces the `SecurityConfig` CRD and uses it to configure security capabilities and make them available for [Skiperator](https://github.com/kartverket/skiperator) applications.

## What is in this fork?
Read more [here](FORK.md)

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

## Testing

To run all unit tests:

```bash
make test
```

# Autorisasjon med OPA (open policy agent)

Ved å bruke OPA vil applikasjonen din få en sidecontainer som inneholder en OPA policy-motor. Denne kan kalles på når applikasjonen trenger en autorisasjonsbeslutning. Løsningen gir sentralisert kontroll over autorisasjon og gjør det enklere å auditere, endre og rulle ut nye regler uten å måtte redeploye applikasjonene.

### Autorisasjonsregler

Regler i OPA defineres i språket `Rego`. Å skrive Rego-regler handler i praksis om policy as code, altså å beskrive tilgangsregler som kode. Reglene evaluerer typisk JSON-input mot gitte betingelser og returnerer et svar, for eksempel om en forespørsel skal tillates eller ikke.

En typisk Rego-fil (.rego) består som regel av en `package`, eventuelle imports og selve reglene:

```Rego
package app.authz

# Standardverdi hvis ingen andre regler slår til
default allow := false

# Regelen "allow" er sann hvis betingelsen i {} er oppfylt
allow {
    input.user.role == "admin"
}
```

Et typisk OPA-input er et JSON-objekt som beskriver hvem som gjør hva på hvilken ressurs, og eventuelt litt kontekst. Dette sendes som body i et POST-kall til http://localhost:8181/v1/data/app/authz/allow, pakket inn i et input-felt.

```JSON
{
  "input": {
    "user": { "id": "123", "role": "admin", "name": "Ola Nordmann" },
    "action": "read",
    "resource": { "type": "document", "id": "document-123" },
    "context": { "time": "2026-02-05T12:00:00Z" }
  }
}

```

**Testing av Rego**

Rego-regler kan testes med innebygde tester i egne filer (\_test.rego). Kjør tester med kommandoen: `opa test .`.

```Rego
package app.authz

test_allow_admin {
    allow with input as {"user": {"role": "admin"}}
}

test_deny_user {
    not allow with input as {"user": {"role": "user"}}
}
```

### Miljøvariabler og secrets

OPA-serveren må konfigureres med en public key til den signerte bundlen for å kunne verifisere avsender. Den må også ha en GitHub-token med nødvendige rettigheter til OCI/GHCR-bundlen som podden skal hente.

I den nåværende Accesserator-oppsettet injiseres disse inn i OPA-sidecaren som miljøvariabler:

- `GITHUB_TOKEN` (fra `spec.opa.githubToken`)
- `OPA_PUBLIC_KEY` (fra `spec.opa.bundlePublicKey`)

### Eksempel på konfigurasjon

Applikasjonsmanifest

```yaml
## Applikasjonsmanifestet
apiVersion: skiperator.kartverket.no/v1alpha1
kind: Application
metadata:
  name: another-app
  namespace: test
  labels:
    skiperator/security: "enabled"
spec:
  image: nginxinc/nginx-unprivileged:latest
  port: 8080
  replicas: 1
  accessPolicy:
    inbound:
      rules:
        - application: app
    outbound:
      external:
        # Tillat ekstern kommunikasjon med ghcr
        - host: ghcr.io
          ports:
            - name: https
              protocol: HTTPS
              port: 443
        - host: pkg-containers.githubusercontent.com
          ports:
            - name: https
              protocol: HTTPS
              port: 443
        - host: objects.githubusercontent.com
          ports:
            - name: https
              protocol: HTTPS
              port: 443
---
```

SecurityConfig-manifest

```yaml
apiVersion: accesserator.kartverket.no/v1alpha
kind: SecurityConfig
metadata:
  name: security-config-app
  namespace: test
spec:
  opa:
    enabled: true
    githubToken:
      name: "opa-github-secret"
      key: "github_token"
    bundlePublicKey:
      name: "opa-sign-config"
      key: "public_sign_key"
    bundlePath: "ghcr.io/kartverket/opa-bundle"
    bundleVersion: "latest"
  applicationRef: app
```