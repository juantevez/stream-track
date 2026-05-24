package ports

import (
	"context"
	"streamtrack/internal/core/domain"
)

// StreamingPlatformClient es un Puerto Secundario (Driven).
// Define cómo el negocio interactúa con las APIs externas de streaming.
type StreamingPlatformClient interface {
	ObtenerAudienciaEnVivo(ctx context.Context, channels []domain.Canal) (map[string]domain.Metrica, error)
}
