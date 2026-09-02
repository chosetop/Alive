# syntax=docker/dockerfile:1.7

FROM golang:1.25.4-bookworm AS backend-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/cli ./cmd/cli

FROM debian:bookworm-slim AS backend
RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend-build /out/server /out/cli ./
USER 65532:65532
EXPOSE 8080
CMD ["/app/server"]

FROM node:24-bookworm-slim AS frontend-build
WORKDIR /workspace
COPY packages/ ./packages/
COPY frontend/ ./frontend/
WORKDIR /workspace/frontend
RUN --mount=type=cache,id=alive-npm,target=/root/.npm,sharing=locked \
    npm ci --registry=https://registry.npmjs.org --replace-registry-host=always \
      --maxsockets=3 --fetch-retries=5 \
      --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000
RUN npm run build

FROM node:24-bookworm-slim AS frontend
ENV HOST=0.0.0.0 \
    PORT=3000 \
    NODE_ENV=production
WORKDIR /app
COPY --from=frontend-build --chown=node:node /workspace/frontend/.output ./
USER node
EXPOSE 3000
CMD ["node", "server/index.mjs"]

FROM node:24-bookworm-slim AS admin-build
WORKDIR /workspace
COPY packages/ ./packages/
COPY admin/ ./admin/
WORKDIR /workspace/packages/markdown
RUN --mount=type=cache,id=alive-npm,target=/root/.npm,sharing=locked \
    npm ci --registry=https://registry.npmjs.org --replace-registry-host=always \
      --maxsockets=3 --fetch-retries=5 \
      --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000
WORKDIR /workspace/admin
RUN --mount=type=cache,id=alive-npm,target=/root/.npm,sharing=locked \
    npm ci --registry=https://registry.npmjs.org --replace-registry-host=always \
      --maxsockets=3 --fetch-retries=5 \
      --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000
RUN VITE_API_BASE_URL= npm run build

FROM nginx:1.29-alpine AS gateway
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY --from=admin-build /workspace/admin/dist /usr/share/nginx/html/admin
EXPOSE 8081
