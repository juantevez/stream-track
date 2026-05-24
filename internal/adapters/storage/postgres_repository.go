package storage

import (
	"context"
	"database/sql"
	"fmt"
	"streamtrack/internal/core/domain"
	"streamtrack/internal/core/ports"
)

// PostgresRepository implementa la interfaz ports.MetricsRepository.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository es el constructor que inyecta la conexión a la base de datos.
// Retorna la interfaz del puerto secundario.
func NewPostgresRepository(db *sql.DB) ports.MetricsRepository {
	return &PostgresRepository{
		db: db,
	}
}

// ObtenerCanalesActivos busca en la base de datos todos los canales configurados con activo = true.
func (r *PostgresRepository) ObtenerCanalesActivos(ctx context.Context) ([]domain.Canal, error) {
	// Query explícita apuntando a las columnas del modelo lógico
	query := `
		SELECT id, nombre, youtube_channel_id, activo 
		FROM canales 
		WHERE activo = true;
	`

	// Ejecutamos usando el contexto para soportar cancelaciones o timeouts heredados
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al ejecutar query de canales activos: %w", err)
	}
	defer rows.Close()

	var canales []domain.Canal

	// Iteramos sobre el cursor de resultados
	for rows.Next() {
		var c domain.Canal
		err := rows.Scan(
			&c.ID,
			&c.Nombre,
			&c.YouTubeChannelID,
			&c.Activo,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila de canal: %w", err)
		}
		canales = append(canales, c)
	}

	// Validamos si el bucle terminó por un error oculto en el cursor
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de filas de canales: %w", err)
	}

	return canales, nil
}

// GuardarMetrica inserta un nuevo registro inmutable en el historial de audiencias.
func (r *PostgresRepository) GuardarMetrica(ctx context.Context, m domain.Metrica) error {
	query := `
		INSERT INTO historial_audiencia (canal_id, fecha_hora, espectadores, live_video_id) 
		VALUES ($1, $2, $3, $4);
	`

	// ExecContext se usa para operaciones de escritura (INSERT, UPDATE, DELETE) que no devuelven filas
	_, err := r.db.ExecContext(
		ctx,
		query,
		m.CanalID,      // $1
		m.FechaHora,    // $2 - Go maneja nativamente la conversión de time.Time a TIMESTAMP de Postgres
		m.Espectadores, // $3
		m.LiveVideoID,  // $4
	)

	if err != nil {
		return fmt.Errorf("error al insertar métrica en la base de datos: %w", err)
	}

	return nil
}
