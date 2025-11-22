package migrator

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// простая обертка над golang-migrator для удобства использования.

// Migrator основная структура.
type Migrator struct {
	migrate *migrate.Migrate
}

func NewMigrator(db *sql.DB, migrationsDir string) (*Migrator, error) {
	if db == nil {
		return nil, errors.New("database connection is nil")
	}

	if migrationsDir == "" {
		return nil, errors.New("migrations directory is empty")
	}

	info, err := os.Stat(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access migrations path %q: %w", migrationsDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("migrations path %q is not a directory", migrationsDir)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres driver: %w\", err", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		normalizePath(migrationsDir),
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{m}, nil
}

// Up накатываем все непримененные миграции.
func (m *Migrator) Up() error {
	err := m.migrate.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

// Down откатываем последнюю примененную миграцию.
func (m *Migrator) Down() error {
	err := m.migrate.Down()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

// Version возвращает текущую версию.
func (m *Migrator) Version() (uint, error) {
	ver, dirty, err := m.migrate.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, nil
		}
		return 0, err
	}
	if dirty {
		return ver, fmt.Errorf("database is dirty at version %d (migration failed midway)", ver)
	}
	return ver, nil
}

// MigrateTo применяет миграции или откатывает их до указанной версии.
func (m *Migrator) MigrateTo(version uint) error {
	return m.migrate.Migrate(version)
}

// Close освобождаем ресурсы.
func (m *Migrator) Close() error {
	if m.migrate != nil {
		serr, derr := m.migrate.Close()
		return fmt.Errorf("close error: source error: %w , database error: %w", serr, derr)
	}
	return nil
}
