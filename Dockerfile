FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o main .

FROM alpine:3.20
RUN apk add --no-cache libc6-compat curl && rm -rf /var/cache/apk/*
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv python install 3.12 && rm -rf /root/.cache
RUN addgroup -g 1000 appgroup && adduser -u 1000 -G appgroup -D appuser
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN mkdir -p /app/data && chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD curl -f http://localhost:8080/health || exit 1
CMD ["./main"]
