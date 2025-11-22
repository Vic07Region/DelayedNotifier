package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgman "DelayedNotifier/internal/config"
	"DelayedNotifier/internal/delivery/handlers"
	"DelayedNotifier/internal/delivery/middleware"
	"DelayedNotifier/internal/migrator"
	"DelayedNotifier/internal/repository/pg"
	"DelayedNotifier/internal/repository/rabbit"
	emailsender "DelayedNotifier/internal/sender/email"
	"DelayedNotifier/internal/service"
	"DelayedNotifier/internal/worker"
	"DelayedNotifier/pkg/rabbitmq"
	"DelayedNotifier/pkg/retry"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/redis"
	"github.com/wb-go/wbf/zlog"
)

// Application основная структура приложения.
type Application struct {
	config    *cfgman.Config
	server    *ginext.Engine
	db        *dbpg.DB
	redis     *redis.Client
	rabbit    *rabbitmq.RabbitClient
	publisher *rabbit.Publisher
	consumer  *worker.Consumer
	service   *service.NotificationService
}

// New создает новое приложение.
func New() (*Application, error) {
	// Загружаем конфигурацию
	cfg, err := cfgman.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Инициализируем логгер
	if err := initLogger(cfg.Logging.Level); err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	app := &Application{
		config: cfg,
	}

	return app, nil
}

// Run запускает приложение в зависимости от команды.
func (a *Application) Run() error {
	if len(os.Args) < 2 {
		a.printUsage()
		return fmt.Errorf("no command specified")
	}

	command := os.Args[1]

	switch command {
	case "runserver":
		return a.runServer()
	case "migrate":
		return a.runMigrate()
	case "health":
		return a.runHealthCheck()
	default:
		a.printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

// printUsage печатает инструкции по использованию.
func (a *Application) printUsage() {
	fmt.Println("DelayedNotifier - система отложенных уведомлений")
	fmt.Println()
	fmt.Println("Доступные команды:")
	fmt.Println("  runserver    - запуск HTTP сервера и воркеров")
	fmt.Println("  migrate up   - накат миграций")
	fmt.Println("  migrate down - откат миграций")
	fmt.Println("  health       - проверка состояния сервисов")
	fmt.Println()
	fmt.Println("Примеры:")
	fmt.Println("  <appname> runserver")
	fmt.Println("  <appname> migrate up")
	fmt.Println("  <appname> migrate down")
	fmt.Println("  <appname> health")
}

// runHealthCheck проверяет состояние всех подключений.
func (a *Application) runHealthCheck() error {
	fmt.Println("Running health check...")

	// Проверяем подключение к базе данных
	if err := a.checkDatabase(); err != nil {
		return fmt.Errorf("database check failed: %w", err)
	}
	fmt.Println("✅ Database connection: OK")

	// Проверяем подключение к Redis
	if err := a.checkRedis(); err != nil {
		return fmt.Errorf("redis check failed: %w", err)
	}
	fmt.Println("✅ Redis connection: OK")

	// Проверяем подключение к RabbitMQ
	if err := a.checkRabbitMQ(); err != nil {
		return fmt.Errorf("rabbitmq check failed: %w", err)
	}
	fmt.Println("✅ RabbitMQ connection: OK")

	fmt.Println("🎉 All health checks passed!")
	return nil
}

// checkDatabase проверяет подключение к базе данных.
func (a *Application) checkDatabase() error {
	cfg, err := cfgman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	opts := &dbpg.Options{
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	}

	db, err := dbpg.New(cfg.Database.DSN, nil, opts)
	if err != nil {
		return err
	}
	defer func(Master *sql.DB) {
		_ = Master.Close()
	}(db.Master)

	return db.Master.Ping()
}

// checkRedis проверяет подключение к Redis.
func (a *Application) checkRedis() error {
	cfg, err := cfgman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := redis.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.Ping(ctx).Err()
}

// checkRabbitMQ проверяет подключение к RabbitMQ.
func (a *Application) checkRabbitMQ() error {
	cfg, err := cfgman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	publishStrategy := retry.Strategy{
		Attempts: cfg.RabbitMQ.PublishRetry.Attempts,
		Delay:    cfg.RabbitMQ.PublishRetry.Delay,
		Backoff:  float64(cfg.RabbitMQ.PublishRetry.Backoff),
	}

	clientConfig := rabbitmq.ClientConfig{
		URL:            cfg.RabbitMQ.URL,
		ConnectionName: cfg.RabbitMQ.ConnectionName + "-health",
		ConnectTimeout: 5 * time.Second,
		Heartbeat:      5 * time.Second,
		PublishRetry:   publishStrategy,
	}

	client, err := rabbitmq.NewClient(clientConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	// Простая проверка - попытка подключения
	return client.Ping()
}

// initLogger инициализирует логгер.
func initLogger(level string) error {
	zlog.Init()

	zerologLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return err
	}
	err = zlog.SetLevel(zerologLevel.String())
	if err != nil {
		return err
	}

	return nil
}

// runServer запускает приложение в режиме сервера.
func (a *Application) runServer() error {
	zlog.Logger.Info().Msg("Starting DelayedNotifier server...")

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := a.initConnections(); err != nil {
		return fmt.Errorf("failed to init connections: %w", err)
	}
	defer a.cleanup()
	if err := a.setupHTTPServer(); err != nil {
		return fmt.Errorf("failed to setup HTTP server: %w", err)
	}
	if err := a.startWorkers(ctx); err != nil {
		return fmt.Errorf("failed to start workers: %w", err)
	}
	zlog.Logger.Info().Str("address", a.config.HTTP.GetConnectionString()).Msg("HTTP server starting")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- a.server.Run(a.config.HTTP.GetConnectionString())
	}()
	zlog.Logger.Info().Msg("HTTP server started, waiting for shutdown signal...")
	select {
	case err := <-serverErr:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-ctx.Done():
		zlog.Logger.Info().Msg("Received shutdown signal")
		return nil
	}
}

// runMigrate запускает приложение в режиме миграций.
func (a *Application) runMigrate() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("migrate command requires direction (up/down)")
	}

	direction := os.Args[2]

	switch direction {
	case "up":
		return a.runMigrateUp()
	case "down":
		return a.runMigrateDown()
	default:
		return fmt.Errorf("unknown migrate direction: %s (use up/down)", direction)
	}
}

