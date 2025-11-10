// Package config отвечает за загрузку и хранение конфигурации приложения.
// Считывает настройки из переменных окружения, включая параметры подключения к БД,
// интервал опроса заданий, размер батча и идентификатор worker'а для распределённого развёртывания.
// Поддерживает загрузку переменных из .env файла (если он существует).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит всю конфигурацию приложения worker'а
type Config struct {
	Database DatabaseConfig
	Worker   WorkerConfig
}

// DatabaseConfig содержит параметры подключения к PostgreSQL
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// WorkerConfig содержит настройки worker'а для опроса и обработки заданий
type WorkerConfig struct {
	WorkerID        string        // Уникальный идентификатор worker'а для логирования
	PollingInterval time.Duration // Интервал опроса БД для новых заданий
	BatchSize       int           // Количество заданий, извлекаемых за один запрос
	CleanerInterval time.Duration // Интервал запуска cleaner для поиска зависших заданий
	StuckTimeout    time.Duration // Время, после которого задание считается зависшим
}

// Load загружает конфигурацию из переменных окружения.
// Сначала пытается загрузить .env файл (если существует),
// затем читает переменные окружения.
// Возвращает указатель на структуру Config или ошибку, если обязательные параметры не заданы.
func Load() (*Config, error) {
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	pollingInterval, err := strconv.Atoi(getEnv("WORKER_POLLING_INTERVAL", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_POLLING_INTERVAL: %w", err)
	}

	batchSize, err := strconv.Atoi(getEnv("WORKER_BATCH_SIZE", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_BATCH_SIZE: %w", err)
	}

	cleanerInterval, err := strconv.Atoi(getEnv("WORKER_CLEANER_INTERVAL", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_CLEANER_INTERVAL: %w", err)
	}

	stuckTimeout, err := strconv.Atoi(getEnv("WORKER_STUCK_TIMEOUT", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_STUCK_TIMEOUT: %w", err)
	}

	// Определяем WORKER_ID: приоритет ENV переменной, затем hostname, затем дефолт
	workerID := getEnv("WORKER_ID", "")
	if workerID == "" {
		// Если WORKER_ID не задан, используем hostname (для docker-compose scale)
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			workerID = hostname
		} else {
			workerID = "worker-1" // Fallback значение
		}
	}

	config := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "at_scheduler"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Worker: WorkerConfig{
			WorkerID:        workerID,
			PollingInterval: time.Duration(pollingInterval) * time.Second,
			BatchSize:       batchSize,
			CleanerInterval: time.Duration(cleanerInterval) * time.Minute,
			StuckTimeout:    time.Duration(stuckTimeout) * time.Minute,
		},
	}

	return config, nil
}

// DSN формирует строку подключения к PostgreSQL (Data Source Name).
// Возвращает строку в формате: "host=... port=... user=... password=... dbname=... sslmode=..."
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию.
// Параметры:
//   - key: имя переменной окружения
//   - defaultValue: значение, которое будет возвращено, если переменная не установлена
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
