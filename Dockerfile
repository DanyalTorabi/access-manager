# syntax=docker/dockerfile:1
# Build from repo root; context includes ./go (T19).
# Runtime: distroless non-root; SQLite DB on tmpfs (see docker-compose.yml).

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./
# Image platform matches target (linux/amd64 or linux/arm64); no CGO for modernc.org/sqlite.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN mkdir -p /out/migrations && cp -r migrations/sqlite /out/migrations/sqlite

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /home/nonroot
COPY --from=build /out/server /home/nonroot/server
COPY --from=build /out/migrations /home/nonroot/migrations
# Relative to WORKDIR: migrations live under ./migrations/sqlite
ENV MIGRATIONS_DIR=migrations/sqlite
# Override in compose for container networking and writable DB path
ENV HTTP_ADDR=0.0.0.0:8080
ENV DATABASE_URL=file:/data/access.db?_pragma=foreign_keys(1)
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/home/nonroot/server"]
