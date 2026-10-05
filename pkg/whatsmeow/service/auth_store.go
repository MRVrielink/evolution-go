package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

// The service is copied by value. Keep initialization state on the heap so all
// instances share the same container, schema migration and bounded database pool.
type sharedAuthStore struct {
	mu        sync.Mutex
	container *sqlstore.Container
}

func (s *sharedAuthStore) get(authDB *sql.DB, cfg *config.Config, exPath string) (*sqlstore.Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.container != nil {
		return s.container, nil
	}

	var dbLog waLog.Logger
	if cfg.WaDebug != "" {
		dbLog = waLog.Stdout("Database", cfg.WaDebug, true)
	}
	db, dialect := authDB, "postgres"
	owned := false
	if cfg.PostgresAuthDB == "" {
		dialect = "sqlite"
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000&cache=shared&mode=rwc&_journal_mode=WAL", filepath.ToSlash(filepath.Join(exPath, "dbdata", "main.db")))
		var err error
		db, err = sql.Open(dialect, dsn)
		if err != nil {
			return nil, err
		}
		owned = true
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
		db.SetConnMaxIdleTime(time.Minute)
	} else if db == nil {
		return nil, fmt.Errorf("postgres auth database was not initialized")
	}

	container := sqlstore.NewWithDB(db, dialect, dbLog)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := container.Upgrade(ctx); err != nil {
		if owned {
			_ = db.Close()
		}
		// A transient database failure must remain retryable. Never close the
		// Postgres pool: main owns it, and every instance uses it.
		return nil, fmt.Errorf("failed to upgrade auth store: %w", err)
	}
	s.container = container
	return container, nil
}
