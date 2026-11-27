# Databases stand TLS material

`make e2e-databases` runs `gencerts.sh` before it starts the stand. The script
writes a fresh test-only CA and server certificate for `localhost`,
`127.0.0.1` and `db.local` to `tls/` (gitignored), used by the databases stand
(`deploy/e2e/databases.yaml`) and by `TestVerifyFullRealPostgres`. No key is
committed; the CA key is never written to disk outside a temporary directory.
