# ─── Stage 1: Build ─────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Install dependencies first (better layer caching)
COPY package*.json ./
RUN npm ci --prefer-offline

# Copy source and build
COPY . .
RUN npm run build

# ─── Stage 2: Serve ─────────────────────────────────────────────────────────
FROM nginx:1.27-alpine

# Install gettext for envsubst
RUN apk add --no-cache gettext

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx template (uses envsubst for runtime config)
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Copy SEO static files
COPY public/robots.txt    /usr/share/nginx/html/robots.txt
COPY public/sitemap.xml   /usr/share/nginx/html/sitemap.xml

# Runtime env defaults
ENV VAULT_API_URL=http://korva-vault:7437

EXPOSE 80

# Substitute env vars into nginx config at startup, then launch nginx
CMD ["/bin/sh", "-c", \
  "envsubst '${VAULT_API_URL}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"]
