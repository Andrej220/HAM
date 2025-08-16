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
COPY apps/${SERVICE_NAME}/config.yaml /workspace/


#------------------
# Final stage
#------------------

FROM alpine:3.20

ARG SERVICE_NAME
ARG SERVICE_PORT
ENV SERVICE_NAME=${SERVICE_NAME} 
# Add ca-certificates for SSH/HTTPS
RUN apk --no-cache add ca-certificates && mkdir -p /etc/ham

# Copy binary
COPY --from=builder /workspace/${SERVICE_NAME} /usr/local/bin/app 
RUN chmod +x /usr/local/bin/app 
# Copy runtime config
COPY  docconfig.json /etc/ham/docconfig.json
COPY  --from=builder /workspace/config.yaml  /etc/ham/config.yaml

# Entry point
ENTRYPOINT ["/usr/local/bin/app"]
