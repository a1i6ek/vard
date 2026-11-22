# syntax=docker/dockerfile:1
#
# The panel (web/console) as a Node server. Build context: the repository root.
#
#   docker build -f web/console/Dockerfile -t vard-console .
#
# The context is the repository, as for the API and the sign-in pages, so
# compose.yaml builds every image the same way.
#
# Runtime settings, all from compose.yaml: API_URL (the API inside the
# network), PORT (3000), and adapter-node's own — ADDRESS_HEADER, which header
# names the caller once a proxy is in front, and BODY_SIZE_LIMIT.

# Bun at the version the lockfile was written with: --frozen-lockfile then
# installs exactly what it says, hoisted into node_modules (bunfig.toml).
FROM oven/bun:1.4.2-alpine AS build
# Where browsers reach the app (compose.yaml passes ADMIN_URL): it is built
# into the app, which adapter-node no longer reads at run time.
ARG ORIGIN
WORKDIR /src/web/console
COPY web/console/package.json web/console/bun.lock web/console/bunfig.toml ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile --ignore-scripts
COPY web/console /src/web/console
RUN ORIGIN="${ORIGIN}" bun run build \
	&& rm -rf node_modules \
	&& bun install --frozen-lockfile --production --ignore-scripts

FROM node:24-alpine
ARG VERSION=dev
LABEL org.opencontainers.image.title="vard-console" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.source="https://github.com/a1i6ek/vard"
WORKDIR /app
ENV NODE_ENV=production PORT=3000
COPY --from=build --chown=root:root /src/web/console/package.json ./
COPY --from=build --chown=root:root /src/web/console/node_modules ./node_modules
COPY --from=build --chown=root:root /src/web/console/build ./build
# The files belong to root and the server runs as node, so it can read its
# code and change none of it.
USER node
EXPOSE 3000
CMD ["node", "build"]
