package config

import (
	"log"
	"time"

	"github.com/wb-go/wbf/config"
)

// Config основная конфигурация приложения.
type Config struct {
	// HTTP сервер
	HTTP HTTPConfig `config:"http"`

	// База данных
	Database DatabaseConfig `config:"database"`

	// Redis
	Redis RedisConfig `config:"redis"`

	// RabbitMQ
	RabbitMQ RabbitMQConfig `config:"rabbitmq"`

	// Email отправщик
	Email EmailConfig `config:"email"`

	// Миграции
	Migrations MigrationConfig `config:"migrations"`

	// Логирование
	Logging LoggingConfig `config:"logging"`
}

// HTTPConfig конфигурация HTTP сервера.
type HTTPConfig struct {
	Host string `config:"host" default:"localhost"`
	Port string `config:"port" default:"8080"`
}

// DatabaseConfig конфигурация базы данных.
type DatabaseConfig struct {
	DSN          string `config:"dsn"`
	MaxOpenConns int    `config:"max_open_conns" default:"10"`
	MaxIdleConns int    `config:"max_idle_conns" default:"5"`
}

// RedisConfig конфигурация Redis.
type RedisConfig struct {
	Addr     string `config:"addr" default:"localhost:6379"`
	Password string `config:"password"`
	DB       int    `config:"db" default:"0"`
}

// RabbitMQConfig конфигурация RabbitMQ.
type RabbitMQConfig struct {
	URL            string              `config:"url"`
	ConnectionName string              `config:"connectionname" default:"delayednotifier"`
	ConnectTimeout time.Duration       `config:"connecttimeout" default:"5s"`
	Heartbeat      time.Duration       `config:"heartbeat" default:"5s"`
	ExchangeName   string              `config:"exchangename" default:"DelayedNotifier"`
	QueueName      string              `config:"queuename" default:"notification"`
	RoutingKey     string              `config:"routingkey" default:"notification"`
	PublishRetry   RabbitMqRetryConfig `config:"publishretry"`
	ConsumerRetry  RabbitMqRetryConfig `config:"consumerretry"`
}

type RabbitMqRetryConfig struct {
	Attempts int           `config:"attempts" default:"5"`
	Delay    time.Duration `config:"delay" default:"1s"`
	Backoff  int           `config:"backoff" default:"2"`
}

// EmailConfig конфигурация email отправщика.
type EmailConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Username string `config:"username"`
	Password string `config:"password"`
	From     string `config:"from"`
	UseTLS   bool   `config:"usetls" default:"false"`
}

// MigrationConfig конфигурация миграций.
type MigrationConfig struct {
	Path string `config:"path" default:"./migrations"`
}

// LoggingConfig конфигурация логирования.
type LoggingConfig struct {
	Level string `config:"level" default:"info"`
}

// LoadConfig загружает конфигурацию из переменных окружения.
func LoadConfig() (*Config, error) {
	wbfCfg := config.New()
	if err := wbfCfg.LoadEnvFiles(".env"); err != nil {
		log.Printf("failed to load env vars: %v", err)
	}
	// Включаем переменные окружения с префиксом
	wbfCfg.EnableEnv("DELAYED_NOTIFIER")

	// Устанавливаем значения по умолчанию
	// run server config
	wbfCfg.SetDefault("http.host", "localhost")
	wbfCfg.SetDefault("http.port", "8080")
	// database connection config
	wbfCfg.SetDefault("database.dsn", "postgres://postgres:postgres@localhost:5432/notifier?sslmode=disable")
	wbfCfg.SetDefault("database.max_open_conns", 10)
	wbfCfg.SetDefault("database.max_idle_conns", 5)
	// redis connection config
	wbfCfg.SetDefault("redis.addr", "localhost:6379")
	wbfCfg.SetDefault("redis.password", "")
	wbfCfg.SetDefault("redis.db", 0)
	// rabbitmq connection config
	wbfCfg.SetDefault("rabbitmq.connectionname", "delayednotifier")
	wbfCfg.SetDefault("rabbitmq.url", "amqp://guest:guest@localhost:5672/")
	wbfCfg.SetDefault("rabbitmq.connecttimeout", "5s")
	wbfCfg.SetDefault("rabbitmq.heartbeat", "5s")
	wbfCfg.SetDefault("rabbitmq.exchangename", "DelayedNotifier")
	wbfCfg.SetDefault("rabbitmq.queuename", "notification")
	wbfCfg.SetDefault("rabbitmq.routingkey", "notification1")
	// retry strategy
	wbfCfg.SetDefault("rabbitmq.publishretry.attempts", 3)
	wbfCfg.SetDefault("rabbitmq.publishretry.delay", "3s")
	wbfCfg.SetDefault("rabbitmq.publishretry.backoff", 3)
	wbfCfg.SetDefault("rabbitmq.consumerretry.attempts", 3)
	wbfCfg.SetDefault("rabbitmq.consumerretry.delay", "3s")
	wbfCfg.SetDefault("rabbitmq.consumerretry.backoff", 3)
	// email smtp connection config
	wbfCfg.SetDefault("email.host", "localhost")
	wbfCfg.SetDefault("email.port", 445)
	wbfCfg.SetDefault("email.username", "developer")
	wbfCfg.SetDefault("email.password", "")
	wbfCfg.SetDefault("email.from", "developer")
	wbfCfg.SetDefault("email.usetls", false)
	// other config
	wbfCfg.SetDefault("migrations.path", "./migrations")
	wbfCfg.SetDefault("logging.level", "info")

	// Парсим флаги
	if err := wbfCfg.ParseFlags(); err != nil {
		return nil, err
	}

	// Создаем структуру конфигурации и загружаем данные
	appConfig := &Config{}
	if err := wbfCfg.Unmarshal(appConfig); err != nil {
		return nil, err
	}
	return appConfig, nil
}

// GetConnectionString формирует строку подключения для HTTP.
func (c *HTTPConfig) GetConnectionString() string {
	return c.Host + ":" + c.Port
}
