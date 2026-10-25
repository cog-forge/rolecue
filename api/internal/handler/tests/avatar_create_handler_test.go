package tests

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type slowAvatarCreation struct{}

func (slowAvatarCreation) Create(ctx context.Context, _ authdomain.User, _ io.ReadSeeker, _ int64, _ domain.CreateAvatarInput) (domain.Avatar, error) {
	select {
	case <-time.After(150 * time.Millisecond):
		return domain.Avatar{Name: "Test"}, nil
	case <-ctx.Done():
		return domain.Avatar{}, ctx.Err()
	}
}

// Use a real server: a recorder cannot reproduce net/http's write deadline.
func TestAvatarCreationExtendsServerWriteTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		setUser(c, authdomain.User{ID: uuid.New(), Role: "candidate", EmailVerified: true})
	})
	router.POST("/inventory/avatars", handler.NewAvatarCreateHandler(slowAvatarCreation{}, 1024, time.Second).Create)
	server := httptest.NewUnstartedServer(router)
	server.Config.WriteTimeout = 25 * time.Millisecond
	server.Start()
	defer server.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "avatar.glb")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "test GLB"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("metadata", `{"name":"Test","personality":"Friendly"}`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.Timeout = 5 * time.Second
	response, err := client.Post(server.URL+"/inventory/avatars", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("conversion outlasting server write timeout lost response: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
}
