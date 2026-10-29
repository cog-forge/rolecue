package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPostingFundingAndSubmissionFilters(t *testing.T) {
	for _, query := range []string{"funded=false", "funded=true", "review_submitted=false", "review_submitted=true"} {
		t.Run(query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/job-postings?"+query, nil)
			filter, ok := jobPostingFilter(c)
			if !ok {
				t.Fatal("valid boolean filter rejected")
			}
			value := filter.Funded
			if query[0] == 'r' {
				value = filter.ReviewSubmitted
			}
			if value == nil || *value != (query[len(query)-4:] == "true") {
				t.Fatalf("filter was not preserved: %+v", filter)
			}
		})
	}
	for _, query := range []string{"funded=maybe", "funded=", "funded=true&funded=false", "review_submitted=1", "review_submitted=false&review_submitted=true"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/job-postings?"+query, nil)
		if _, ok := jobPostingFilter(c); ok {
			t.Errorf("accepted invalid filter %s", query)
		}
	}
}
