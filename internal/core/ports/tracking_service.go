package ports

import (
	"context"
)

// TrackingService es el Puerto Primario (Driving).
// Define lo que el disparador exterior (Cron/Worker) puede invocar en nuestro core.
type TrackingService interface {
	EjecutarCicloMonitoreo(ctx context.Context) error
}
