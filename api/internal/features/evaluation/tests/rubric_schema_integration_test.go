//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQL error classes the schema is expected to raise (KAN-325, migration 000032).
const (
	pgForeignKey = "23503"
	pgUnique     = "23505"
	pgCheck      = "23514"
)

func expectPgCode(t *testing.T, err error, want string, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("%s: error = %v, want SQLSTATE %s", what, err, want)
	}
}

// insertRubric adds a rubric and registers its removal. owner may be nil (a system rubric).
func (s *seed) insertRubric(scope string, owner *uuid.UUID) (uuid.UUID, error) {
	s.t.Helper()
	id := uuid.New()
	_, err := s.pool.Exec(s.ctx, `INSERT INTO public.rubrics (id, name, question_type, scope, owner_id)
		VALUES ($1, 'schema test rubric', 'concept_explanation', $2, $3)`, id, scope, owner)
	if err == nil {
		s.later(`DELETE FROM public.rubrics WHERE id = $1`, id)
	}
	return id, err
}

func (s *seed) mustRubric(scope string, owner *uuid.UUID) uuid.UUID {
	s.t.Helper()
	id, err := s.insertRubric(scope, owner)
	if err != nil {
		s.t.Fatalf("insert %s rubric: %v", scope, err)
	}
	return id
}

func (s *seed) insertCriterion(rubricID, metricID uuid.UUID, position, maxPoints int, name, guide string) error {
	s.t.Helper()
	_, err := s.pool.Exec(s.ctx, `INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
		VALUES ($1, $2, $3, $4, $5, $6)`, rubricID, metricID, name, maxPoints, guide, position)
	return err
}

func tableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func TestRubricSchemaAddsOnlyTheTwoConfigurationTables(t *testing.T) {
	ctx, pool := openPool(t)
	for _, name := range []string{"rubrics", "rubric_criteria"} {
		if !tableExists(t, ctx, pool, name) {
			t.Fatalf("table %s is missing", name)
		}
	}
	// The spec rules out snapshot/version tables, an expected-points table and result tables in this task.
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND (table_name LIKE 'rubric%' OR table_name LIKE '%expected%'
		   OR table_name LIKE '%snapshot%' OR table_name LIKE '%version%' OR table_name = 'interview_criterion_scores')
		  AND table_name <> 'schema_migrations'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	allowed := map[string]bool{"rubrics": true, "rubric_criteria": true}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if !allowed[name] {
			t.Errorf("unexpected table %s: KAN-325 adds exactly rubrics and rubric_criteria", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRubricScopeAndOwnerAreConsistent(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.recruiter()

	if _, err := s.insertRubric("system", nil); err != nil {
		t.Fatalf("a system rubric without an owner was refused: %v", err)
	}
	if _, err := s.insertRubric("recruiter", &owner); err != nil {
		t.Fatalf("a recruiter rubric with an owner was refused: %v", err)
	}
	_, err := s.insertRubric("system", &owner)
	expectPgCode(t, err, pgCheck, "a system rubric with an owner")
	_, err = s.insertRubric("recruiter", nil)
	expectPgCode(t, err, pgCheck, "a recruiter rubric without an owner")
	_, err = s.insertRubric("shared", nil)
	expectPgCode(t, err, pgCheck, "an unknown scope")

	missing := uuid.New()
	_, err = s.insertRubric("recruiter", &missing)
	expectPgCode(t, err, pgForeignKey, "an owner that is not a user")
}

func TestRubricDefaultsAndTextRules(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	id := s.mustRubric("system", nil)

	var active bool
	var createdAtSet, updatedAtSet bool
	if err := pool.QueryRow(ctx, `SELECT is_active, created_at IS NOT NULL, updated_at IS NOT NULL
		FROM public.rubrics WHERE id = $1`, id).Scan(&active, &createdAtSet, &updatedAtSet); err != nil {
		t.Fatal(err)
	}
	if !active || !createdAtSet || !updatedAtSet {
		t.Fatalf("defaults: is_active=%v created_at set=%v updated_at set=%v, want an active rubric with timestamps", active, createdAtSet, updatedAtSet)
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.rubrics (name, question_type, scope) VALUES ('   ', 'concept_explanation', 'system')`)
	expectPgCode(t, err, pgCheck, "a blank rubric name")
	_, err = pool.Exec(ctx, `INSERT INTO public.rubrics (name, question_type, scope) VALUES ('x', '', 'system')`)
	expectPgCode(t, err, pgCheck, "an empty question type")
	_, err = pool.Exec(ctx, `INSERT INTO public.rubrics (name, question_type, scope) VALUES ('x', 'Not A Slug', 'system')`)
	expectPgCode(t, err, pgCheck, "a question type that is not a lower-case slug")
}

func TestRubricCriterionRules(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	rubric := s.mustRubric("system", nil)
	guide := "0 = wrong; 1 = right"

	if err := s.insertCriterion(rubric, accuracyID, 1, 1, "Correct mechanism", guide); err != nil {
		t.Fatalf("a valid criterion was refused: %v", err)
	}
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 2, 0, "zero points", guide), pgCheck, "max_points = 0")
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 2, -3, "negative points", guide), pgCheck, "max_points < 0")
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 0, 1, "position below the first", guide), pgCheck, "position 0")
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 2, 1, "  ", guide), pgCheck, "a blank criterion name")
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 2, 1, "blank guide", " \n"), pgCheck, "a blank scoring guide")
	expectPgCode(t, s.insertCriterion(rubric, accuracyID, 1, 1, "same position", guide), pgUnique, "a duplicate position in one rubric")
	expectPgCode(t, s.insertCriterion(rubric, uuid.New(), 2, 1, "unknown metric", guide), pgForeignKey, "a metric outside the catalog")
	expectPgCode(t, s.insertCriterion(uuid.New(), accuracyID, 1, 1, "unknown rubric", guide), pgForeignKey, "a rubric that does not exist")

	// The same position is fine in another rubric.
	other := s.mustRubric("system", nil)
	if err := s.insertCriterion(other, depthID, 1, 2, "Depth", guide); err != nil {
		t.Fatalf("position 1 in a second rubric was refused: %v", err)
	}
}

func TestRubricCriteriaCanSwapPositionsInOneTransaction(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	rubric := s.mustRubric("system", nil)
	for position, name := range []string{"first", "second"} {
		if err := s.insertCriterion(rubric, accuracyID, position+1, 1, name, "0 = no; 1 = yes"); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE public.rubric_criteria SET position = 3 - position WHERE rubric_id = $1`, rubric); err != nil {
		t.Fatalf("swapping positions inside one transaction failed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing the swapped positions failed: %v", err)
	}
	var first string
	if err := pool.QueryRow(ctx, `SELECT name FROM public.rubric_criteria WHERE rubric_id = $1 AND position = 1`, rubric).Scan(&first); err != nil || first != "second" {
		t.Fatalf("criterion at position 1 = %q, %v, want %q", first, err, "second")
	}
}

func TestRubricCascadesAndOwnerErasure(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.recruiter()
	rubric := s.mustRubric("recruiter", &owner)
	if err := s.insertCriterion(rubric, accuracyID, 1, 2, "Mechanism", "0 = no; 1 = partly; 2 = fully"); err != nil {
		t.Fatal(err)
	}
	count := func(query string, id uuid.UUID) int {
		var n int
		if err := pool.QueryRow(ctx, query, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// Erasing an account that owns private rubrics must not be blocked by them.
	if _, err := pool.Exec(ctx, `DELETE FROM public.users WHERE id = $1`, owner); err != nil {
		t.Fatalf("deleting a user who owns a private rubric failed: %v", err)
	}
	if n := count(`SELECT count(*) FROM public.rubrics WHERE id = $1`, rubric); n != 0 {
		t.Fatalf("rubrics left after the owner was deleted = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM public.rubric_criteria WHERE rubric_id = $1`, rubric); n != 0 {
		t.Fatalf("criteria left after the owner was deleted = %d, want 0", n)
	}
}

func TestCoreQuestionRubricColumnsAreNullableAndGuarded(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.recruiter()
	_, jdID := s.posting(owner, "pending")

	// Existing rows and the existing insert shape keep working: both new columns are optional.
	var rubricID *uuid.UUID
	var expected *string
	if err := pool.QueryRow(ctx, `SELECT rubric_id, expected_answer FROM public.core_questions WHERE jd_id = $1`, jdID).Scan(&rubricID, &expected); err != nil {
		t.Fatal(err)
	}
	if rubricID != nil || expected != nil {
		t.Fatalf("existing question has rubric_id=%v expected_answer=%v, want both NULL", rubricID, expected)
	}

	rubric := s.mustRubric("recruiter", &owner)
	questionID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO public.core_questions (id, jd_id, content, rubric_id, expected_answer)
		VALUES ($1, $2, 'graded question', $3, 'Index = auxiliary structure; reads skip scans; writes pay maintenance.')`,
		questionID, jdID, rubric); err != nil {
		t.Fatalf("a question with a rubric and expected answer was refused: %v", err)
	}
	s.later(`DELETE FROM public.core_questions WHERE id = $1`, questionID)

	_, err := pool.Exec(ctx, `INSERT INTO public.core_questions (jd_id, content, rubric_id) VALUES ($1, 'bad ref', $2)`, jdID, uuid.New())
	expectPgCode(t, err, pgForeignKey, "a question that points at a missing rubric")

	// A rubric a question still uses cannot be hard-deleted; the references must be cleared first.
	_, err = pool.Exec(ctx, `DELETE FROM public.rubrics WHERE id = $1`, rubric)
	expectPgCode(t, err, pgForeignKey, "hard-deleting a referenced rubric")
	if _, err := pool.Exec(ctx, `UPDATE public.core_questions SET rubric_id = NULL WHERE id = $1`, questionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.rubrics WHERE id = $1`, rubric); err != nil {
		t.Fatalf("deleting an unreferenced rubric failed: %v", err)
	}
}

func TestSystemRubricTemplatesAreSeeded(t *testing.T) {
	ctx, pool := openPool(t)
	rows, err := pool.Query(ctx, `SELECT r.id, r.name, r.question_type, r.is_active, r.owner_id IS NULL
		FROM public.rubrics r WHERE r.scope = 'system' ORDER BY r.question_type`)
	if err != nil {
		t.Fatal(err)
	}
	type template struct {
		id       uuid.UUID
		name     string
		kind     string
		active   bool
		ownerNil bool
	}
	var templates []template
	for rows.Next() {
		var tpl template
		if err := rows.Scan(&tpl.id, &tpl.name, &tpl.kind, &tpl.active, &tpl.ownerNil); err != nil {
			t.Fatal(err)
		}
		templates = append(templates, tpl)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]bool{"concept_explanation": false, "comparison_choice": false, "scenario_debugging": false, "technical_experience": false}
	for _, tpl := range templates {
		if _, ok := wantTypes[tpl.kind]; ok {
			wantTypes[tpl.kind] = true
		}
		if !tpl.active || !tpl.ownerNil {
			t.Errorf("template %q: active=%v ownerless=%v, want an active system template", tpl.name, tpl.active, tpl.ownerNil)
		}
		checkSeededCriteria(t, ctx, pool, tpl.id, tpl.name)
	}
	for kind, found := range wantTypes {
		if !found {
			t.Errorf("no system template for question type %q", kind)
		}
	}

	// The canonical example of the spec: R01 has C01/C02 on Technical Accuracy (2 and 3) and C03 on
	// Problem-Solving (5), 10 points in total.
	var total int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(c.max_points), 0) FROM public.rubric_criteria c
		JOIN public.rubrics r ON r.id = c.rubric_id WHERE r.question_type = 'concept_explanation' AND r.scope = 'system'`).Scan(&total); err != nil || total != 10 {
		t.Fatalf("concept_explanation template total = %d, %v, want 10", total, err)
	}
}

func checkSeededCriteria(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rubricID uuid.UUID, name string) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT c.name, c.max_points, c.scoring_guide, c.position, m.name
		FROM public.rubric_criteria c JOIN public.metrics m ON m.id = c.metric_id
		WHERE c.rubric_id = $1 ORDER BY c.position`, rubricID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n, expectedPosition := 0, 1
	for rows.Next() {
		var critName, guide, metric string
		var maxPoints, position int
		if err := rows.Scan(&critName, &maxPoints, &guide, &position, &metric); err != nil {
			t.Fatal(err)
		}
		n++
		if position != expectedPosition {
			t.Errorf("%s / %s: position = %d, want %d (a gap-free order)", name, critName, position, expectedPosition)
		}
		expectedPosition++
		// Every integer level 0..max_points must be described, so the scorer has no gap to guess in.
		for level := 0; level <= maxPoints; level++ {
			if !strings.Contains(guide, fmt.Sprintf("%d = ", level)) {
				t.Errorf("%s / %s (%s): scoring guide does not describe level %d: %q", name, critName, metric, level, guide)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Errorf("%s has %d criteria, want at least 3", name, n)
	}
}
