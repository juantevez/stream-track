package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"streamtrack/internal/core/domain"
	"streamtrack/internal/core/ports"
)

// trackingService es la estructura privada que implementa la interfaz ports.TrackingService.
// Mantiene las referencias a los puertos secundarios (driven) mediante inyección de dependencias.
type trackingService struct {
	repo           ports.MetricsRepository
	platformClient ports.StreamingPlatformClient
}

// NewTrackingService actúa como constructor (Provider) del caso de uso.
// Devuelve la interfaz del puerto primario para mantener el desacoplamiento.
func NewTrackingService(r ports.MetricsRepository, p ports.StreamingPlatformClient) ports.TrackingService {
	return &trackingService{
		repo:           r,
		platformClient: p,
	}
}

// EjecutarCicloMonitoreo orquesta el flujo funcional completo de recolección de métricas.
// Se ejecuta cada 5 minutos gatillado por el Driver externo (Cron/Ticker).
func (s *trackingService) EjecutarCicloMonitoreo(ctx context.Context) error {
	// 1. Obtener la hora actual en la zona horaria del negocio (Argentina UTC-3)
	// Nota: El Ticker principal ya valida la ventana horaria, pero capturamos el timestamp exacto del ciclo.
	currentTime := time.Now().In(time.FixedZone("ART", -3*3600))

	log.Printf("[TrackingService] Iniciando ciclo de monitoreo continuo - %s", currentTime.Format("2006-01-02 15:04:05"))

	// 2. Fase de Descubrimiento: Buscar los canales que están configurados y activos en el sistema
	canales, err := s.repo.ObtenerCanalesActivos(ctx)
	if err != nil {
		return fmt.Errorf("error al recuperar canales activos desde el repositorio: %w", err)
	}

	// Si no hay canales configurados, salimos prematuramente de manera exitosa
	if len(canales) == 0 {
		log.Println("[TrackingService] No se encontraron canales activos para monitorear en este ciclo.")
		return nil
	}

	// 3. Fase de Recolección: Consultar las métricas de audiencia en la plataforma externa (Batch)
	// Le pasamos el slice de canales para que el adaptador maneje la resolución de IDs dinámicos si es necesario.
	metricasMap, err := s.platformClient.ObtenerAudienciaEnVivo(ctx, canales)
	if err != nil {
		// Logueamos el error de la API pero no interrumpimos el flujo completo.
		// Intentamos procesar lo que se pueda o asegurar que se guarden en 0 si la API falló por completo.
		log.Printf("[TrackingService][Error] Falla parcial o total al consultar la plataforma externa: %v", err)
	}

	// 4. Fase de Persistencia: Consolidar y registrar cada muestra en el historial inmutable
	for _, canal := range canales {
		var metricaAPersistir domain.Metrica

		// Verificamos si el adaptador externo nos trajo métricas válidas para este canal específico
		metricaExterna, existe := metricasMap[canal.ID]

		if existe {
			// Si el canal está transmitiendo, preparamos el registro con los datos reales obtenidos
			metricaAPersistir = domain.Metrica{
				CanalID:      canal.ID,
				FechaHora:    currentTime, // Normalizamos todas las métricas del ciclo con el mismo timestamp
				Espectadores: metricaExterna.Espectadores,
				LiveVideoID:  metricaExterna.LiveVideoID,
			}
			log.Printf("[TrackingService] Canal [%s] EN VIVO - Viewers: %d - Video ID: %s",
				canal.Nombre, metricaAPersistir.Espectadores, metricaAPersistir.LiveVideoID)
		} else {
			// Caso de borde / Regla de negocio: Si el canal no está transmitiendo, guardamos 0 espectadores
			// Esto evita baches en las series temporales de la base de datos relacional.
			metricaAPersistir = domain.Metrica{
				CanalID:      canal.ID,
				FechaHora:    currentTime,
				Espectadores: 0,
				LiveVideoID:  "", // Sin ID de video ya que está inactivo
			}
			log.Printf("[TrackingService] Canal [%s] INACTIVO - Registrando 0 espectadores", canal.Nombre)
		}

		// Impactamos la métrica en la base de datos relacional a través del puerto secundario
		if err := s.repo.GuardarMetrica(ctx, metricaAPersistir); err != nil {
			// Si un canal falla al guardar, logueamos el error específico y continuamos con los demás
			log.Printf("[TrackingService][Error] No se pudo persistir la métrica para el canal %s (%s): %v",
				canal.Nombre, canal.ID, err)
			continue
		}
	}

	log.Println("[TrackingService] Ciclo de monitoreo finalizado correctamente.")
	return nil
}
