package server

import (
	"context"
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/flyteorg/flyte/flyteadmin/pkg/repositories"
	"github.com/flyteorg/flyte/flyteadmin/pkg/repositories/config"
	"github.com/flyteorg/flyte/flyteadmin/pkg/runtime"
	"github.com/flyteorg/flyte/flytestdlib/logger"
)

const adminStartupAdvisoryLock int64 = 0x464c59544541444d
const advisoryLockTimeout = "30s"

func withDB(ctx context.Context, do func(db *gorm.DB) error) error {
	configuration := runtime.NewConfigurationProvider()
	databaseConfig := configuration.ApplicationConfiguration().GetDbConfig()
	logConfig := logger.GetConfig()

	db, err := repositories.GetDB(ctx, databaseConfig, logConfig)
	if err != nil {
		logger.Fatal(ctx, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal(ctx, err)
	}

	defer func(deferCtx context.Context) {
		if err = sqlDB.Close(); err != nil {
			logger.Fatal(deferCtx, err)
		}
	}(ctx)

	if err = sqlDB.Ping(); err != nil {
		return err
	}

	return do(db)
}

func withAdvisoryLock(ctx context.Context, db *gorm.DB, key int64, do func(db *gorm.DB) error) error {
	if db.Dialector.Name() != "postgres" {
		return do(db)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SET lock_timeout = '"+advisoryLockTimeout+"'"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		return fmt.Errorf("could not acquire PostgreSQL advisory lock %d: %w", key, err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
	}()

	lockedDB := db.Session(&gorm.Session{NewDB: true})
	lockedDB.ConnPool = conn
	return do(lockedDB)
}

func migrate(ctx context.Context, db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, config.Migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("database migration failed: %v", err)
	}
	logger.Infof(ctx, "Migration ran successfully")
	return nil
}

// Migrate runs all configured migrations
func Migrate(ctx context.Context) error {
	return withDB(ctx, func(db *gorm.DB) error {
		return withAdvisoryLock(ctx, db, adminStartupAdvisoryLock, func(db *gorm.DB) error {
			return migrate(ctx, db)
		})
	})
}

// MigrateAndSeedProjects serializes startup migrations and project seeding across
// combined-binary replicas.
func MigrateAndSeedProjects(ctx context.Context, projects []config.SeedProject) error {
	return withDB(ctx, func(db *gorm.DB) error {
		return withAdvisoryLock(ctx, db, adminStartupAdvisoryLock, func(db *gorm.DB) error {
			if err := migrate(ctx, db); err != nil {
				return err
			}

			if err := config.SeedProjects(db, projects); err != nil {
				return fmt.Errorf("could not add projects to database with err: %v", err)
			}
			logger.Infof(ctx, "Successfully added projects to database")
			return nil
		})
	})
}

// Rollback rolls back the last migration
func Rollback(ctx context.Context) error {
	return withDB(ctx, func(db *gorm.DB) error {
		m := gormigrate.New(db, gormigrate.DefaultOptions, config.Migrations)
		err := m.RollbackLast()
		if err != nil {
			return fmt.Errorf("could not rollback latest migration: %v", err)
		}
		logger.Infof(ctx, "Rolled back one migration successfully")
		return nil
	})
}

// SeedProjects creates a set of given projects in the DB
func SeedProjects(ctx context.Context, projects []config.SeedProject) error {
	return withDB(ctx, func(db *gorm.DB) error {
		return withAdvisoryLock(ctx, db, adminStartupAdvisoryLock, func(db *gorm.DB) error {
			if err := config.SeedProjects(db, projects); err != nil {
				return fmt.Errorf("could not add projects to database with err: %v", err)
			}
			logger.Infof(ctx, "Successfully added projects to database")
			return nil
		})
	})
}
