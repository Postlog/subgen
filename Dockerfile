# syntax=docker/dockerfile:1
#
# subgen container image. Multi-stage: build a fully static, pure-Go binary
# (modernc sqlite → CGO off), then ship it on distroless/static (no shell, runs as
# nonroot). Templates, static assets and the SQL schema are embedded in the binary
# (go:embed), so the runtime image needs nothing else.
#
# RU1 has little RAM (~376 MB, no swap) — build the image locally / in CI, not on
# the node; ship it via a registry or `docker save | ssh … docker load`.

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/subgen ./cmd/service

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/subgen /app/subgen
# Operational data (SQLite) is written under ./db (relative to WORKDIR) — mount a
# volume at /app/db to persist it. Bootstrap config comes from the environment.
EXPOSE 2097
ENTRYPOINT ["/app/subgen"]
