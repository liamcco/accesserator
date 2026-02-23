# discoveryserver

This package contains the core logic used by the OPA discovery bundle fetcher sidecar.

It does not run an HTTP server itself. Instead, it provides the runtime logic for:

- pulling an OPA bundle from an OCI registry (GHCR in current setup)
- verifying the bundle signature (`.signatures.json` + file hashes)
- stripping signature metadata after verification
- writing the mirrored bundle atomically to a file served by `nginx`
- skipping unnecessary downloads using remote digest + local checksum checks

## How It Is Used

Runtime topology (current design):

- `nginx` container serves:
  - discovery config (`/discovery.json`)
  - mirrored bundle (`/bundles/authz.tar.gz`)
- `opa-discovery-fetcher` sidecar:
  - calls into this package on startup and on refresh ticks
  - fetches/verifies/rewrites the mirrored bundle file
- shared volume (`emptyDir`) stores the mirrored bundle file

The fetcher binary is in `cmd/opa-discovery-fetcher`, and it uses:

- `discoveryserver.FetchAndVerifyToFile(...)`
- `discoveryserver.FetchAndVerifyToFileIfChanged(...)`

## Package Structure

- `server.go`
  - orchestration entrypoints
  - config validation
  - "smart refresh" flow (digest check -> verify -> strip -> write)

- `oci.go`
  - OCI registry access (`remote.Get`, `remote.Image`)
  - GHCR auth options (PAT via basic auth)
  - bundle layer selection

- `verify.go`
  - parses `.signatures.json`
  - parses compact JWS (`header.payload.signature`)
  - verifies RS256 signature using mounted RSA public key
  - verifies file hash claims against bundle contents

- `archive.go`
  - reads tar.gz bundle entries into memory for verification
  - normalizes paths for claim matching
  - canonicalizes JSON/YAML before hashing (OPA-style semantics)
  - strips `.signatures.json` and rebuilds a tar.gz archive

- `files.go`
  - small file helpers (trim mounted files, atomic writes, SHA-256 checksums)

## End-to-End Flow

`FetchAndVerifyToFileIfChanged(...)` performs the full refresh decision and write path:

1. Validate required config (`BundleRef`, token file, public key file, output file)
2. Read and trim the mounted GitHub token file
3. Read the remote OCI descriptor digest (cheap change detection)
4. If the remote digest is unchanged:
   - hash the local mirrored bundle file
   - skip download if checksum matches last known local checksum
   - otherwise continue (self-heal local corruption/deletion)
5. Pull OCI bundle archive bytes (compressed tar.gz layer)
6. Verify signature and file hashes using the mounted public key
7. Strip `.signatures.json` from the archive
8. Atomically write the unsigned mirrored bundle to the configured output path
9. Return new remote digest + local checksum to the caller (fetcher loop state)

## Signature / Trust Model

Current trust model:

- The discovery fetcher verifies the signed upstream bundle once.
- The mirrored bundle served by `nginx` is intentionally rewritten **without** `.signatures.json`.
- OPA sidecars consume the mirrored bundle without local signing/key configuration.

This means:

- signature verification is centralized in the discovery pod
- sidecars no longer need GitHub tokens or public verification keys

## Important Behavior Notes

- Local corruption detection:
  - The package checks both remote OCI digest and local file checksum.
  - If the upstream digest is unchanged but the local file is missing/corrupt, it refetches and repairs.

- Atomic writes:
  - The mirrored bundle is written to `<output>.tmp` and then renamed.
  - This avoids serving partially written archives while `nginx` is reading.

- Structured file hashing:
  - JSON / YAML / `.manifest` files are canonicalized before hashing.
  - This matches OPA bundle signing semantics and avoids false mismatches due to formatting.

## Current Assumptions / Limitations

- Only `RS256` JWS signatures are supported.
- Only the first signature in `.signatures.json` is validated.
- Single-key verification is the current operational model.
- `kid` / `keyid` enforcement is optional (configurable by caller).
- Bundle payload is expected in the last OCI layer (works for current bundle layout).

## Minimal Example (caller perspective)

The fetcher sidecar typically calls:

```go
cfg := discoveryserver.Config{
    BundleRef:       "ghcr.io/example/opa-bundle:latest",
    GithubTokenFile: "/var/run/accesserator/opa-secret/github-token",
    PublicKeyFile:   "/var/run/accesserator/opa-public-key/public_sign_key",
    OutputFile:      "/usr/share/nginx/html/bundles/authz.tar.gz",
}

lastDigest, lastChecksum, changed, err := discoveryserver.FetchAndVerifyToFileIfChanged(
    ctx, cfg, previousDigest, previousChecksum,
)
```

`changed == false` means the remote digest and local mirrored bundle checksum were unchanged, so no rewrite was needed.
