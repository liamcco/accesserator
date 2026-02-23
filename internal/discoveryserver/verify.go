package discoveryserver

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
)

type bundleSignatures struct {
	// OPA stores one or more compact JWS strings in .signatures.json.
	// Example shape: {"signatures":["<header>.<payload>.<sig>"]}.
	Signatures []string `json:"signatures"`
}

type jwsHeader struct {
	// Only RS256 is supported in the current fetcher implementation.
	Alg string `json:"alg"`
	// Optional key identifier metadata from the signer.
	Kid string `json:"kid,omitempty"`
}

type signaturePayload struct {
	// One claim per bundle file (excluding .signatures.json).
	Files []signaturePayloadFile `json:"files"`
	// Optional signer key ID duplicated in payload metadata by some tooling.
	KeyID string `json:"keyid,omitempty"`
}

type signaturePayloadFile struct {
	// Path inside the tar.gz bundle.
	Name string `json:"name"`
	// Expected SHA-256 digest encoded as hex/base64/base64url (supported below).
	Hash string `json:"hash"`
	// Optional hash algorithm label; defaults to SHA256 if omitted.
	Algorithm string `json:"algorithm,omitempty"`
}

func verifyBundleArchiveSignature(bundleArchive, publicKeyPEM []byte, expectedKeyID string) error {
	// Verification is intentionally two-step:
	// 1. verify the JWS signature over the signature payload
	// 2. verify each bundle file hash claimed in that payload
	// This prevents writing tampered bundle content into the mirrored path.
	// Example: if `data.json` inside the tar.gz is modified after signing, the JWS
	// may still parse, but `verifyFileHashes(...)` will fail with a hash mismatch.
	files, signaturesJSON, err := readBundleArchiveFiles(bundleArchive)
	if err != nil {
		return err
	}

	signatureFile := bundleSignatures{}
	if err := json.Unmarshal(signaturesJSON, &signatureFile); err != nil {
		return fmt.Errorf("parse %s: %w", bundleSignaturesFileName, err)
	}
	if len(signatureFile.Signatures) == 0 {
		return fmt.Errorf("bundle signature file contains no signatures")
	}

	// Current implementation validates the first signature only, which matches
	// the current single-signer setup. Multi-signature/key-rotation support would
	// iterate and accept any configured trusted signer.
	header, payload, signingInput, sigBytes, err := parseJWSCompact(signatureFile.Signatures[0])
	if err != nil {
		return err
	}
	if expectedKeyID != "" {
		// kid/keyid checks are optional metadata policy checks. Cryptographic
		// verification against the mounted public key is still the primary trust
		// decision when running with a single trusted key.
		if header.Kid != "" && header.Kid != expectedKeyID {
			return fmt.Errorf("unexpected jws header kid %q", header.Kid)
		}
		if payload.KeyID != "" && payload.KeyID != expectedKeyID {
			return fmt.Errorf("unexpected signature payload keyid %q", payload.KeyID)
		}
	}

	if err := verifyRS256(signingInput, sigBytes, publicKeyPEM); err != nil {
		return err
	}

	// Once the JWS is trusted, the payload becomes the source of truth for the
	// expected file set + hashes in the bundle archive.
	return verifyFileHashes(files, payload.Files)
}

func parseJWSCompact(compact string) (jwsHeader, signaturePayload, []byte, []byte, error) {
	var header jwsHeader
	var payload signaturePayload

	// Compact JWS format is exactly three base64url segments:
	// "<header>.<payload>.<signature>".
	// Example input:
	// "eyJhbGciOiJSUzI1NiIsImtpZCI6ImRlZmF1bHQifQ.eyJmaWxlcyI6W119.ABC..."
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return header, payload, nil, nil, fmt.Errorf("invalid jws compact signature")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return header, payload, nil, nil, fmt.Errorf("decode jws header: %w", err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return header, payload, nil, nil, fmt.Errorf("parse jws header: %w", err)
	}
	// Keep algorithm handling narrow on purpose. If more algorithms are ever
	// needed, add them explicitly instead of silently accepting alternatives.
	if !strings.EqualFold(header.Alg, "RS256") {
		return header, payload, nil, nil, fmt.Errorf("unsupported jws alg %q", header.Alg)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return header, payload, nil, nil, fmt.Errorf("decode jws payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return header, payload, nil, nil, fmt.Errorf("parse jws payload: %w", err)
	}

	// The third segment is the detached RSA signature bytes over signingInput.
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return header, payload, nil, nil, fmt.Errorf("decode jws signature: %w", err)
	}

	// JWS signs the ASCII "<base64url(header)>.<base64url(payload)>" string,
	// not the decoded JSON bytes.
	return header, payload, []byte(parts[0] + "." + parts[1]), sigBytes, nil
}

