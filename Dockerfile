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

ARG SERVICE_NAME
ARG SERVICE_PORT
ENV SERVICE_NAME=${SERVICE_NAME} 
# Add ca-certificates for SSH/HTTPS
RUN apk --no-cache add ca-certificates && mkdir -p /etc/ham

# Copy binary
COPY --from=builder /workspace/${SERVICE_NAME} /usr/local/bin/${SERVICE_NAME}
RUN chmod +x /usr/local/bin/${SERVICE_NAME}
# Copy runtime config
COPY  docconfig.json /etc/ham/docconfig.json

# Expose port
EXPOSE ${SERVICE_PORT}
# Verify the binary exists and has execute permissions
RUN ls -la /usr/local/bin/${SERVICE_NAME} && \
    [ -f /usr/local/bin/${SERVICE_NAME} ] || (echo "Binary missing!" && exit 1)
RUN echo "Service binary: /usr/local/bin/${SERVICE_NAME}" && \
    ls -la /usr/local/bin/

# Entry point
CMD /usr/local/bin/${SERVICE_NAME}
