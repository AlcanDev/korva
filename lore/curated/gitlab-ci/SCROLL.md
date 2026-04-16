---
id: gitlab-ci
version: 1.0.0
team: devops
stack: GitLab CI, Docker, HashiCorp Vault, Kubernetes, Harbor
---

# Scroll: GitLab CI Patterns

## Triggers — load when:
- Files: `.gitlab-ci.yml`, `vault/*.hcl`, `Dockerfile`
- Keywords: gitlab, pipeline, harbor, registry.your-company.com, vault, hcl, npm ci, USER node, configurable-pipelines, your-org/dx, multi-stage, secret
- Tasks: creating a CI pipeline, writing a Dockerfile, managing secrets, configuring a build job, deploying to Kubernetes

## Context
Production GitLab CI pipelines pull Docker images from a private registry and inject runtime application secrets via HashiCorp Vault Agent — secrets are never baked into the image or listed in `.gitlab-ci.yml`. Build-time configuration uses GitLab CI variables; the two most common security failures are (1) hardcoded credentials in `.gitlab-ci.yml` and (2) running Node.js as root in Docker containers. Both are caught by the patterns below.

---

## Rules

### 1. Reusable pipelines from configurable-pipelines v5

```yaml
# .gitlab-ci.yml
include:
  - project: 'your-org/dx/configurable-pipelines'
    ref: 'v5'
    file:
      - '/templates/node-bff.gitlab-ci.yml'

variables:
  APP_NAME: your-app
  NODE_VERSION: "20"
  HARBOR_REGISTRY: registry.your-company.com
  IMAGE_NAME: registry.your-company.com/home/your-app
```

Do not rewrite pipeline stages that `configurable-pipelines` already defines (lint, test, build, publish). Extend or override only when truly necessary.

### 2. Docker images from Harbor — never docker.io

All base images must come from the internal Harbor registry. This ensures vulnerability scanning and license compliance.

```dockerfile
# CORRECT
FROM registry.your-company.com/base/node:20-latest AS development
FROM registry.your-company.com/base/node:20-latest AS production

# WRONG
FROM node:20-alpine      # public registry, not allowed
FROM node:20             # public registry, not allowed
```

### 3. Secrets strategy: build-time vs runtime

| Secret type | Where it lives | How it's injected |
|---|---|---|
| Build config (registry URL, app name) | GitLab CI variables | `$VARIABLE` in `.gitlab-ci.yml` |
| Docker registry credentials | GitLab CI protected variables | `$HARBOR_USER`, `$HARBOR_PASS` |
| Application secrets (DB pass, API keys) | HashiCorp Vault | Vault Agent → K8s env vars |

```yaml
# CORRECT — build-time config from GitLab variables
build:
  script:
    - docker build --build-arg APP_VERSION=$CI_COMMIT_SHORT_SHA -t $IMAGE_NAME:$CI_COMMIT_SHORT_SHA .
    - docker push $IMAGE_NAME:$CI_COMMIT_SHORT_SHA
```

```hcl
# CORRECT — vault/prod.hcl declares which Vault paths the app can read
path "secret/data/home-api/prod/external-api" {
  capabilities = ["read"]
}
```

### 4. vault/*.hcl file structure

Each app must have exactly two HCL files — one per environment. The K8s Vault Agent sidecar reads these to render secrets as environment variables.

```
apps/cl/
  vault/
    qa.hcl       <- non-prod paths
    prod.hcl     <- production paths
```

```hcl
# vault/qa.hcl
path "secret/data/home-api/qa/*" {
  capabilities = ["read"]
}
```

```hcl
# vault/prod.hcl
path "secret/data/home-api/prod/*" {
  capabilities = ["read"]
}
```

Never add a `staging.hcl` or `dev.hcl` — use the `qa` policy for all non-production environments.

### 5. Dockerfile: npm ci, multi-stage, USER node

```dockerfile
# Dockerfile
ARG APP_VERSION=local

FROM registry.your-company.com/base/node:20-latest AS development
WORKDIR /app
COPY package*.json ./
RUN npm ci                         # deterministic — always npm ci, never npm install
COPY . .
RUN npm run build

FROM registry.your-company.com/base/node:20-latest AS production
WORKDIR /app
ENV NODE_ENV=production
ARG APP_VERSION
ENV APP_VERSION=$APP_VERSION
COPY package*.json ./
RUN npm ci --only=production       # production deps only
COPY --from=development /app/dist ./dist
USER node                          # never root in production stage
EXPOSE 3000
CMD ["node", "dist/apps/cl/main.js"]
```

### 6. Pipeline stages order

```yaml
stages:
  - lint
  - test
  - build
  - publish
  - deploy-qa
  - deploy-prod
```

`deploy-prod` must require manual approval (`when: manual`) and must depend on `deploy-qa` completing successfully.

```yaml
deploy-prod:
  stage: deploy-prod
  when: manual
  needs: ["deploy-qa"]
  environment:
    name: production
```

### 7. Tagging images

Images must be tagged with the git commit SHA for traceability. Never use `latest` as the only tag in CI.

```yaml
# CORRECT
IMAGE_TAG: $CI_COMMIT_SHORT_SHA

# WRONG — latest tag loses traceability
IMAGE_TAG: latest
```

---

## Anti-Patterns

### BAD: Application secrets in .gitlab-ci.yml
```yaml
# BAD — secrets visible in pipeline logs and stored in repo
variables:
  EXTERNAL_API_API_KEY: "abc123"
  DB_PASSWORD: "supersecret"
```

```hcl
# GOOD — secrets declared as Vault paths, injected at runtime
path "secret/data/home-api/prod/external-api" {
  capabilities = ["read"]
}
```

### BAD: Public Docker base image
```dockerfile
# BAD
FROM node:20-alpine
```

```dockerfile
# GOOD
FROM registry.your-company.com/base/node:20-latest
```

### BAD: npm install instead of npm ci in Dockerfile
```dockerfile
# BAD — non-deterministic, ignores package-lock.json
RUN npm install
```

```dockerfile
# GOOD — reproducible builds
RUN npm ci
```

### BAD: Running production container as root
```dockerfile
# BAD — no USER instruction, process runs as root
FROM registry.your-company.com/base/node:20-latest
COPY --from=development /app/dist ./dist
CMD ["node", "dist/main.js"]
```

```dockerfile
# GOOD
USER node
CMD ["node", "dist/main.js"]
```

### BAD: Only tagging with latest
```bash
# BAD
docker tag $IMAGE_NAME:latest
docker push $IMAGE_NAME:latest
```

```bash
# GOOD — commit SHA + optional semver tag
docker tag $IMAGE_NAME:$CI_COMMIT_SHORT_SHA
docker push $IMAGE_NAME:$CI_COMMIT_SHORT_SHA
```
