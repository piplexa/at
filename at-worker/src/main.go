// Главный файл приложения at-worker.
// Точка входа в worker-сервис для обработки запланированных заданий.
// Инициализирует конфигурацию, подключение к БД, запускает Worker и Cleaner,
// обеспечивает graceful shutdown при получении сигналов SIGINT/SIGTERM.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"at-worker/config"
	"at-worker/db"
	"at-worker/worker"

	"github.com/joho/godotenv"
)

func main() {
	log.Println("=== AT Worker Starting ===")

	// Пытаемся загрузить .env файл, если он существует
	// Если файла нет, используем переменные окружения системы
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	} else {
		log.Println("Loaded configuration from .env file")
	}

	// Загрузка конфигурации из переменных окружения
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Worker ID: %s", cfg.Worker.WorkerID)
	log.Printf("Polling interval: %v", cfg.Worker.PollingInterval)
	log.Printf("Batch size: %d", cfg.Worker.BatchSize)
	log.Printf("Cleaner interval: %v", cfg.Worker.CleanerInterval)
	log.Printf("Stuck timeout: %v", cfg.Worker.StuckTimeout)

	// Подключение к базе данных PostgreSQL
	database, err := db.NewPostgresDB(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Successfully connected to database")

	// Создание контекста с возможностью отмены для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создание и запуск Worker
	w := worker.NewWorker(
		database,
		cfg.Worker.WorkerID,
		cfg.Worker.PollingInterval,
		cfg.Worker.BatchSize,
	)

	// Создание и запуск Cleaner
	c := worker.NewCleaner(
		database,
		cfg.Worker.CleanerInterval,
		cfg.Worker.StuckTimeout,
	)

	// Запуск Worker и Cleaner в отдельных goroutines
	go w.Start(ctx)
	go c.Start(ctx)

	log.Println("Worker and Cleaner started successfully")

	// Ожидание сигнала для graceful shutdown
	// Поддерживаемые сигналы: SIGINT (Ctrl+C), SIGTERM (docker stop)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Блокируемся до получения сигнала
	sig := <-sigChan
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	// Отменяем контекст, что приведет к остановке Worker и Cleaner
	cancel()

	// Даем время на завершение текущих задач (можно добавить sync.WaitGroup для более точного контроля)
	// time.Sleep(5 * time.Second)

	log.Println("=== AT Worker Stopped ===")
}