// runMigrateUp выполняет накат миграций.
func (a *Application) runMigrateUp() error {
	zlog.Logger.Info().Msg("Running migrations up...")
	db, err := initDatabase(a.config.Database)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer func(Master *sql.DB) {
		_ = Master.Close()
	}(db.Master)
	m, err := migrator.NewMigrator(db.Master, a.config.Migrations.Path)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	if err := m.Up(); err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}

	zlog.Logger.Info().Msg("Migrations applied successfully")
	return nil
}

// runMigrateDown выполняет откат миграций.
func (a *Application) runMigrateDown() error {
	zlog.Logger.Info().Msg("Running migrations down...")

	db, err := initDatabase(a.config.Database)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer func(Master *sql.DB) {
		_ = Master.Close()
	}(db.Master)

	m, err := migrator.NewMigrator(db.Master, a.config.Migrations.Path)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	if err := m.Down(); err != nil {
		return fmt.Errorf("migration down failed: %w", err)
	}

	zlog.Logger.Info().Msg("Migrations rolled back successfully")
	return nil
}

// initConnections инициализирует все подключения.
func (a *Application) initConnections() error {
	var err error

	a.db, err = initDatabase(a.config.Database)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}

	a.redis, err = initRedis(a.config.Redis)
	if err != nil {
		return fmt.Errorf("failed to init redis: %w", err)
	}

	a.rabbit, err = initRabbitMQ(a.config.RabbitMQ)
	if err != nil {
		return fmt.Errorf("failed to init rabbitmq: %w", err)
	}

	if err := a.initServices(); err != nil {
		return fmt.Errorf("failed to init services: %w", err)
	}

	return nil
}

// initDatabase инициализирует подключение к базе данных.
func initDatabase(cfg cfgman.DatabaseConfig) (*dbpg.DB, error) {
	opts := &dbpg.Options{
		MaxOpenConns: cfg.MaxOpenConns,
		MaxIdleConns: cfg.MaxIdleConns,
	}

	db, err := dbpg.New(cfg.DSN, nil, opts)
	if err != nil {
		return nil, err
	}

	if err := db.Master.Ping(); err != nil {
		return nil, err
	}

	zlog.Logger.Info().Msg("Database connection established")
	return db, nil
}

