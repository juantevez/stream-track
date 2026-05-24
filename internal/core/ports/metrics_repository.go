package ports

import (
	"context"
	"streamtrack/internal/core/domain"
)

// MetricsRepository es un Puerto Secundario (Driven).
// Define las operaciones de persistencia que el negocio requiere del almacenamiento.
type MetricsRepository interface {
	ObtenerCanalesActivos(ctx context.Context) ([]domain.Canal, error)
	GuardarMetrica(ctx context.Context, metrica domain.Metrica) error
}
