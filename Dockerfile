FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o main .

FROM alpine:latest
RUN apk add --no-cache libc6-compat curl

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Install Python via uv
RUN uv python install 3.12

WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/frontend/dist ./frontend/dist
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./main"]
