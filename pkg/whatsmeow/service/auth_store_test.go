package whatsmeow_service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
)

func testSQLiteStore(t *testing.T) (*sharedAuthStore, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dbdata"), 0700); err != nil {
		t.Fatal(err)
	}
	return &sharedAuthStore{}, dir
}

func TestAuthStoreConcurrentReconnectsPreserveSession(t *testing.T) {
	s, dir := testSQLiteStore(t)
	cfg := &config.Config{}
	container, err := s.get(nil, cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	verifyConcurrentSessionStore(t, s, container, nil, cfg, dir)
}

func verifyConcurrentSessionStore(t *testing.T, s *sharedAuthStore, first *sqlstore.Container, db *sql.DB, cfg *config.Config, dir string) {
	t.Helper()
	jid := types.NewJID("15551234567", types.DefaultUserServer)
	device := first.NewDevice()
	device.ID = &jid
	device.Account = &waAdv.ADVSignedDeviceIdentity{Details: []byte{8, 1}, AccountSignature: make([]byte, 64), AccountSignatureKey: make([]byte, 32), DeviceSignature: make([]byte, 64)}
	if err := device.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = device.Delete(context.Background()) })
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			container, err := s.get(db, cfg, dir)
			if err != nil {
				t.Error(err)
				return
			}
			if container != first {
				t.Error("reconnect created another auth container")
				return
			}
			loaded, err := container.GetDevice(context.Background(), jid)
			if err != nil {
				t.Error(err)
				return
			}
			if loaded == nil || *loaded.IdentityKey.Pub != *device.IdentityKey.Pub {
				t.Error("reconnect lost the paired device identity")
			}
		}()
	}
	wg.Wait()
}

func TestAuthStoreRetriesAfterInitializationFailure(t *testing.T) {
	s, dir := testSQLiteStore(t)
	cfg := &config.Config{PostgresAuthDB: "configured"}
	if _, err := s.get(nil, cfg, dir); err == nil {
		t.Fatal("nil Postgres pool must be rejected")
	}
	cfg.PostgresAuthDB = ""
	container, err := s.get(nil, cfg, dir)
	if err != nil {
		t.Fatalf("initialization failure was cached permanently: %v", err)
	}
	t.Cleanup(func() { _ = container.Close() })
}

func TestPostgresAuthStoreUsesBoundedPool(t *testing.T) {
	dsn := os.Getenv("EVOGO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("requires isolated CI Postgres")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(2)
	s := &sharedAuthStore{}
	cfg := &config.Config{PostgresAuthDB: dsn}
	container, err := s.get(db, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	verifyConcurrentSessionStore(t, s, container, db, cfg, "")
	if db.Stats().MaxOpenConnections != 3 || db.Stats().OpenConnections > 3 {
		t.Fatalf("auth pool limits were lost: %+v", db.Stats())
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("shared pool was closed on reconnect: %v", err)
	}
}
