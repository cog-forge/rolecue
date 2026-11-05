package config

import (
	"os"
	"strings"
	"testing"
)

func prodConfig() Config {
	cfg := validConfig()
	cfg.App.Environment = "production"
	cfg.Auth.URL = "https://rolecue.example"
	cfg.Auth.PublicURL = "https://rolecue.example"
	cfg.Auth.SecureCookies = true
	cfg.Auth.CookieDomain = "rolecue.example"
	cfg.Database = DatabaseConfig{Host: "db.internal", Port: 5432, User: "rolecue", Password: "s3cret-value", Name: "rolecue", SSLMode: "disable"}
	return cfg
}

func TestProductionDatabaseCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modify  func(c *Config)
		wantErr string
	}{
		{"strong credentials", func(c *Config) {}, ""},
		{"default postgres credentials", func(c *Config) { c.Database.User, c.Database.Password = "postgres", "postgres" }, "default postgres/postgres"},
		{"empty password", func(c *Config) { c.Database.Password = "" }, "non-empty database password"},
		{"missing dsn", func(c *Config) { c.Database = DatabaseConfig{} }, "requires DATABASE_URL"},
		{"url with default credentials", func(c *Config) {
			c.Database = DatabaseConfig{URL: "postgres://postgres:postgres@db/rolecue"}
		}, "default postgres/postgres"},
		{"url without password", func(c *Config) { c.Database = DatabaseConfig{URL: "postgres://rolecue@db/rolecue"} }, "non-empty database password"},
		{"url with strong credentials", func(c *Config) {
			c.Database = DatabaseConfig{URL: "postgres://rolecue:s3cret-value@db/rolecue"}
		}, ""},
		{"non-postgres url", func(c *Config) { c.Database = DatabaseConfig{URL: "mysql://u:p@db/x"} }, "postgres:// URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := prodConfig()
			tc.modify(&cfg)
			assertValidate(t, cfg, tc.wantErr)

			// The same settings are accepted outside production.
			cfg.App.Environment = "development"
			cfg.Auth = validConfig().Auth
			if err := cfg.Validate(); err != nil {
				t.Fatalf("development Validate() = %v, want nil", err)
			}
		})
	}
}

func TestRequireTLSBySSLMode(t *testing.T) {
	const cert = "/etc/ssl/db-ca.pem"
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full", ""} {
		for _, requireTLS := range []bool{false, true} {
			for _, env := range []string{"production", "development"} {
				t.Run(env+"/"+mode+"/requireTLS="+map[bool]string{false: "off", true: "on"}[requireTLS], func(t *testing.T) {
					cfg := prodConfig()
					cfg.App.Environment = env
					if env == "development" {
						cfg.Auth = validConfig().Auth
					}
					cfg.Database.SSLMode = mode
					cfg.Database.SSLRootCert = cert
					cfg.Database.RequireTLS = requireTLS
					want := ""
					if env == "production" && requireTLS && mode != "verify-full" {
						want = "sslmode=verify-full"
					}
					assertValidate(t, cfg, want)
				})
			}
		}
	}
}

func TestRequireTLSNeedsRootCert(t *testing.T) {
	cfg := prodConfig()
	cfg.Database.RequireTLS = true
	cfg.Database.SSLMode = "verify-full"
	assertValidate(t, cfg, "sslrootcert")

	cfg.Database.SSLRootCert = "/etc/ssl/db-ca.pem"
	assertValidate(t, cfg, "")

	cfg.Database = DatabaseConfig{URL: "postgres://rolecue:s3cret-value@db/rolecue?sslmode=verify-full", RequireTLS: true}
	assertValidate(t, cfg, "sslrootcert")
	cfg.Database.URL += "&sslrootcert=/etc/ssl/db-ca.pem"
	assertValidate(t, cfg, "")

	cfg.Database.URL = "postgres://rolecue:s3cret-value@db/rolecue?sslmode=prefer&sslrootcert=/x"
	assertValidate(t, cfg, "sslmode=verify-full")
}

func TestDatabaseDSNIncludesRootCert(t *testing.T) {
	d := DatabaseConfig{Host: "db", Port: 5432, User: "u", Password: "p", Name: "n", SSLMode: "verify-full", SSLRootCert: "/ca.pem"}
	if dsn := d.DSN(); !strings.Contains(dsn, "sslrootcert=%2Fca.pem") || !strings.Contains(dsn, "sslmode=verify-full") {
		t.Fatalf("DSN = %q", dsn)
	}
}

func TestRequireTLSDefaultsOff(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "id")
	t.Setenv("PAYOS_API_KEY", "key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "sum")
	t.Setenv("DATABASE_REQUIRE_TLS", "")
	if err := os.Unsetenv("DATABASE_REQUIRE_TLS"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.RequireTLS {
		t.Fatal("database.require_tls must default to false")
	}
	t.Setenv("DATABASE_REQUIRE_TLS", "true")
	cfg, err = Load(writeConfig(t, minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Database.RequireTLS {
		t.Fatal("DATABASE_REQUIRE_TLS=true was not applied")
	}
}

func assertValidate(t *testing.T, cfg Config, wantErr string) {
	t.Helper()
	err := cfg.Validate()
	switch {
	case wantErr == "" && err != nil:
		t.Fatalf("Validate() = %v, want nil", err)
	case wantErr != "" && (err == nil || !strings.Contains(err.Error(), wantErr)):
		t.Fatalf("Validate() = %v, want error containing %q", err, wantErr)
	}
}
