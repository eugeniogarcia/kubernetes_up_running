FROM golang:1.25-alpine AS build
RUN apk add --no-cache git build-base
WORKDIR /src
COPY . .
RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN go build -gcflags="all=-N -l" -o /out/multiplicador-operator ./

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/dlv /usr/local/bin/dlv
COPY --from=build /out/multiplicador-operator /usr/local/bin/multiplicador-operator
ENTRYPOINT ["/usr/local/bin/dlv", "--listen=:40000", "--headless=true", "exec", "/usr/local/bin/multiplicador-operator"]
