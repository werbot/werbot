# Contributing

## DCO

Every commit must be signed off (Developer Certificate of Origin):

```sh
git commit -s
```

## Local setup

Requirements: Go 1.27, Bun.

```sh
docker compose -f deploy/docker-compose.test.yaml up -d
make test
```
