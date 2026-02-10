FROM golang:alpine AS build
RUN apk add --no-cache git build-base
WORKDIR /src
COPY . .
RUN go build -o /out/multiplicador-operator ./

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /out/multiplicador-operator /usr/local/bin/multiplicador-operator
ENTRYPOINT ["/usr/local/bin/multiplicador-operator"]
