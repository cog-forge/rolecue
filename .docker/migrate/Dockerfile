# syntax=docker/dockerfile:1
# Ships the API migrations so a one-off ECS task can run them. Build from the repository root:
#   docker build -f .docker/migrate/Dockerfile -t rolecue-migrate .
# Keep the base image in sync with the migrate service in .docker/compose.deploy.yaml.
FROM migrate/migrate:v4.19.0@sha256:d5c978181e3bfa55cc50e3bd8d7da3d87418a87693453250a8804b81ee6494db
COPY api/migrations /migrations
# AWS RDS CA bundle for sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem (see .docker/certs/README.md).
COPY --chmod=0644 api/certs/rds-global-bundle.pem /etc/ssl/certs/rds-global-bundle.pem
# DATABASE_URL is expanded by the shell so it is not baked into the image or the task definition's command.
ENTRYPOINT ["/bin/sh", "-c", "exec migrate -path /migrations -database \"$DATABASE_URL\" up"]
USER 65532:65532
