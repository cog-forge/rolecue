package provider

import (
	"net/url"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/profile/repository"
	"github.com/cog-forge/rolecue/api/internal/features/profile/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideProfileRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideProfileService(cfg *config.Config, repo *repository.Repository) *service.Service {
	return service.NewService(repo, service.WithImageHosts(profileImageHosts(cfg)))
}
func ProvideProfileHandler(svc *service.Service) *handler.ProfileHandler {
	return handler.NewProfileHandler(svc)
}

// defaultProfileImageHosts are the avatar hosts of the sign-in providers Better Auth fills the image from.
var defaultProfileImageHosts = []string{"lh3.googleusercontent.com", "avatars.githubusercontent.com"}

// profileImageHosts returns the hosts a profile image URL may point to: PROFILE_IMAGE_HOSTS when set,
// otherwise the known avatar hosts plus the object storage endpoint host.
func profileImageHosts(cfg *config.Config) []string {
	if len(cfg.Profile.ImageHosts) > 0 {
		return cfg.Profile.ImageHosts
	}
	hosts := append([]string(nil), defaultProfileImageHosts...)
	if u, err := url.Parse(cfg.Storage.Endpoint); err == nil && u.Hostname() != "" {
		hosts = append(hosts, strings.ToLower(u.Hostname()))
	}
	return hosts
}
