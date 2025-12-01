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
RUN CGO_ENABLED=1 go build -o main .

FROM alpine:latest
RUN apk add --no-cache libc6-compat curl

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Install Python via uv
RUN uv python install 3.12

WORKDIR /app
COPY --from=builder /app/main .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./main"]
