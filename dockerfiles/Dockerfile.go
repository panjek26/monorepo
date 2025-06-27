# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Optional: Install Git and SSL roots (if needed by Go modules)
RUN apk add --no-cache git ca-certificates

COPY go-services/ .

RUN go mod tidy && \
    go build -o app .

# Stage 2: Run
FROM gcr.io/distroless/static:nonroot
WORKDIR /

# Copy from builder
COPY --from=builder /app/app /

# Use non-root user
USER nonroot:nonroot

ENTRYPOINT ["/app"]
