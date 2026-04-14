---
id: docker-k8s
version: 1.0.0
team: devops
stack: Docker, Kubernetes, Helm, Istio, HPA
---

# Scroll: Docker & Kubernetes — Production Hardening

## Triggers — load when:
- Files: `Dockerfile`, `docker-compose*.yml`, `helm/**`, `k8s/**`, `*.yaml`
- Keywords: docker, dockerfile, kubernetes, k8s, helm, pod, deployment, ingress, HPA, resource limits, health check, liveness, readiness, secret, configmap

## Rules

### 1. Dockerfile — production-hardened template

```dockerfile
# ─── Stage 1: Dependencies ─────────────────────────
FROM node:20.11.1-alpine3.19 AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

# ─── Stage 2: Build ────────────────────────────────
FROM node:20.11.1-alpine3.19 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# ─── Stage 3: Runtime ──────────────────────────────
FROM node:20.11.1-alpine3.19 AS runtime
WORKDIR /app

# Non-root user (mandatory)
RUN addgroup -g 1001 -S appgroup && adduser -S appuser -u 1001 -G appgroup
USER appuser

# Copy only what's needed
COPY --from=deps    --chown=appuser:appgroup /app/node_modules ./node_modules
COPY --from=builder --chown=appuser:appgroup /app/dist ./dist

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:3000/healthz || exit 1

CMD ["node", "dist/main"]
```

**.dockerignore (always present):**
```
node_modules
dist
.git
*.env
*.key
coverage
.nyc_output
docs
*.md
```

### 2. Kubernetes Deployment — full production spec

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  replicas: 2          # never 1 — single point of failure
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0   # zero-downtime deploys
  selector:
    matchLabels:
      app: my-service
  template:
    metadata:
      labels:
        app: my-service
        version: "{{ .Values.image.tag }}"
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        fsGroup: 1001
      containers:
        - name: my-service
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: Always
          ports:
            - containerPort: 3000

          # Resource limits — always set both requests and limits
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"

          # Health probes
          livenessProbe:
            httpGet:
              path: /healthz
              port: 3000
            initialDelaySeconds: 30
            periodSeconds: 10
            failureThreshold: 3

          readinessProbe:
            httpGet:
              path: /readyz
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 3

          # Security context per container
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]

          env:
            - name: NODE_ENV
              value: production
            - name: PORT
              value: "3000"
          # Secrets come from Vault Agent Injector — not from here
```

### 3. HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-service
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### 4. PodDisruptionBudget

```yaml
# Ensures at least 1 pod is always available during node drain
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-service-pdb
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: my-service
```

### 5. Health endpoint implementation

```typescript
// src/health/health.controller.ts
@Controller()
export class HealthController {
  constructor(
    @InjectConnection() private connection: Connection, // or remove if stateless
  ) {}

  @Get('healthz')
  @Public()
  liveness() {
    // Liveness: is the process alive?
    return { status: 'ok', timestamp: new Date().toISOString() };
  }

  @Get('readyz')
  @Public()
  async readiness() {
    // Readiness: is the service ready to accept traffic?
    // For stateless BFFs: always ready if process is up
    // For stateful services: check downstream dependencies
    return { status: 'ok', timestamp: new Date().toISOString() };
  }
}
```

---

## Anti-patterns

```dockerfile
# ❌ Single-stage build (ships dev dependencies)
FROM node:20
COPY . .
RUN npm install && npm run build
CMD ["node", "dist/main"]

# ❌ Running as root
# No USER directive = runs as root = security risk

# ❌ Pinning to mutable tags
FROM node:latest
FROM node:20-alpine   # tag can change without notice

# ❌ Secrets in ENV
ENV API_KEY=my-secret-key-here

# ❌ No .dockerignore (sends node_modules to Docker daemon)
```

```yaml
# ❌ No resource limits (can starve other pods)
containers:
  - name: app
    image: my-app:latest   # also: never latest in prod

# ❌ replicas: 1 in production

# ❌ No health probes (K8s can't detect unhealthy pods)

# ❌ Secrets in ConfigMap (plaintext in etcd)
```
