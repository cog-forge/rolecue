// Package tests holds the slow, opt-in tests of the question-bank feature. They are kept apart
// from the production code because they need a real PostgreSQL database and/or a real Gemini key.
// Unit tests that need neither live next to the code they test (service/, provider/, handler/).
//
// Load the local settings first (the key only ever goes in the git-ignored .env):
//
//	cd api && set -a && . ../.env && set +a
//
// Database tests (also what `make test-integration` runs through ./internal/...):
//
//	go test -tags=integration ./internal/features/questionbank/tests -count=1
//
// One real Gemini call with synthetic requirements:
//
//	go test -tags=llm_smoke ./internal/features/questionbank/tests -run TestLLMSmoke -v
//
// Full flow (HTTP handlers, service, Gemini and PostgreSQL; only the login session is stubbed):
//
//	go test -tags "integration llm_smoke" ./internal/features/questionbank/tests -run E2E -v
package tests