// initRedis инициализирует подключение к Redis.
func initRedis(cfg cfgman.RedisConfig) (*redis.Client, error) {
	client := redis.New(cfg.Addr, cfg.Password, cfg.DB)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	zlog.Logger.Info().Msg("Redis connection established")
	return client, nil
}

// initRabbitMQ инициализирует подключение к RabbitMQ.
func initRabbitMQ(cfg cfgman.RabbitMQConfig) (*rabbitmq.RabbitClient, error) {
	publishStrategy := retry.Strategy{
		Attempts: cfg.PublishRetry.Attempts,
		Delay:    cfg.PublishRetry.Delay,
		Backoff:  float64(cfg.PublishRetry.Backoff),
	}

	clientConfig := rabbitmq.ClientConfig{
		URL:            cfg.URL,
		ConnectionName: cfg.ConnectionName,
		ConnectTimeout: cfg.ConnectTimeout,
		Heartbeat:      cfg.Heartbeat,
		PublishRetry:   publishStrategy,
	}

	client, err := rabbitmq.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}
	err = client.DeclareQueue(cfg.QueueName, cfg.ExchangeName, cfg.QueueName, false, false, false, nil)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Failed to declare queue")
		return nil, err
	}
	zlog.Logger.Info().Msg("RabbitMQ connection established")
	return client, nil
}

// initServices инициализирует сервисы приложения.
func (a *Application) initServices() error {
	pgRepo := pg.NewPostgresRepo(a.db)

	a.publisher = rabbit.NewPublisher(
		a.rabbit,
		a.config.RabbitMQ.ExchangeName,
		"application/json",
		a.config.RabbitMQ.QueueName)

	a.service = service.NewNotificationService(pgRepo, a.publisher, a.redis, 24*time.Hour)

	return nil
}

// setupHTTPServer настраивает HTTP сервер.
func (a *Application) setupHTTPServer() error {
	a.server = ginext.New(gin.ReleaseMode)
	//a.server.Use(middleware.CORSMiddleware())
	a.server.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-IJT"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

	a.server.Use(middleware.RequestIDMiddleware())
	a.server.Use(middleware.LoggingMiddleware())
	a.server.Static("/web", "./web")
	a.server.LoadHTMLGlob("web/*.html")
	h := handlers.NewHandlersSet(a.service)
	a.server.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Главная страница",
		})
	})
	group := a.server.RouterGroup.Group("notify")
	group.POST("/", h.CreateNotificationHandler)
	group.GET("/:id", h.GetNotificationHandler)
	group.DELETE("/:id", h.DeleteNotificationHandler)

	return nil
}

// startWorkers запускает воркеры для обработки сообщений.
func (a *Application) startWorkers(ctx context.Context) error {
	emailSender, err := emailsender.NewSMTPSender(
		a.config.Email.Host,
		a.config.Email.Port,
		a.config.Email.Username,
		a.config.Email.Password,
		a.config.Email.From,
		a.config.Email.UseTLS,
	)
	if err != nil {
		return fmt.Errorf("failed to init email sender: %w", err)
	}

	retryStrategy := retry.Strategy{
		Attempts: a.config.RabbitMQ.ConsumerRetry.Attempts,
		Delay:    a.config.RabbitMQ.ConsumerRetry.Delay,
		Backoff:  float64(a.config.RabbitMQ.ConsumerRetry.Backoff),
	}

	a.consumer, err = worker.NewConsumer(a.service, a.rabbit, emailSender, retryStrategy)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	go a.consumer.Start(ctx, a.config.RabbitMQ.QueueName, 10, 5)

	zlog.Logger.Info().Msg("Workers started successfully")
	return nil
}

// cleanup освобождает ресурсы.
func (a *Application) cleanup() {
	zlog.Logger.Info().Msg("Cleaning up resources...")

	if a.rabbit != nil {
		_ = a.rabbit.Close()
	}

	if a.db != nil {
		_ = a.db.Master.Close()
	}

	zlog.Logger.Info().Msg("Cleanup completed")
}
