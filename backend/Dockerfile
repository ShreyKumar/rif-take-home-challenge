# Multi-stage build for the mutant-detector service.
# Build from the REPO ROOT so the context includes both backend/ and frontend/:
#   docker build -f backend/Dockerfile -t mutant-detector .
#
# The pure-Go SQLite driver (modernc.org/sqlite) means CGO_ENABLED=0 yields a
# fully static binary — no cgo, no libc — so the runtime image can be minimal.

FROM golang:1.26-alpine AS build
WORKDIR /src/backend

# Cache module downloads on the go.mod/go.sum layer.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.22
RUN adduser -D -u 10001 app && mkdir -p /app/data && chown app /app/data
WORKDIR /app

COPY --from=build /out/server /app/server
COPY frontend/ /app/frontend/

ENV PORT=8080 \
    DB_PATH=/app/data/mutant.db \
    FRONTEND_DIR=/app/frontend

EXPOSE 8080
VOLUME ["/app/data"]
USER app
ENTRYPOINT ["/app/server"]
