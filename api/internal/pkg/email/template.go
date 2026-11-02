package email

import (
	"bytes"
	"fmt"
	"text/template"
)

// Template renders a Message from data. Both parts are parsed once at construction, so a broken template
// fails at startup instead of on the first email.
type Template struct {
	subject *template.Template
	body    *template.Template
}

// NewTemplate parses subject and body as text/template. missingkey=error makes a typo in a placeholder fail
// the render instead of silently printing "<no value>" to a candidate.
func NewTemplate(name, subject, body string) (*Template, error) {
	s, err := parse(name+" subject", subject)
	if err != nil {
		return nil, err
	}
	b, err := parse(name+" body", body)
	if err != nil {
		return nil, err
	}
	return &Template{subject: s, body: b}, nil
}

func parse(name, text string) (*template.Template, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse email template %s: %w", name, err)
	}
	return t, nil
}

// Render executes both parts against data and returns the message addressed to to. Template errors name the
// placeholder but never echo data values, so a failed render cannot leak a recipient into a log.
func (t *Template) Render(to string, data any) (Message, error) {
	var subject, body bytes.Buffer
	if err := t.subject.Execute(&subject, data); err != nil {
		return Message{}, fmt.Errorf("render email subject: %w", err)
	}
	if err := t.body.Execute(&body, data); err != nil {
		return Message{}, fmt.Errorf("render email body: %w", err)
	}
	return Message{To: to, Subject: subject.String(), Body: body.String()}, nil
}
