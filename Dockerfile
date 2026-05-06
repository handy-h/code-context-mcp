# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -v -trimpath -ldflags="-s -w" -o code-context-mcp ./cmd/code-context-mcp

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and config files
COPY --from=builder /app/cmd/code-context-mcp/code-context-mcp /app/
COPY --from=builder /app/.env.example /app/
COPY --from=builder /app/LICENSE /app/
COPY --from=builder /app/README.md /app/
COPY --from=builder /app/docs /app/docs/

# Create a non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

USER appuser

# Expose port (if needed for health checks)
EXPOSE 8080

# Set environment variables
ENV OLLAMA_URL=http://localhost:11434
ENV OLLAMA_EMBED_MODEL=nomic-embed-text:latest
ENV EMBEDDING_DIM=768
ENV SCAN_EXTENSIONS=.go,.vue,.js,.ts,.py,.md
ENV CHUNK_SIZE=800
ENV MAX_CHUNK_SIZE=1500
ENV AUTO_INDEX=true

# Health check (optional)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["/app/code-context-mcp"]
