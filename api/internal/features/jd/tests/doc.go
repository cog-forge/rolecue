// Package tests holds the slow, opt-in tests of the JD feature. They need a real PostgreSQL
// database migrated to the current schema (including the level seed, 000007), so they are kept
// apart from the service unit tests.
//
// Load the local settings first, then run them with the integration tag:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/jd/tests -count=1
//
// Every test inserts rows with random IDs and deletes exactly those rows afterwards.
package tests
