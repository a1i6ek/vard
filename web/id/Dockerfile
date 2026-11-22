# syntax=docker/dockerfile:1
#
# The sign-in pages (web/id) as a Node server. Build context: the repository root.
#
#   docker build -f web/id/Dockerfile -t vard-id .
#
# The context is the repository and not this directory because the app
# imports the shipped translations from i18n/ at the top of it
# (src/lib/i18n/messages.ts), so the app keeps its place in the tree inside
# the image — /src/web/id — and that relative import still points at
# /src/i18n.
#
# Runtime settings, all from compose.yaml: API_URL (the API inside the
# network), PORT (3000), and adapter-node's own — ADDRESS_HEADER, which header
# names the caller once a proxy is in front, and BODY_SIZE_LIMIT.

# Bun at the version the lockfile was written with: --frozen-lockfile then
# installs exactly what it says, hoisted into node_modules (bunfig.toml).
FROM oven/bun:1.4.2-alpine AS build
# Where browsers reach the app (compose.yaml passes PUBLIC_URL): it is built
# into the app, which adapter-node no longer reads at run time.
ARG ORIGIN
WORKDIR /src/web/id
COPY web/id/package.json web/id/bun.lock web/id/bunfig.toml ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile --ignore-scripts
COPY i18n /src/i18n
COPY web/id /src/web/id
RUN ORIGIN="${ORIGIN}" bun run build \
	&& rm -rf node_modules \
	&& bun install --frozen-lockfile --production --ignore-scripts

FROM node:24-alpine
ARG VERSION=dev
LABEL org.opencontainers.image.title="vard-id" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.source="https://github.com/a1i6ek/vard"
WORKDIR /app
ENV NODE_ENV=production PORT=3000
COPY --from=build --chown=root:root /src/web/id/package.json ./
COPY --from=build --chown=root:root /src/web/id/node_modules ./node_modules
COPY --from=build --chown=root:root /src/web/id/build ./build
# The files belong to root and the server runs as node, so it can read its
# code and change none of it.
USER node
EXPOSE 3000
CMD ["node", "build"]
