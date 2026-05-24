# StreamTrack

Prueba de concepto para monitorear la audiencia en vivo de canales de YouTube. Un worker en Go recolecta métricas de espectadores concurrentes cada 10 minutos durante la ventana horaria configurada y las persiste en PostgreSQL como una serie temporal. Una API REST es expuesta automáticamente por PostgREST para consumo desde cualquier frontend.

## Arquitectura

El proyecto aplica **Arquitectura Hexagonal (Ports & Adapters)**. El núcleo de negocio no conoce ni depende de la infraestructura — solo define interfaces (puertos) que los adaptadores implementan.

```
cmd/server/main.go          → Composition root. Inyecta dependencias y dispara el worker.
internal/
  core/
    domain/                 → Entidades puras: Canal, Metrica
    ports/                  → Interfaces: MetricsRepository, StreamingPlatformClient, TrackingService
    services/               → Caso de uso: EjecutarCicloMonitoreo
  adapters/
    storage/                → Adaptador secundario: PostgreSQL
    streaming/              → Adaptador secundario: YouTube Data API v3
scripts/
  init.sql                  → Schema, índices y datos iniciales de la PoC
```

```
┌─────────────────────────────────────────────────────────┐
│                      main.go (Driver)                   │
│                  Ticker cada 10 minutos                 │
└──────────────────────────┬──────────────────────────────┘
                           │ puerto primario
                           ▼
┌──────────────────────────────────────────────────────────┐
│               TrackingService (Core)                     │
│   1. Obtener canales activos                             │
│   2. Consultar audiencia en vivo (batch)                 │
│   3. Persistir métricas                                  │
└────────────┬─────────────────────────────┬───────────────┘
             │ puerto secundario           │ puerto secundario
             ▼                             ▼
┌────────────────────┐         ┌───────────────────────────┐
│ PostgresRepository │         │     YouTubeAdapter        │
│   (storage)        │         │   (YouTube Data API v3)   │
└────────────────────┘         └───────────────────────────┘
```

## Stack

| Componente | Tecnología |
|---|---|
| Worker | Go 1.25, compilado en imagen Alpine |
| Base de datos | PostgreSQL 16 |
| API REST | PostgREST v12 |
| Contenedores | Docker Compose |

## Requisitos

- Docker y Docker Compose
- API Key de Google Cloud con la **YouTube Data API v3** habilitada

## Configuración

Editá el archivo `.env` o directamente el `docker-compose.yml` y reemplazá el valor de `YOUTUBE_API_KEY`:

```yaml
- YOUTUBE_API_KEY=tu_api_key_de_google_cloud
```

## Levantar el proyecto

```bash
# Primera vez (o si modificaste init.sql)
docker compose down -v
docker compose up -d --build

# Corridas posteriores
docker compose up -d
```

Los tres contenedores que deben quedar corriendo:

```
streamtrack-db       → PostgreSQL en el puerto 5432
streamtrack-api      → PostgREST en el puerto 3000
streamtrack-worker   → Worker Go (sin puerto expuesto)
```

## API REST

PostgREST expone las tablas del schema `public` automáticamente en `http://localhost:3000`.

### Canales configurados

```
GET /canales
GET /canales?activo=eq.true
```

### Historial de audiencia

```bash
# Últimas métricas de un canal
GET /historial_audiencia?canal_id=eq.luzu-tv&order=fecha_hora.desc&limit=50

# Solo registros en vivo (viewers > 0)
GET /historial_audiencia?canal_id=eq.luzu-tv&espectadores=gt.0&order=fecha_hora.desc

# Rango de fechas
GET /historial_audiencia?canal_id=eq.luzu-tv&fecha_hora=gte.2025-01-01T09:00:00&order=fecha_hora.desc
```

El schema OpenAPI completo está disponible en `GET http://localhost:3000/`.

## Comportamiento del Worker

- **Ventana operativa:** lunes a viernes, 09:00 a 13:00 (horario Argentina, UTC-3)
- **Intervalo:** cada 10 minutos
- **Canales configurados (PoC):** Luzu TV y Olga En Vivo
- Si un canal no está transmitiendo, se registran 0 espectadores para mantener la continuidad de la serie temporal
- El worker hace un ciclo inicial al arrancar si está dentro de la ventana horaria

## Cuota de YouTube Data API

La búsqueda de streams activos usa el endpoint `search` (100 unidades por canal) y la consulta de viewers usa `videos.list` en batch (1 unidad por ciclo completo). Con 2 canales cada 10 minutos durante 4 horas se consumen aproximadamente **960 unidades diarias** — dentro del límite gratuito de 10.000 unidades por día.

## Base de datos

```sql
canales               -- configuración de canales monitoreados
historial_audiencia   -- serie temporal de espectadores concurrentes
```

Los índices `(fecha_hora DESC)` y `(canal_id, fecha_hora DESC)` están optimizados para las consultas típicas de visualización de series temporales.
