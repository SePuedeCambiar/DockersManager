# --- ETAPA 1: Construcción ---
FROM golang:1.21-alpine AS builder

# Instalamos git porque algunas dependencias de Go lo requieren
RUN apk add --no-cache git

WORKDIR /app

# Copiamos los archivos de dependencias primero para aprovechar la caché de Docker
COPY go.mod go.sum ./
RUN go mod download

# Copiamos el resto del código
COPY . .

# Compilamos el binario. 
# CGO_ENABLED=0 hace que el binario sea estático y funcione en cualquier distro de Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o manager ./cmd/manager/main.go

# --- ETAPA 2: Ejecución ---
FROM alpine:latest

# Instalamos certificados CA para que el programa pueda hacer peticiones HTTPS (para la fase 3)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copiamos solo el binario compilado desde la etapa anterior
COPY --from=builder /app/manager .
COPY --from=builder /app/.env .env

# Ejecutamos el binario
CMD ["./manager"]