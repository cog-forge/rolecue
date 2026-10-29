package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBoardPublicationWindow(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/job-board?published_from=2026-10-29T00:00:00Z&published_to=2026-10-30T00:00:00Z", nil)
	filter, ok := jobBoardFilter(c)
	if !ok || filter.PublishedFrom == nil || filter.PublishedTo == nil || filter.PublishedTo.Sub(*filter.PublishedFrom).Hours() != 24 {
		t.Fatalf("publication window lost: %+v", filter)
	}
	for _, query := range []string{"published_from=yesterday", "published_to=", "published_from=2026-10-29T00:00:00Z&published_from=2026-10-28T00:00:00Z"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/job-board?"+query, nil)
		if _, ok := jobBoardFilter(c); ok {
			t.Errorf("accepted invalid publication filter %s", query)
		}
	}
}
