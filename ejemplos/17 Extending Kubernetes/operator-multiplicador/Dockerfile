FROM golang:1.20-alpine AS build
RUN apk add --no-cache git build-base
WORKDIR /src
COPY . .
RUN go build -o /out/multiplicador-operator ./

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /out/multiplicador-operator /usr/local/bin/multiplicador-operator
ENTRYPOINT ["/usr/local/bin/multiplicador-operator"]
