package database

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Production points DATABASE_URL at the RDS CA bundle baked into the image at
// /etc/ssl/certs/rds-global-bundle.pem. pgx must read that file itself, trust only
// it, and keep certificate and hostname verification on.
func TestPoolConfigTrustsSSLRootCertFromURL(t *testing.T) {
	bundle := filepath.Join("..", "..", "certs", "rds-global-bundle.pem")
	pem, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	want := x509.NewCertPool()
	if !want.AppendCertsFromPEM(pem) {
		t.Fatal("bundle holds no parsable certificates")
	}

	abs, err := filepath.Abs(bundle)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DatabaseConfig{URL: "postgres://rolecue:secret@db.example.rds.amazonaws.com:5432/rolecue?sslmode=verify-full&sslrootcert=" + abs}
	pc, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	tlsCfg := pc.ConnConfig.TLSConfig
	if tlsCfg == nil {
		t.Fatal("TLSConfig is nil for sslmode=verify-full")
	}
	if tlsCfg.RootCAs == nil || !tlsCfg.RootCAs.Equal(want) {
		t.Fatal("RootCAs is not exactly the sslrootcert bundle")
	}
	if tlsCfg.InsecureSkipVerify {
		t.Fatal("certificate verification is disabled")
	}
	if tlsCfg.ServerName != "db.example.rds.amazonaws.com" {
		t.Fatalf("ServerName = %q, want the database host", tlsCfg.ServerName)
	}
	if len(pc.ConnConfig.Fallbacks) != 0 {
		t.Fatalf("verify-full must not fall back to a weaker connection, got %d fallbacks", len(pc.ConnConfig.Fallbacks))
	}
}

func TestPoolConfigWithoutTLSIsUntouched(t *testing.T) {
	cfg := config.DatabaseConfig{URL: "postgres://rolecue:secret@db:5432/rolecue?sslmode=disable"}
	pc, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if pc.ConnConfig.TLSConfig != nil {
		t.Fatal("sslmode=disable must not configure TLS")
	}
}
