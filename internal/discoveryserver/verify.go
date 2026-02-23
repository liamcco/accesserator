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
	Signatures []string `json:"signatures"`
}

type jwsHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid,omitempty"`
}

type signaturePayload struct {
	Files []signaturePayloadFile `json:"files"`
	KeyID string                 `json:"keyid,omitempty"`
}

type signaturePayloadFile struct {
	Name      string `json:"name"`
	Hash      string `json:"hash"`
	Algorithm string `json:"algorithm,omitempty"`
}

func verifyBundleArchiveSignature(bundleArchive, publicKeyPEM []byte, expectedKeyID string) error {
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

	header, payload, signingInput, sigBytes, err := parseJWSCompact(signatureFile.Signatures[0])
	if err != nil {
		return err
	}
	if expectedKeyID != "" {
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

	return verifyFileHashes(files, payload.Files)
}

func parseJWSCompact(compact string) (jwsHeader, signaturePayload, []byte, []byte, error) {
	var header jwsHeader
	var payload signaturePayload

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

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return header, payload, nil, nil, fmt.Errorf("decode jws signature: %w", err)
	}

	return header, payload, []byte(parts[0] + "." + parts[1]), sigBytes, nil
}

func verifyRS256(signingInput, signature, publicKeyPEM []byte) error {
	pubKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(signingInput)
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, sum[:], signature); err != nil {
		return fmt.Errorf("verify bundle signature: %w", err)
	}
	return nil
}

func parseRSAPublicKey(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("decode public key pem: no pem block found")
	}

	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if rsaKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("certificate public key is not RSA")
	}

	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := parsed.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("public key is not RSA")
	}

	if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaKey, nil
	}

	return nil, fmt.Errorf("unsupported RSA public key format")
}

func verifyFileHashes(bundleFiles map[string][]byte, claims []signaturePayloadFile) error {
	if len(claims) == 0 {
		return fmt.Errorf("bundle signature payload contains no file claims")
	}
	if len(bundleFiles) != len(claims) {
		return fmt.Errorf("bundle file set does not match signature payload")
	}

	for _, claim := range claims {
		name := normalizeBundlePath(claim.Name)
		content, ok := bundleFiles[name]
		if !ok {
			return fmt.Errorf("bundle file %q missing from signature payload", name)
		}
		alg := strings.ToUpper(strings.ReplaceAll(claim.Algorithm, "-", ""))
		if alg == "" {
			alg = "SHA256"
		}
		if alg != "SHA256" {
			return fmt.Errorf("unsupported file hash algorithm %q for %s", claim.Algorithm, name)
		}
		hashInput, err := canonicalizeBundleFileForHash(name, content)
		if err != nil {
			return fmt.Errorf("canonicalize bundle file %s: %w", name, err)
		}
		sum := sha256.Sum256(hashInput)
		if !sha256ClaimMatches(sum[:], claim.Hash) {
			return fmt.Errorf("hash mismatch for bundle file %s", name)
		}
	}

	return nil
}

func sha256ClaimMatches(sum []byte, claimHash string) bool {
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
