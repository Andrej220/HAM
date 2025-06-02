# Build stage
FROM golang:1.23 AS builder

WORKDIR /workspace

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Copy all source files
COPY pkg ./pkg
COPY apps/datacollector ./apps/datacollector

# Download modules 
RUN go mod download

# Build the binary
RUN cd apps/datacollector && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -o /workspace/datacollector-service .

# Final stage
FROM alpine:latest

# Add ca-certificates for SSH/HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary
COPY --from=builder /workspace/datacollector-service /usr/local/bin/datacollector
# Copy runtime config
COPY  docconfig.json /etc/ham/docconfig.json

# Expose port
EXPOSE 8081

# Entry point
ENTRYPOINT ["/usr/local/bin/datacollector"]
