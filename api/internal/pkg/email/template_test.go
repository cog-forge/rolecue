package email_test

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
)

func TestTemplateRendersSubjectAndBody(t *testing.T) {
	tpl, err := email.NewTemplate("invite",
		"You are invited, {{.Name}}",
		"Hi {{.Name}}, the role is {{.Role}}.")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.Render("candidate@example.com", map[string]string{"Name": "Ana", "Role": "Backend"})
	if err != nil {
		t.Fatal(err)
	}
	want := email.Message{
		To:      "candidate@example.com",
		Subject: "You are invited, Ana",
		Body:    "Hi Ana, the role is Backend.",
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestTemplateMissingKeyIsAnError(t *testing.T) {
	tpl, err := email.NewTemplate("invite", "Hi", "Role: {{.Role}}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.Render("a@example.com", map[string]string{"Name": "Ana"}); err == nil {
		t.Fatal("a missing key must fail the render instead of printing <no value>")
	}
}

func TestTemplateInvalidTextIsAnError(t *testing.T) {
	if _, err := email.NewTemplate("bad", "{{.Name", "body"); err == nil {
		t.Fatal("invalid subject must fail NewTemplate")
	}
	if _, err := email.NewTemplate("bad", "subject", "{{if}}"); err == nil {
		t.Fatal("invalid body must fail NewTemplate")
	}
}
