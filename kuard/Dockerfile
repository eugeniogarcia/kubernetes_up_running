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

# Copy the built binary
COPY --from=build /go/bin/kuard /kuard

# Copy application files into the image so the runtime FS browser can read them
COPY . /app

# Ensure the runtime user can read the files and set working dir
RUN chown -R nobody:nobody /app /kuard
WORKDIR /app

USER nobody:nobody
EXPOSE 8080

ENTRYPOINT ["/kuard"]
