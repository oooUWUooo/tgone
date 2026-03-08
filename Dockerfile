# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Cache dependency downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

# Copy source (docs/ is embedded at compile time)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-s -w" \
        -trimpath \
        -o habr-rss-bot \
        .

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates  → HTTPS to Habr / Telegram works correctly
# tzdata           → correct timestamps in logs
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /build/habr-rss-bot .

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./habr-rss-bot"]
