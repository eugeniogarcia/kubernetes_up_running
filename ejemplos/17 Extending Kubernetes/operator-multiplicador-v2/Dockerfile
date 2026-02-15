#*************************************************************
#*************************************************************
# usamos una imagen base de Go para compilar el operador
#*************************************************************
#*************************************************************
FROM golang:1.25-alpine AS build 
RUN apk add --no-cache git build-base

# creamos el directorio de trabajo y copiamos el código fuente
WORKDIR /src
COPY . .

# instalamos las dependencias
RUN go mod download  

RUN go build -o /out/multiplicador-operator ./

#*************************************************************
#*************************************************************
# creamos la imagen final basada en Alpine para ejecutar el operador
#*************************************************************
#*************************************************************
FROM alpine:latest

# instala ca-certificates en runtime (útil para conexiones HTTPS si el operador las usa)
RUN apk add --no-cache ca-certificates 

# Copiar el binario compilado
COPY --from=build /out/multiplicador-operator /usr/local/bin/multiplicador-operator

ENTRYPOINT ["/usr/local/bin/multiplicador-operator"]
