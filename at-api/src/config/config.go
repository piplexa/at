// Package config отвечает за загрузку и хранение конфигурации приложения.
// Считывает настройки из переменных окружения, включая параметры подключения к БД
// и настройки HTTP сервера.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
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

// ServerConfig содержит настройки HTTP сервера
type ServerConfig struct {
	Port string
}

// Load загружает конфигурацию из переменных окружения.
// Возвращает указатель на структуру Config или ошибку, если обязательные параметры не заданы.
func Load() (*Config, error) {
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
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
		Server: ServerConfig{
			Port: getEnv("API_PORT", "8080"),
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
