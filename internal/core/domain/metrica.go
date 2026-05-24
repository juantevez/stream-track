package domain

import "time"

type Metrica struct {
	ID           int64
	CanalID      string
	FechaHora    time.Time
	Espectadores int
	LiveVideoID  string
}
