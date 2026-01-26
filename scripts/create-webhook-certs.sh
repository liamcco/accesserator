set -euo pipefail

OUT="$(pwd)/webhook-certs"
mkdir -p "$OUT"

# 1) Create a CA (self-signed)
openssl genrsa -out "$OUT/ca.key" 4096
openssl req -x509 -new -nodes -key "$OUT/ca.key" -sha256 -days 3650 \
  -subj "/CN=accesserator-webhook-ca" \
  -out "$OUT/ca.crt"

# 1b) Write caBundle file (base64-encoded, for webhook configurations)
base64 < "$OUT/ca.crt" | tr -d '\n' > "$OUT/caBundle"

# 2) Create a server key
openssl genrsa -out "$OUT/tls.key" 2048

# 3) CSR config with SANs (IMPORTANT)
cat > "$OUT/server.cnf" <<'EOF'
[ req ]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = dn
req_extensions     = req_ext

[ dn ]
CN = accesserator-webhook

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = host.docker.internal
DNS.2 = localhost
IP.1  = 127.0.0.1
EOF

# 4) Create CSR
openssl req -new -key "$OUT/tls.key" -out "$OUT/server.csr" -config "$OUT/server.cnf"

# 5) Sign server cert with CA
cat > "$OUT/v3.ext" <<'EOF'
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = host.docker.internal
DNS.2 = localhost
IP.1  = 127.0.0.1
EOF

openssl x509 -req -in "$OUT/server.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" -CAcreateserial \
  -out "$OUT/tls.crt" -days 365 -sha256 -extfile "$OUT/v3.ext"

echo "Wrote: $OUT/ca.crt, $OUT/caBundle, $OUT/tls.crt, $OUT/tls.key"