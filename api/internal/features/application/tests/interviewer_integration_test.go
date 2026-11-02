//go:build integration

package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestCompanyInterviewerAccess(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner, candidate, foreign := s.user("recruiter"), s.user("candidate"), s.user("candidate")
	inventoryID, avatarID, voiceID := uuid.New(), uuid.New(), uuid.New()
	s.exec(`INSERT INTO inventories (id,user_id,slot) VALUES ($1,$2,3)`, inventoryID, owner)
	s.later(`DELETE FROM inventories WHERE id=$1`, inventoryID)
	s.exec(`INSERT INTO avatars (id,inventory_id,name,model_path,personality) VALUES ($1,$2,'Company interviewer','private/model.vrm','private personality')`, avatarID, inventoryID)
	s.later(`DELETE FROM avatars WHERE id=$1`, avatarID)
	s.exec(`INSERT INTO voices (id,name) VALUES ($1,'Company voice')`, voiceID)
	s.later(`DELETE FROM voices WHERE id=$1`, voiceID)
	postingID := s.posting(owner, "open", slot(3))
	s.exec(`UPDATE job_postings SET required_avatar_id=$2, required_voice_id=$3 WHERE id=$1`, postingID, avatarID, voiceID)
	appID := s.application(postingID, candidate)
	profileID := uuid.New()
	s.exec(`INSERT INTO role_profiles(id,jd_id,candidate_id,title) SELECT $1,jd_id,$2,'Practice' FROM job_postings WHERE id=$3`, profileID, candidate, postingID)
	s.later(`DELETE FROM role_profiles WHERE id=$1`, profileID)
	sessionID := uuid.New()
	s.exec(`INSERT INTO interviews(id,type,application_id) VALUES ($1,'recruitment',$2)`, sessionID, appID)
	s.later(`DELETE FROM interviews WHERE id=$1`, sessionID)
	var initialTransactions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE "from" IN ($1,$2) OR "to" IN ($1,$2)`, owner.String(), candidate.String()).Scan(&initialTransactions); err != nil {
		t.Fatal(err)
	}
	svc := newApplyService(t, pool, &fakeCVStore{})
	for _, tc := range []struct {
		name, status, posting                                   string
		expired, foreign, session, practice, completed, allowed bool
	}{
		{name: "owner preflight", status: "interview_eligible", posting: "open", allowed: true},
		{name: "intake closed preserves eligibility", status: "interview_eligible", posting: "intake_closed", allowed: true},
		{name: "owner session", status: "interview_eligible", posting: "open", session: true, allowed: true},
		{name: "foreign candidate", status: "interview_eligible", posting: "open", foreign: true},
		{name: "foreign session", status: "interview_eligible", posting: "open", session: true, foreign: true},
		{name: "expired session", status: "interview_eligible", posting: "open", session: true, expired: true},
		{name: "expired", status: "interview_eligible", posting: "open", expired: true},
		{name: "terminal posting", status: "interview_eligible", posting: "closed"},
		{name: "pending", status: "pending", posting: "open"},
		{name: "interviewed", status: "interviewed", posting: "open"},
		{name: "approved", status: "approved", posting: "open"},
		{name: "rejected", status: "rejected", posting: "open"},
		{name: "practice", status: "interview_eligible", posting: "open", session: true, practice: true},
		{name: "completed session", status: "interview_eligible", posting: "open", session: true, completed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deadline := time.Now().Add(2 * time.Minute)
			if tc.expired {
				deadline = time.Now().Add(-time.Minute)
			}
			s.exec(`UPDATE applications SET status=$2::application_status, interview_deadline=$3 WHERE id=$1`, appID, tc.status, deadline)
			s.exec(`UPDATE job_postings SET status=$2::job_posting_status WHERE id=$1`, postingID, tc.posting)
			kind := "recruitment"
			if tc.practice {
				kind = "practice"
			}
			var completed *time.Time
			if tc.completed {
				now := time.Now()
				completed = &now
			}
			var linkedApp, linkedProfile *uuid.UUID
			if tc.practice {
				linkedProfile = &profileID
			} else {
				linkedApp = &appID
			}
			s.exec(`UPDATE interviews SET type=$2::interview_type, completed_at=$3, application_id=$4,role_profile_id=$5 WHERE id=$1`, sessionID, kind, completed, linkedApp, linkedProfile)
			who := candidate
			if tc.foreign {
				who = foreign
			}
			application, session := appID, uuid.Nil
			if tc.session {
				application, session = uuid.Nil, sessionID
			}
			got, err := svc.CompanyInterviewer(ctx, who, "candidate", application, session)
			if !tc.allowed {
				assertCode(t, err, apperror.CodeApplicationNotFound)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "Company interviewer" || got.VoiceName != "Company voice" || got.ModelAccess.Method != "GET" || got.ModelAccess.ExpiresAt.After(deadline.Add(time.Millisecond)) {
				t.Fatalf("bad grant: %+v", got)
			}
			encoded, _ := json.Marshal(got)
			for _, secret := range []string{"private personality", "inventory_id", "model_path", "core_questions", "weights", "transcript"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("leaked %s", secret)
				}
			}
		})
	}

	var transactions, sessions, capacity int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM transactions WHERE "from" IN ($3,$4) OR "to" IN ($3,$4)), (SELECT count(*) FROM interviews WHERE application_id=$1), (SELECT interview_slot FROM job_postings WHERE id=$2)`, appID, postingID, owner.String(), candidate.String()).Scan(&transactions, &sessions, &capacity); err != nil {
		t.Fatal(err)
	}
	if transactions != initialTransactions || sessions != 1 || capacity != 3 {
		t.Fatalf("reads changed ledger, session count or capacity: %d %d %d", transactions, sessions, capacity)
	}
}
