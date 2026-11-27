#!/bin/sh
# Generates the test-only TLS material of the databases stand
# (deploy/e2e/databases.yaml) into tls/ (gitignored): a CA and a server
# certificate for localhost, 127.0.0.1 and db.local, valid for 10 years.
# make e2e-databases runs it before every stand start. Run from any directory.
set -eu
mkdir -p "$(dirname "$0")/tls"
cd "$(dirname "$0")/tls"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

openssl ecparam -name prime256v1 -genkey -noout -out "$tmp/ca-key.pem"
openssl req -x509 -new -key "$tmp/ca-key.pem" -sha256 -days 3650 \
  -subj "/CN=werbot databases stand test CA" \
  -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -out ca.pem

openssl ecparam -name prime256v1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt -out server-key.pem
openssl req -new -key server-key.pem -subj "/CN=localhost" -out "$tmp/server.csr"
printf '%s\n' "basicConstraints=critical,CA:FALSE" "keyUsage=critical,digitalSignature" \
  "extendedKeyUsage=serverAuth" "subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:db.local" > "$tmp/server.ext"
openssl x509 -req -in "$tmp/server.csr" -CA ca.pem -CAkey "$tmp/ca-key.pem" -CAcreateserial \
  -CAserial "$tmp/ca.srl" -sha256 -days 3650 -extfile "$tmp/server.ext" -out server.pem
