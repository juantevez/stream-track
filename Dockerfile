# --- Etapa 1: Compilación ---
# Cambiamos 1.22 por 1.25 para que coincida con tu go.mod
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copiar archivos de dependencias primero para aprovechar el cache de Docker
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código fuente
COPY . .

# Compilar el binario estático optimizado para producción
RUN CGO_ENABLED=0 GOOS=linux go build -o streamtrack-app ./cmd/server/main.go

# --- Etapa 2: Imagen de Ejecución Ejecutable ---
FROM alpine:3.19

WORKDIR /app

# Instalar certificados CA e instalar la data de zonas horarias (tzdata)
RUN apk --no-cache add ca-certificates tzdata

# Copiar el binario desde la etapa de compilación
COPY --from=builder /app/streamtrack-app .

# Definir variables de entorno por defecto
ENV TZ=America/Argentina/Buenos_Aires

CMD ["./streamtrack-app"]
