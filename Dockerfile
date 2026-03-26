FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.25-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends build-essential ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o main .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gcc \
    g++ \
    libffi-dev \
    libssl-dev \
    libcurl4-openssl-dev \
    gosu \
    && rm -rf /var/lib/apt/lists/*
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv python install 3.12 && rm -rf /root/.cache
RUN groupadd -g 1000 appgroup && useradd -u 1000 -g appgroup -m -d /home/appuser -s /usr/sbin/nologin appuser
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && mkdir -p /app/data && chown -R appuser:appgroup /app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD curl -f http://localhost:8080/health || exit 1
ENTRYPOINT ["/entrypoint.sh"]
CMD ["./main"]
