package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
	err      error
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return w.err
}

type deadlineService struct {
	RoleProfileService
	called bool
}

func (s *deadlineService) Analyze(context.Context, uuid.UUID, string, uuid.UUID) (domain.RoleProfile, error) {
	s.called = true
	return domain.RoleProfile{}, nil
}
func (s *deadlineService) Confirm(context.Context, uuid.UUID, string, uuid.UUID) (domain.RoleProfile, error) {
	s.called = true
	return domain.RoleProfile{}, nil
}
func (s *deadlineService) Get(context.Context, uuid.UUID, string, uuid.UUID) (domain.RoleProfile, error) {
	s.called = true
	return domain.RoleProfile{}, nil
}

type deadlineDocuments struct {
	DocumentService
	called bool
}

func (s *deadlineDocuments) Complete(context.Context, uuid.UUID, string, uuid.UUID) (domain.RoleProfile, error) {
	s.called = true
	return domain.RoleProfile{}, nil
}

func TestRoleProfileWriteDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, operation := range []string{"analyze", "confirm", "complete", "get"} {
		t.Run(operation, func(t *testing.T) {
			service := &deadlineService{}
			documents := &deadlineDocuments{}
			h := NewRoleProfileHandler(service, documents)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: "candidate"})
			})
			handle := h.Get
			if operation == "analyze" {
				handle = h.Analyze
			}
			if operation == "confirm" {
				handle = h.Confirm
			}
			if operation == "complete" {
				handle = h.CompleteUpload
			}
			r.POST("/:id", handle)
			w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
			start := time.Now()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/"+uuid.NewString(), nil))
			end := time.Now()
			if (!service.called && !documents.called) || w.Code != http.StatusOK {
				t.Fatalf("called=%t status=%d", service.called, w.Code)
			}
			if operation == "get" {
				if !w.deadline.IsZero() {
					t.Fatal("extended ordinary route deadline")
				}
				return
			}
			timeout := 75 * time.Second
			if operation == "complete" {
				timeout = 50 * time.Second
			}
			if w.deadline.Before(start.Add(timeout)) || w.deadline.After(end.Add(timeout)) {
				t.Fatalf("deadline %v outside request window + %v", w.deadline, timeout)
			}
		})
	}
}
func TestRoleProfileWriteDeadlineFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &deadlineService{}
	h := NewRoleProfileHandler(service, nil)
	r := gin.New()
	r.POST("/:id", func(c *gin.Context) {
		c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: "candidate"})
		h.Analyze(c)
	})
	w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder(), err: errors.New("closed connection")}
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/"+uuid.NewString(), nil))
	if service.called || w.Code != http.StatusInternalServerError {
		t.Fatalf("called=%t status=%d", service.called, w.Code)
	}
}
