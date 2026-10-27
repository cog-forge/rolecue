# syntax=docker/dockerfile:1
FROM oven/bun:1.4.2 AS bun
FROM node:24.20.0-trixie-slim AS build
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY . .
ARG NEXT_PUBLIC_API_BASE_URL
# Public Avaturn project origin for the avatar creator. Optional: without it, creation is disabled.
ARG NEXT_PUBLIC_AVATURN_PROJECT_URL
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL NEXT_PUBLIC_AVATURN_PROJECT_URL=$NEXT_PUBLIC_AVATURN_PROJECT_URL NEXT_TELEMETRY_DISABLED=1
RUN node -e 'const u = new URL(process.env.NEXT_PUBLIC_API_BASE_URL); if (!["http:","https:"].includes(u.protocol) || u.username || u.password) process.exit(1)' \
    && bun run build

# A minimal, patched runtime avoids shipping npm and Debian administration packages.
FROM cgr.dev/chainguard/node:latest@sha256:88c4f6e6941bdd4fb9039d98ad05384e1bfe5693dd31ccc8eb2f0c8522bdb40e AS runtime
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 HOSTNAME=0.0.0.0 PORT=3000
COPY --from=build --chown=65532:65532 /app/.next/standalone ./
COPY --from=build --chown=65532:65532 /app/.next/static ./.next/static
COPY --from=build --chown=65532:65532 /app/public ./public
USER 65532
EXPOSE 3000
CMD ["server.js"]
