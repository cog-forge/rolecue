package service

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func invalid(message string) error { return apperror.New(apperror.CodeValidation, message) }
func validatePatch(p *domain.Patch, role string) error {
	if !p.FullName.Present && !p.Image.Present && !p.CompanyName.Present && !p.CompanyWebsite.Present {
		return invalid("at least one profile field is required")
	}
	if role != "recruiter" && (p.CompanyName.Present || p.CompanyWebsite.Present) {
		return apperror.New(apperror.CodeForbidden, "company fields are only available to recruiters")
	}
	for _, v := range []*domain.StringChange{&p.FullName, &p.Image, &p.CompanyName, &p.CompanyWebsite} {
		if v.Value != nil {
			s := strings.TrimSpace(*v.Value)
			v.Value = &s
		}
	}
	if p.FullName.Present && (p.FullName.Value == nil || utf8.RuneCountInString(*p.FullName.Value) < 1 || utf8.RuneCountInString(*p.FullName.Value) > 100) {
		return invalid("full_name must contain 1 to 100 characters")
	}
	if p.CompanyName.Value != nil {
		if *p.CompanyName.Value == "" {
			p.CompanyName.Value = nil
		} else if utf8.RuneCountInString(*p.CompanyName.Value) > 200 {
			return invalid("company_name must contain at most 200 characters")
		}
	}
	if p.CompanyWebsite.Value != nil && *p.CompanyWebsite.Value == "" {
		p.CompanyWebsite.Value = nil
	}
	if p.Image.Value != nil && !validURL(*p.Image.Value) {
		return invalid("image must be an HTTPS URL without credentials, at most 2048 characters")
	}
	if p.CompanyWebsite.Value != nil && !validURL(*p.CompanyWebsite.Value) {
		return invalid("company_website must be an HTTPS URL without credentials, at most 2048 characters")
	}
	return nil
}
func validURL(s string) bool {
	if utf8.RuneCountInString(s) > 2048 {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Opaque == ""
}
