FROM oven/bun:1.4.2 AS dependencies
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts

FROM dependencies AS build
COPY tsconfig.json ./
COPY src ./src
RUN bun run typecheck && bun run build

FROM oven/bun:1.4.2 AS production-dependencies
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --production --frozen-lockfile --ignore-scripts

FROM cgr.dev/chainguard/wolfi-base:latest@sha256:81211e15a86ea1d5357f284482af45730c51de45eaf10ad9ce205d6bfc2df323 AS runtime
RUN apk add --no-cache ca-certificates libstdc++ libgcc && adduser -D -u 10001 rolecue
COPY --from=dependencies /usr/local/bin/bun /usr/local/bin/bun
WORKDIR /app
COPY --from=production-dependencies --chown=10001:10001 /app/node_modules ./node_modules
COPY --from=build --chown=10001:10001 /app/dist ./dist
USER rolecue
EXPOSE 3001
HEALTHCHECK --interval=10s --timeout=6s --start-period=15s --retries=3 CMD bun -e 'const r=await fetch("http://127.0.0.1:"+process.env.PORT+"/readyz");process.exit(r.ok?0:1)'
CMD ["bun", "dist/server.js"]
