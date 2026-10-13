// Package tests holds the opt-in database tests of the Admin voice catalog. They need a real
// PostgreSQL database migrated to the current schema:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/admin/voicecatalog/tests -count=1
//
// Every test inserts rows with random IDs and deletes exactly those rows afterwards.
package tests
