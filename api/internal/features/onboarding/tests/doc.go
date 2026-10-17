// Package tests holds opt-in PostgreSQL tests for onboarding. Unit tests stay
// beside auth/profile services and handlers; database tests use the integration tag.
//
// Point DATABASE_URL at a disposable database with the repository migrations applied:
//
//	go test -tags=integration ./internal/features/onboarding/tests -count=1 -v
//
// Fixtures are unique users created inside transactions that always roll back.
// These tests never apply migrations, truncate tables, or call Better Auth/Gemini.
package tests
