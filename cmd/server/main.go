package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"streamtrack/internal/adapters/storage"
	"streamtrack/internal/adapters/streaming"
	"streamtrack/internal/core/services"

	_ "github.com/lib/pq" // Driver puro de Postgres para database/sql
)

func main() {
	log.Println("[Main] Iniciando servicio StreamTrack (PoC)...")

	// 1. Leer configuraciones desde las variables de entorno (Inyectadas por Docker)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		// Fallback local por si lo corrés fuera de Docker en etapa de desarrollo
		dsn = "postgres://track_user:track_password_123@localhost:5432/streamtrack?sslmode=disable"
	}

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" || apiKey == "ACA_VA_TU_API_KEY_DE_GOOGLE" {
		log.Println("[Main][Advertencia] YOUTUBE_API_KEY no configurada o es por defecto. Las llamadas a la API podrían fallar.")
	}

	// 2. Inicializar Infraestructura: Conexión segura a la Base de Datos
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[Main][Error Crítico] No se pudo inicializar el driver de Postgres: %v", err)
	}
	defer func() {
		log.Println("[Main] Cerrando conexiones de base de datos...")
		db.Close()
	}()

	// Verificar la conectividad real con la base de datos con reintentos (Resiliencia Docker)
	var dbConnected bool
	maxRetries := 5

	for i := 1; i <= maxRetries; i++ {
		log.Printf("[Main] Verificando conexión a la base de datos (Intento %d de %d)...", i, maxRetries)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			dbConnected = true
			break
		}

		log.Printf("[Main][Advertencia] Base de datos no disponible aún, esperando para reintentar... Módulo de red dice: %v", err)
		time.Sleep(2 * time.Second) // Esperar 2 segundos antes del próximo intento
	}

	if !dbConnected {
		log.Fatalf("[Main][Error Crítico] Base de datos inaccesible tras %d intentos. Abortando.", maxRetries)
	}

	log.Println("[Main] Conexión a Postgres establecida exitosamente.")

	// 3. Ensamblado de Arquitectura Hexagonal (Inyección de Dependencias manual)
	repo := storage.NewPostgresRepository(db)
	ytClient, err := streaming.NewYouTubeAdapter(apiKey)
	if err != nil {
		log.Fatalf("[Main][Error Crítico] No se pudo inicializar el cliente de YouTube: %v", err)
	}
	trackingService := services.NewTrackingService(repo, ytClient)

	log.Println("[Main] Capas de arquitectura inyectadas correctamente.")
	log.Println("[Main] Configuración PoC: 2 canales, intervalo 10 min, ventana de 4hs (09:00 a 13:00 ART).")

	// 4. Configurar Ticker de Control Temporal (10 Minutos para la PoC)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Capturar señales del Sistema Operativo para un apagado limpio (Graceful Shutdown)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Forzar la locación a la hora oficial de Argentina de manera explícita
	locationART, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		log.Printf("[Main][Advertencia] No se encontró tzdata local, usando offset fijo UTC-3: %v", err)
		locationART = time.FixedZone("ART", -3*3600)
	}

	// Ejecutar un ciclo inicial asíncrono al arrancar para validar que todo funcione sin esperar 10 min
	go func() {
		initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer initCancel()
		now := time.Now().In(locationART)

		// Validamos si el arranque ocurre dentro de la ventana de la PoC
		if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday && now.Hour() >= 9 && now.Hour() < 13 {
			log.Println("[Main] Ejecutando verificación de ciclo inicial inmediato...")
			if err := trackingService.EjecutarCicloMonitoreo(initCtx); err != nil {
				log.Printf("[Main][Error] Falló la verificación inicial: %v", err)
			}
		} else {
			log.Println("[Main] Verificación inicial omitida: Fuera de la ventana operativa de la PoC.")
		}
	}()

	// 5. Bucle Principal de Eventos (Event Loop)
	for {
		select {
		case <-ticker.C:
			now := time.Now().In(locationART)

			// Regla de Negocio Temporal PoC: Lunes a Viernes
			if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday {
				// Ventana de 4 horas: de 09:00:00 a 12:59:59 hs
				if now.Hour() >= 9 && now.Hour() < 13 {
					// Generamos un contexto con timeout de 45 segundos para que el ciclo no quede colgado infinitamente
					cycleCtx, cycleCancel := context.WithTimeout(context.Background(), 45*time.Second)

					if err := trackingService.EjecutarCicloMonitoreo(cycleCtx); err != nil {
						log.Printf("[Main][Error] Error en ciclo de las %02d:%02d -> %v", now.Hour(), now.Minute(), err)
					}
					cycleCancel()
				}
			}

		case sig := <-stopChan:
			log.Printf("[Main] Señal de parada recibida (%v). Finalizando Worker de forma limpia...", sig)
			return
		}
	}
}
