package streaming

import (
	"context"
	"fmt"
	"strings"
	"time"

	"streamtrack/internal/core/domain"
	"streamtrack/internal/core/ports"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YouTubeAdapter struct {
	service *youtube.Service
}

// NewYouTubeAdapter inicializa el adaptador con la API Key pública de Google Cloud
func NewYouTubeAdapter(apiKey string) (ports.StreamingPlatformClient, error) {
	svc, err := youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("no se pudo inicializar el servicio de YouTube API: %w", err)
	}
	return &YouTubeAdapter{service: svc}, nil
}

// ObtenerAudienciaEnVivo resuelve los IDs de directo y recupera los viewers en un esquema optimizado.
func (a *YouTubeAdapter) ObtenerAudienciaEnVivo(ctx context.Context, channels []domain.Canal) (map[string]domain.Metrica, error) {
	resultado := make(map[string]domain.Metrica)

	if len(channels) == 0 {
		return resultado, nil
	}

	// Mapa auxiliar para relacionar de manera rápida: YouTubeChannelID -> ID de nuestro Dominio
	channelMap := make(map[string]string)
	for _, ch := range channels {
		channelMap[ch.YouTubeChannelID] = ch.ID
	}

	// 2. FASE DE DESCUBRIMIENTO (Búsqueda de streams activos)
	// Lamentablemente, el endpoint 'search' de YouTube no permite buscar por múltiples ChannelIDs separados por coma.
	// Por ende, debemos hacer un loop corto (máximo 10 iteraciones, una por canal).
	// Cada llamada a 'search' consume 100 puntos de cuota. Con 10 canales cada 5 min de 8 a 20 hs,
	// consumiríamos unos 144.000 puntos diarios (excede la cuota gratuita de 10.000).
	//
	// 👉 SOLUCIÓN DE OPTIMIZACIÓN DE CUOTA:
	// Para mantenernos dentro de los 10,000 puntos gratuitos, usamos una alternativa que consume solo 1 PUNTO:
	// Usamos el feed de "Live" simulado o asumimos que podemos consultar directamente los videos si el canal
	// expone su stream de forma predecible, o agrupamos por palabras clave.
	// No obstante, para implementar la API oficial estricta de manera segura, el flujo estándar es el siguiente:

	var activeVideoIDs []string
	// Este mapa asociará un VideoID de YouTube al ID de nuestro canal interno
	videoToChannelMap := make(map[string]string)

	for _, ch := range channels {
		// Buscamos si el canal tiene un directo activo en este momento (Costo: 100 unidades si se usa search)
		// NOTA DE PRODUCCIÓN: Si tu cuota es la estándar de 10k, se recomienda usar scraping ligero de la página
		// /live del canal solo para obtener el VideoID, o pedir un aumento de cuota a Google (es gratuito y rápido).
		searchCall := a.service.Search.List([]string{"id"}).
			ChannelId(ch.YouTubeChannelID).
			Type("video").
			EventType("live").
			MaxResults(1)

		searchResponse, err := searchCall.Do()
		if err != nil {
			// Si falla un canal (ej: baneo de cuota temporal), lo salteamos y seguimos con el resto
			continue
		}

		if len(searchResponse.Items) > 0 {
			videoID := searchResponse.Items[0].Id.VideoId
			activeVideoIDs = append(activeVideoIDs, videoID)
			videoToChannelMap[videoID] = ch.ID
		}
	}

	// Si ningún canal está transmitiendo en vivo, terminamos acá ahorrando llamadas
	if len(activeVideoIDs) == 0 {
		return resultado, nil
	}

	// 3. FASE DE RECOLECCIÓN (Consulta de Viewers Concurrentes en BATCH)
	// El endpoint 'Videos.List' SÍ permite pasar múltiples Video IDs separados por coma.
	// Esto consume únicamente 1 PUNTO de cuota por la consulta entera de los 10 canales.
	idsParam := strings.Join(activeVideoIDs, ",")

	videoCall := a.service.Videos.List([]string{"liveStreamingDetails"}).
		Id(idsParam)

	videoResponse, err := videoCall.Do()
	if err != nil {
		return nil, fmt.Errorf("error al consultar detalles de videos en batch: %w", err)
	}

	currentTime := time.Now()

	// 4. Mapear la respuesta de la API a nuestro modelo de Dominio
	for _, videoItem := range videoResponse.Items {
		internalChannelID := videoToChannelMap[videoItem.Id]

		// Si el video tiene detalles de transmisión en vivo activos
		if videoItem.LiveStreamingDetails != nil {
			viewers := int(videoItem.LiveStreamingDetails.ConcurrentViewers)

			resultado[internalChannelID] = domain.Metrica{
				CanalID:      internalChannelID,
				FechaHora:    currentTime,
				Espectadores: viewers,
				LiveVideoID:  videoItem.Id,
			}
		}
	}

	return resultado, nil
}
