# STAGE 1: Build
FROM golang:alpine AS build

# Install build dependencies
RUN apk add --no-cache git bash npm dos2unix

WORKDIR /go/src/kuard

# Copy all sources in
COPY . .

# Build application
RUN dos2unix build/build.sh && \
    VERBOSE=0 PKG=kuard ARCH=amd64 VERSION=test bash build/build.sh

# STAGE 2: Runtime
FROM alpine:latest

# Add ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

COPY --from=build --chown=nobody:nobody /go/bin/kuard /kuard

USER nobody:nobody
EXPOSE 8080

ENTRYPOINT ["/kuard"]
