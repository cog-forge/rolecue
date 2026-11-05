package config

import (
	"errors"
	"net/url"
	"strings"
)

// databaseConn is the part of a connection setting the production checks look at.
type databaseConn struct {
	user, password, sslMode, sslRootCert string
}

func (d DatabaseConfig) conn() (databaseConn, error) {
	if d.URL == "" {
		return databaseConn{user: d.User, password: d.Password, sslMode: d.SSLMode, sslRootCert: d.SSLRootCert}, nil
	}
	u, err := url.Parse(d.URL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return databaseConn{}, errors.New("database.url must be a postgres:// URL")
	}
	c := databaseConn{sslMode: u.Query().Get("sslmode"), sslRootCert: u.Query().Get("sslrootcert")}
	if u.User != nil {
		c.user = u.User.Username()
		c.password, _ = u.User.Password()
	}
	return c, nil
}

// validateProduction rejects deployment defaults that must never reach production.
func (d DatabaseConfig) validateProduction() error {
	if d.URL == "" && (strings.TrimSpace(d.Host) == "" || strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.User) == "") {
		return errors.New("production requires DATABASE_URL or an explicit database host, name and user")
	}
	c, err := d.conn()
	if err != nil {
		return err
	}
	if c.password == "" {
		return errors.New("production requires a non-empty database password")
	}
	if c.user == "postgres" && c.password == "postgres" {
		return errors.New("production must not use the default postgres/postgres database credentials")
	}
	if d.RequireTLS {
		if c.sslMode != "verify-full" {
			return errors.New("database.require_tls requires sslmode=verify-full")
		}
		if strings.TrimSpace(c.sslRootCert) == "" {
			return errors.New("database.require_tls requires sslrootcert")
		}
	}
	return nil
}
