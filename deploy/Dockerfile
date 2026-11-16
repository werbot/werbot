FROM oven/bun:1.3 AS web
ARG VERSION=dev
WORKDIR /src/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN VITE_WERBOT_VERSION=$VERSION bun run build

FROM golang:1.27-alpine AS go
ARG VERSION=dev
ARG COMMIT=none
RUN apk add --no-cache make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build ./web/build
RUN CGO_ENABLED=0 make -o web build VERSION=$VERSION COMMIT=$COMMIT

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /src/bin/werbot /werbot
ENTRYPOINT ["/werbot"]
CMD ["server"]
