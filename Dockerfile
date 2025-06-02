#------------------
# Build stage
#------------------

FROM golang:1.23 AS builder


# 1.1. Declare variabls
#      docker build --build-arg SERVICE_NAME=datacollector
ARG SERVICE_NAME
ARG SERVICE_PORT

WORKDIR /workspace

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Copy packages
COPY pkg ./pkg

# Copy only the service folder
COPY . .

# Download modules 
RUN go mod download

# Build the binary
RUN cd apps/${SERVICE_NAME} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -o /workspace/${SERVICE_NAME} .


#------------------
# Final stage
#------------------

FROM alpine:latest

# Add ca-certificates for SSH/HTTPS
RUN apk --no-cache add ca-certificates
RUN mkdir -p /etc/ham

# Copy binary
COPY --from=builder /workspace/${SERVICE_NAME} /usr/local/bin/${SERVICE_NAME}
# Copy runtime config
COPY  docconfig.json /etc/ham/docconfig.json

# Expose port
EXPOSE ${SERVICE_PORT}

# Entry point
ENTRYPOINT ["/bin/sh", "-c", "/usr/local/bin/"${SERVICE_NAME}]
