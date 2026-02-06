# Accesserator

Konfigurerer en SKIP application med tilgangsstyrings policies som applikasjoner kan bruke.

## Autorisasjon med OPA (open policy agent)

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

OPA-serveren må konfigureres med en public key til den signerte bundlen for å kunne verifisere avsender. Den må også ha en public access key fra en GitHub-bruker med nødvendige rettigheter til repoet sidecaren er koblet mot.

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
    githubCredentials:
      clientTokenKey: "Referanse til GitHub token"
      clientTokenRef: "Ønsket navn til GitHub token"

  applicationRef: app
```
