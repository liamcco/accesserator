package discoveryserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"sigs.k8s.io/yaml"
)

const (
	bundleSignaturesFileName = ".signatures.json"
)

type Config struct {
	BundleRef       string
	GithubTokenFile string
	PublicKeyFile   string
	ExpectedKeyID   string
	OutputFile      string
}

func FetchAndVerifyToFile(ctx context.Context, cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	token, err := readOptionalTrimmedFile(cfg.GithubTokenFile)
	if err != nil {
		return fmt.Errorf("read github token: %w", err)
	}
	publicKeyPEM, err := os.ReadFile(cfg.PublicKeyFile)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}

	mirroredBundle, err := fetchOCIBundleArchive(ctx, cfg.BundleRef, token)
	if err != nil {
		return err
	}
	if err := verifyBundleArchiveSignature(mirroredBundle, publicKeyPEM, cfg.ExpectedKeyID); err != nil {
		return err
	}

	unsignedBundle, err := stripBundleSignatureFile(mirroredBundle)
	if err != nil {
		return err
	}

	return writeFileAtomically(cfg.OutputFile, unsignedBundle, 0o644)
}

func validateConfig(cfg Config) error {
	switch {
	case cfg.BundleRef == "":
		return fmt.Errorf("missing bundle ref")
	case cfg.GithubTokenFile == "":
		return fmt.Errorf("missing github token file")
	case cfg.PublicKeyFile == "":
		return fmt.Errorf("missing public key file")
	case cfg.OutputFile == "":
		return fmt.Errorf("missing output file")
	default:
		return nil
	}
}

func readOptionalTrimmedFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func writeFileAtomically(outputPath string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, mode); err != nil {
		return fmt.Errorf("write temp bundle file: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move bundle file into place: %w", err)
	}
	return nil
}

func stripBundleSignatureFile(bundleArchive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundleArchive))
	if err != nil {
		return nil, fmt.Errorf("open bundle gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	var out bytes.Buffer
	gzipWriter := gzip.NewWriter(&out)
	tarReader := tar.NewReader(gzipReader)
	tarWriter := tar.NewWriter(gzipWriter)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("read bundle archive: %w", err)
		}
		if header == nil {
			continue
		}

		if normalizeBundlePath(header.Name) == bundleSignaturesFileName {
			// Signature metadata is removed after server-side verification so OPA
			// sidecars can consume the mirrored bundle without local key config.
			continue
		}

		cloned := *header
		if err := tarWriter.WriteHeader(&cloned); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write bundle header %s: %w", header.Name, err)
		}

		if header.FileInfo().Mode().IsRegular() {
			if _, err := io.Copy(tarWriter, tarReader); err != nil {
				_ = tarWriter.Close()
				_ = gzipWriter.Close()
				return nil, fmt.Errorf("copy bundle file %s: %w", header.Name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}
	return out.Bytes(), nil
}

func fetchOCIBundleArchive(ctx context.Context, bundleRef, githubToken string) ([]byte, error) {
	ref, err := name.ParseReference(bundleRef)
	if err != nil {
		return nil, fmt.Errorf("parse bundle ref: %w", err)
	}

	options := []remote.Option{remote.WithContext(ctx)}
	if githubToken != "" {
		options = append(options, remote.WithAuth(&authn.Basic{
			Username: "oauth2",
			Password: githubToken,
		}))
	} else {
		options = append(options, remote.WithAuth(authn.Anonymous))
	}

	img, err := remote.Image(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("pull bundle image: %w", err)
	}

	layer, err := selectBundleLayer(img)
	if err != nil {
		return nil, err
	}
	reader, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("open compressed bundle layer: %w", err)
	}
	defer func() { _ = reader.Close() }()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read compressed bundle layer: %w", err)
	}
	return content, nil
}

func selectBundleLayer(img v1.Image) (v1.Layer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("list bundle layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("bundle image has no layers")
	}
	return layers[len(layers)-1], nil
}

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

func readBundleArchiveFiles(bundleArchive []byte) (map[string][]byte, []byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundleArchive))
	if err != nil {
		return nil, nil, fmt.Errorf("open bundle gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)

	files := map[string][]byte{}
	var signatures []byte
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read bundle archive: %w", err)
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}

		name := normalizeBundlePath(header.Name)
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, nil, fmt.Errorf("read bundle file %s: %w", name, err)
		}

		if name == bundleSignaturesFileName {
			signatures = content
			continue
		}
		files[name] = content
	}

	if len(signatures) == 0 {
		return nil, nil, fmt.Errorf("bundle is missing %s", bundleSignaturesFileName)
	}
	return files, signatures, nil
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

func normalizeBundlePath(p string) string {
	clean := path.Clean(strings.TrimSpace(p))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	return clean
}

func canonicalizeBundleFileForHash(name string, content []byte) ([]byte, error) {
	if !isStructuredBundleFile(name) {
		return content, nil
	}

	var jsonValue any
	switch {
	case strings.HasSuffix(name, ".json") || name == ".manifest":
		if err := json.Unmarshal(content, &jsonValue); err != nil {
			return nil, err
		}
	case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
		jsonBytes, err := yaml.YAMLToJSON(content)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(jsonBytes, &jsonValue); err != nil {
			return nil, err
		}
	default:
		return content, nil
	}

	// encoding/json emits deterministic object-key ordering, which matches the
	// canonicalized structured-file hashing OPA describes for bundle signatures.
	return json.Marshal(jsonValue)
}

func isStructuredBundleFile(name string) bool {
	return name == ".manifest" ||
		strings.HasSuffix(name, ".json") ||
		strings.HasSuffix(name, ".yaml") ||
		strings.HasSuffix(name, ".yml")
}