func verifyRS256(signingInput, signature, publicKeyPEM []byte) error {
	// Parse the mounted key on each verification call. This keeps the helper pure
	// and simple; if verification throughput grows, this could be cached.
	// Example: verifies the JWS signature bytes for
	// `base64url(header)+"."+base64url(payload)` using the mounted RSA public key.
	pubKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(signingInput)
	// RSA PKCS#1 v1.5 is what JWS RS256 uses under the hood.
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, sum[:], signature); err != nil {
		return fmt.Errorf("verify bundle signature: %w", err)
	}
	return nil
}

func parseRSAPublicKey(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	// Accept certificate PEM, PKIX public key PEM, and PKCS#1 public key PEM so
	// operators can mount common key formats without conversion.
	// Examples accepted:
	// - "-----BEGIN CERTIFICATE-----"
	// - "-----BEGIN PUBLIC KEY-----"
	// - "-----BEGIN RSA PUBLIC KEY-----"
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("decode public key pem: no pem block found")
	}

	// Try certificate first because many users mount an X.509 cert instead of a
	// bare public key. We only extract the RSA public key from it.
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if rsaKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("certificate public key is not RSA")
	}

	// Then try generic PKIX SubjectPublicKeyInfo PEM.
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := parsed.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("public key is not RSA")
	}

	// Finally accept legacy PKCS#1 RSA public key PEM.
	if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaKey, nil
	}

	return nil, fmt.Errorf("unsupported RSA public key format")
}

func verifyFileHashes(bundleFiles map[string][]byte, claims []signaturePayloadFile) error {
	// Example: claim `{name:"./data.json", hash:"..."}` is normalized to
	// `data.json`, canonicalized (for JSON/YAML), hashed, and compared to the
	// expected digest from the signature payload.
	if len(claims) == 0 {
		return fmt.Errorf("bundle signature payload contains no file claims")
	}
	// Require an exact file-set match so unexpected extra files in the bundle are
	// rejected, not just missing claimed files.
	if len(bundleFiles) != len(claims) {
		return fmt.Errorf("bundle file set does not match signature payload")
	}

	for _, claim := range claims {
		// Normalize path names before matching because tar headers and signature
		// payloads may represent the same path differently ("./x" vs "x").
		name := normalizeBundlePath(claim.Name)
		content, ok := bundleFiles[name]
		if !ok {
			return fmt.Errorf("bundle file %q missing from signature payload", name)
		}
		alg := strings.ToUpper(strings.ReplaceAll(claim.Algorithm, "-", ""))
		if alg == "" {
			alg = "SHA256"
		}
		// Fail closed on unknown algorithms to avoid false verification success.
		if alg != "SHA256" {
			return fmt.Errorf("unsupported file hash algorithm %q for %s", claim.Algorithm, name)
		}
		hashInput, err := canonicalizeBundleFileForHash(name, content)
		if err != nil {
			return fmt.Errorf("canonicalize bundle file %s: %w", name, err)
		}
		sum := sha256.Sum256(hashInput)
		// Claim hashes are compared after canonicalization because OPA signs the
		// semantic content of JSON/YAML files, not their original formatting.
		if !sha256ClaimMatches(sum[:], claim.Hash) {
			return fmt.Errorf("hash mismatch for bundle file %s", name)
		}
	}

	return nil
}

func sha256ClaimMatches(sum []byte, claimHash string) bool {
	// Support common SHA-256 encodings found in signed bundles (hex/base64/base64url,
	// with or without a "sha256:" prefix) while comparing the same raw digest.
	// Example accepted forms for the same digest:
	//   "ab12..." (hex)
	//   "sha256:ab12..." (prefixed hex)
	//   "qxI..." (base64/base64url)
	claimHash = strings.TrimSpace(claimHash)
	claimHash = strings.TrimPrefix(strings.ToLower(claimHash), "sha256:")

	hexDigest := hex.EncodeToString(sum)
	if strings.EqualFold(hexDigest, claimHash) {
		return true
	}

	if decoded, err := base64.StdEncoding.DecodeString(claimHash); err == nil && bytes.Equal(decoded, sum) {
		return true
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(claimHash); err == nil && bytes.Equal(decoded, sum) {
		return true
	}
	if decoded, err := base64.URLEncoding.DecodeString(claimHash); err == nil && bytes.Equal(decoded, sum) {
		return true
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(claimHash); err == nil && bytes.Equal(decoded, sum) {
		return true
	}

	return false
}
