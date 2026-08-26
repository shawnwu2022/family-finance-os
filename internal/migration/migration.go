package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/shawnwu2022/family-finance-os/db"
	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func Run(ctx context.Context, cfg config.DatabaseConfig) error {
	database, err := sql.Open("pgx", cfg.URL().String())
	if err != nil {
		return fmt.Errorf("open postgres for migrations: %w", err)
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres for migrations: %w", err)
	}

	migrations, err := fs.Sub(dbmigrations.Files, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
