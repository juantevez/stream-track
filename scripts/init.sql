-- Rol de solo lectura para PostgREST (acceso anónimo)
CREATE ROLE web_anon NOLOGIN;
GRANT USAGE ON SCHEMA public TO web_anon;

-- Tabla de Configuración de Canales
CREATE TABLE IF NOT EXISTS canales (
    id VARCHAR(50) PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    youtube_channel_id VARCHAR(100) NOT NULL UNIQUE,
    activo BOOLEAN DEFAULT TRUE
);

-- Tabla de Series Temporales de Audiencia
CREATE TABLE IF NOT EXISTS historial_audiencia (
    registro_id SERIAL PRIMARY KEY,
    canal_id VARCHAR(50) REFERENCES canales(id),
    fecha_hora TIMESTAMP WITH TIME ZONE NOT NULL,
    espectadores INT NOT NULL,
    live_video_id VARCHAR(50)
);

-- Índices críticos para que los gráficos futuros vuelen al consultar por tiempo
CREATE INDEX IF NOT EXISTS idx_historial_fecha ON historial_audiencia (fecha_hora DESC);
CREATE INDEX IF NOT EXISTS idx_historial_canal_fecha ON historial_audiencia (canal_id, fecha_hora DESC);

-- Permisos de lectura para PostgREST
GRANT SELECT ON canales TO web_anon;
GRANT SELECT ON historial_audiencia TO web_anon;

-- Precarga de datos para la PoC (Ejemplo con Luzu y Olga)
INSERT INTO canales (id, nombre, youtube_channel_id, activo) VALUES
('luzu-tv', 'Luzu TV', 'UC7nLdOemZscI89gqM04Z_bA', true),
('olga-envivo', 'Olga En Vivo', 'UCFsc2Y4_KID6x47n89u7s-Q', true)
ON CONFLICT (id) DO NOTHING;
