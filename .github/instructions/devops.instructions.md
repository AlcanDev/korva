---
applyTo: ".gitlab-ci.yml,Dockerfile,docker-compose*.yml,helm/**,k8s/**,*.yaml,*.yml,infra/**"
---

# DevOps — GitLab CI + Docker + Kubernetes

## Dockerfile rules (hardened production images)

```dockerfile
# ✅ Multi-stage build — always
FROM node:20.11.1-alpine3.19 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production      # NOT npm install
COPY . .
RUN npm run build

FROM node:20.11.1-alpine3.19 AS runtime
WORKDIR /app

# ✅ Non-root user — mandatory
RUN addgroup -g 1001 -S nodejs && adduser -S nodeuser -u 1001 -G nodejs
USER nodeuser

COPY --from=builder --chown=nodeuser:nodejs /app/dist ./dist
COPY --from=builder --chown=nodeuser:nodejs /app/node_modules ./node_modules

# ✅ Explicit port — document what the app exposes
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/healthz || exit 1

CMD ["node", "dist/main"]
```

**Forbidden in Dockerfiles:**
- `npm install` in production stage → `npm ci`
- `USER root` or no USER directive
- `COPY . .` without `.dockerignore`
- Hardcoded secrets → use build-args or runtime env only
- Latest tag → pin exact versions

## GitLab CI — pipeline structure

```yaml
# Standard pipeline structure
include:
  - project: 'your-org/shared-jobs'
    ref: main
    file: '/templates/node-build.gitlab-ci.yml'

stages:
  - validate
  - test
  - build
  - security
  - deploy

variables:
  NODE_VERSION: "20.11.1"
  IMAGE_NAME: "$CI_REGISTRY_IMAGE:$CI_COMMIT_SHA"

# ✅ Cache node_modules to speed up pipelines
.node-cache: &node-cache
  cache:
    key: "$CI_COMMIT_REF_SLUG"
    paths:
      - node_modules/
    policy: pull-push

validate:lint:
  <<: *node-cache
  stage: validate
  script:
    - npm ci
    - npm run lint
    - npm run type-check

test:unit:
  <<: *node-cache
  stage: test
  script:
    - npm ci
    - npm run test:coverage
  coverage: '/Lines\s*:\s*(\d+\.?\d*)%/'
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage/cobertura-coverage.xml

security:secret-scan:
  stage: security
  script:
    - gitleaks detect --source . --config .gitleaks.toml --no-git
  allow_failure: false   # ← NEVER allow secret scanning to fail silently
```

## Helm chart rules

```yaml
# values.yaml — never hardcode secrets
image:
  repository: "{{ .Values.registry }}/{{ .Values.image.name }}"
  tag: "{{ .Values.image.tag }}"   # always override in CI, never 'latest'

resources:                          # always set limits
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

livenessProbe:
  httpGet:
    path: /healthz
    port: 3000
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 3000
  initialDelaySeconds: 5
  periodSeconds: 5
```

**Secrets in Kubernetes:** Always use Vault Agent Injector or External Secrets Operator.
Never mount secrets as environment variables from ConfigMaps. Never `kubectl create secret` with inline values in scripts.

## HashiCorp Vault secrets pattern

```hcl
# vault/qa.hcl — paths and policies
path "secret/data/your-org/apps/your-app/qa/*" {
  capabilities = ["read"]
}
```

```yaml
# Deployment annotation for Vault Agent Injector
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "your-app-qa"
  vault.hashicorp.com/agent-inject-secret-config: "secret/data/your-org/apps/your-app/qa/config"
  vault.hashicorp.com/agent-inject-template-config: |
    {{- with secret "secret/data/your-org/apps/your-app/qa/config" -}}
    APP_SECRET={{ .Data.data.app_secret }}
    {{- end }}
```

## Observability — OTel APM

```typescript
// Every NestJS app must have:
// 1. LoggerInterceptor (shared lib) — logs all requests/responses
// 2. OTel APM tracer initialized BEFORE any imports (main.ts)
import './tracer';  // ← must be first import

// tracer.ts
import tracer from 'otel-tracer';
tracer.init({
  service: process.env.SERVICE_NAME || 'example-bff',
  env: process.env.SERVICE_ENV || 'qa',
  version: process.env.SERVICE_VERSION || '1.0.0',
});
export default tracer;
```

**SLO targets (enforce in OTel monitors):**
- Availability: 99.9% (max 8.7h/year downtime)
- Latency p95: < 500ms
- Error rate: < 0.1%

## Istio service mesh

```yaml
# VirtualService — always define timeout and retry policy
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
spec:
  http:
    - timeout: 30s
      retries:
        attempts: 3
        perTryTimeout: 10s
        retryOn: "5xx,reset,connect-failure,retriable-4xx"
```

## Forbidden patterns

```yaml
# ❌ image: latest in Helm values
# ❌ No resource limits/requests
# ❌ No health checks (liveness + readiness)
# ❌ Secrets in ConfigMaps or environment from plain values
# ❌ allow_failure: true on security jobs
# ❌ npm install in Dockerfile
# ❌ root user in container
# ❌ No .dockerignore (sends node_modules to Docker daemon)
```
