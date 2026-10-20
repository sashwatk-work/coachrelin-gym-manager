# syntax=docker/dockerfile:1.7
# Build context: repository root.

FROM node:22-alpine AS builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
RUN npm run build

FROM nginx:1.28-alpine
COPY config/frontend/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /src/frontend/dist /usr/share/nginx/html
USER nginx:nginx
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/frontend-health || exit 1
