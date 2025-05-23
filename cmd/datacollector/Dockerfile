# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy all source files
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o ssh-executor executor.go

# Final stage
FROM alpine:latest

# Add ca-certificates for SSH/HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/ssh-executor .
COPY docconfig.json .

# Expose port
EXPOSE 8081

# Run the app
CMD ["./ssh-executor"]
