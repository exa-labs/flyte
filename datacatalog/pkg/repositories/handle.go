package repositories

import (
	"context"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/flyteorg/flyte/datacatalog/pkg/repositories/config"
	"github.com/flyteorg/flyte/datacatalog/pkg/repositories/models"
	"github.com/flyteorg/flyte/flytestdlib/database"
	"github.com/flyteorg/flyte/flytestdlib/logger"
	"github.com/flyteorg/flyte/flytestdlib/promutils"
)

const datacatalogStartupAdvisoryLock int64 = 0x464c595445434154
const advisoryLockTimeout = "5min"

type DBHandle struct {
	db *gorm.DB
}

func NewDBHandle(ctx context.Context, dbConfigValues database.DbConfig, catalogScope promutils.Scope) (*DBHandle, error) {
	var gormDb *gorm.DB
	var err error

	switch {
	case !dbConfigValues.SQLite.IsEmpty():
		gormDb, err = gorm.Open(sqlite.Open(dbConfigValues.SQLite.File))
	case !dbConfigValues.Postgres.IsEmpty():
		gormDb, err = config.OpenDbConnection(ctx, config.NewPostgresConfigProvider(dbConfigValues, catalogScope.NewSubScope(config.Postgres)))
	default:
		return nil, fmt.Errorf("unrecognized database config, %v. Supported only postgres and sqlite", dbConfigValues)
	}

	if err != nil {
		return nil, err
	}

	out := &DBHandle{
		db: gormDb,
	}

	return out, nil
}

func (h *DBHandle) CreateDB(dbName string) error {
	type DatabaseResult struct {
		Exists bool
	}
	var checkExists DatabaseResult
	result := h.db.Raw("SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = ?)", dbName).Scan(&checkExists)
	if result.Error != nil {
		return result.Error
	}

	// create db if it does not exist
	if !checkExists.Exists {
		logger.Infof(context.TODO(), "Creating Database %v since it does not exist", dbName)

		// NOTE: golang sql drivers do not support parameter injection for CREATE calls
		createDBStatement := fmt.Sprintf("CREATE DATABASE %s", dbName)
		result = h.db.Exec(createDBStatement)

		if result.Error != nil {
			if !database.IsPgErrorWithCode(result.Error, database.PqDbAlreadyExistsCode) && !database.IsPgErrorWithCode(result.Error, database.PgDuplicatedKey) {
				return result.Error
			}
			logger.Infof(context.TODO(), "Not creating database %s, already exists", dbName)
		}
	}

	return nil
}

func (h *DBHandle) Migrate(ctx context.Context) error {
	if h.db.Dialector.Name() != "postgres" {
		return h.migrate()
	}

	return withAdvisoryLock(ctx, h.db, datacatalogStartupAdvisoryLock, func(db *gorm.DB) error {
		return (&DBHandle{db: db}).migrate()
	})
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
		return fmt.Errorf("could not acquire PostgreSQL advisory lock %d: another replica is holding the migration lock; this is expected to be transient: %w", key, err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
	}()

	lockedDB := db.Session(&gorm.Session{NewDB: true})
	lockedDB.ConnPool = conn
	return do(lockedDB)
}

func (h *DBHandle) migrate() error {
	if err := h.db.AutoMigrate(&models.Dataset{}); err != nil {
		return err
	}

	if err := h.db.Debug().AutoMigrate(&models.Artifact{}); err != nil {
		return err
	}

	if err := h.db.AutoMigrate(&models.ArtifactData{}); err != nil {
		return err
	}

	if err := h.db.AutoMigrate(&models.Tag{}); err != nil {
		return err
	}

	if err := h.db.AutoMigrate(&models.PartitionKey{}); err != nil {
		return err
	}

	if err := h.db.AutoMigrate(&models.Partition{}); err != nil {
		return err
	}

	if err := h.db.AutoMigrate(&models.Reservation{}); err != nil {
		return err
	}

	return nil
}
