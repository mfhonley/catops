package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catops/internal/k8s"
	"catops/internal/logger"
)

const (
	// Version информация
	Version = "0.2.4"
)

func main() {
	// Banner
	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║   CatOps Kubernetes Connector v" + Version + "          ║")
	fmt.Println("╚═══════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("DEBUG: Starting configuration load...")

	// Получаем конфигурацию из environment variables
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("ERROR: Failed to load configuration: %v\n", err)
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}
	fmt.Println("DEBUG: Configuration loaded successfully")

	fmt.Println("DEBUG: Starting configuration validation...")

	// Валидация конфигурации
	if err := config.Validate(); err != nil {
		fmt.Printf("ERROR: Invalid configuration: %v\n", err)
		log.Fatalf("❌ Invalid configuration: %v", err)
	}
	fmt.Println("DEBUG: Configuration validated successfully")

	logger.Info("📋 Configuration loaded successfully")
	logger.Info("   Backend URL: %s", config.BackendURL)
	logger.Info("   Node Name: %s", config.NodeName)
	logger.Info("   Namespace: %s", config.Namespace)
	logger.Info("   Collection Interval: %ds", config.CollectionInterval)
	fmt.Println()

	// Создаем Kubernetes client
	logger.Info("🔌 Connecting to Kubernetes API...")
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("❌ Failed to create Kubernetes client: %v", err)
	}
	logger.Info("✅ Connected to Kubernetes API")

	// Проверяем доступность Kubernetes API
	if err := k8sClient.HealthCheck(context.Background()); err != nil {
		log.Fatalf("❌ Kubernetes API health check failed: %v", err)
	}
	logger.Info("✅ Kubernetes API is healthy")

	// Создаем collector
	collector := k8s.NewCollector(k8sClient, config, Version)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("\n🛑 Received shutdown signal, gracefully stopping...")
		cancel()
	}()

	logger.Info("🚀 Starting metrics collection...")
	fmt.Println()

	// Основной цикл сбора метрик
	ticker := time.NewTicker(time.Duration(config.CollectionInterval) * time.Second)
	defer ticker.Stop()

	// Первый сбор сразу при старте
	if err := collector.CollectAndSend(ctx); err != nil {
		logger.Error("Failed to collect metrics: %v", err)
	}

	// Затем по расписанию
	for {
		select {
		case <-ctx.Done():
			logger.Info("👋 Shutdown complete")
			return
		case <-ticker.C:
			if err := collector.CollectAndSend(ctx); err != nil {
				logger.Error("Failed to collect metrics: %v", err)
			}
		}
	}
}

// Config конфигурация приложения
type Config struct {
	// Backend API
	BackendURL string
	AuthToken  string

	// Kubernetes
	NodeName  string
	Namespace string

	// Collection settings
	CollectionInterval int // seconds

	// Prometheus (optional)
	PrometheusURL string
}

// Validate проверяет конфигурацию
func (c *Config) Validate() error {
	if c.BackendURL == "" {
		return fmt.Errorf("CATOPS_BACKEND_URL is required")
	}
	if c.AuthToken == "" {
		return fmt.Errorf("CATOPS_AUTH_TOKEN is required")
	}
	if c.NodeName == "" {
		return fmt.Errorf("NODE_NAME is required (should be set by Kubernetes)")
	}
	if c.CollectionInterval < 10 {
		return fmt.Errorf("collection interval must be at least 10 seconds")
	}
	return nil
}

// Interface methods для Collector
func (c *Config) GetBackendURL() string    { return c.BackendURL }
func (c *Config) GetAuthToken() string     { return c.AuthToken }
func (c *Config) GetNodeName() string      { return c.NodeName }
func (c *Config) GetNamespace() string     { return c.Namespace }
func (c *Config) GetPrometheusURL() string { return c.PrometheusURL }

// loadConfig загружает конфигурацию из environment variables
func loadConfig() (*Config, error) {
	config := &Config{
		BackendURL:         getEnv("CATOPS_BACKEND_URL", "https://api.catops.io"),
		AuthToken:          getEnv("CATOPS_AUTH_TOKEN", ""),
		NodeName:           getEnv("NODE_NAME", ""),
		Namespace:          getEnv("NAMESPACE", "default"),
		CollectionInterval: getEnvInt("COLLECTION_INTERVAL", 60),
		PrometheusURL:      getEnv("PROMETHEUS_URL", ""), // Optional
	}

	return config, nil
}

// getEnv получает environment variable с default значением
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt получает environment variable как int с default значением
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
