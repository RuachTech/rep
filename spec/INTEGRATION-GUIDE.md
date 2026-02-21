# REP Integration Guide

```
Document:  REP Integration Guide
Version:   0.1.0
Status:    Active
Authors:   Olamide Adebayo (Ruach Tech)
Created:   2026-02-18
```

---

## 1. Overview

This guide provides practical integration patterns for adopting REP across different frontend frameworks, container runtimes, and CI/CD pipelines. Each section is self-contained — jump to the section relevant to your stack.

---

## 2. Framework Integration

### 2.1 React (Vite)

**Before REP:**
```typescript
// src/config.ts
export const API_URL = import.meta.env.VITE_API_URL;
export const FEATURE_FLAGS = import.meta.env.VITE_FEATURE_FLAGS?.split(',') ?? [];
```

**After REP:**
```typescript
// src/config.ts
import { rep } from '@rep-protocol/sdk';

export const API_URL = rep.get('API_URL', 'http://localhost:3000');
export const FEATURE_FLAGS = rep.get('FEATURE_FLAGS', '')?.split(',').filter(Boolean) ?? [];
```

**React adapter (optional):**
```typescript
// src/hooks/useConfig.ts
import { rep } from '@rep-protocol/sdk';
import { useState, useEffect } from 'react';

/**
 * Synchronous hook for public REP variables.
 * Re-renders when the variable changes (if hot reload is enabled).
 */
export function useRep(key: string, defaultValue?: string): string | undefined {
  const [value, setValue] = useState(() => rep.get(key, defaultValue));

  useEffect(() => {
    const unsub = rep.onChange(key, (newValue) => setValue(newValue));
    return unsub;
  }, [key]);

  return value;
}

/**
 * Async hook for sensitive REP variables.
 */
export function useRepSecure(key: string) {
  const [state, setState] = useState<{
    value: string | null;
    loading: boolean;
    error: Error | null;
  }>({ value: null, loading: true, error: null });

  useEffect(() => {
    rep.getSecure(key)
      .then((value) => setState({ value, loading: false, error: null }))
      .catch((error) => setState({ value: null, loading: false, error }));
  }, [key]);

  return state;
}
```

**Usage in components:**
```tsx
function ApiStatus() {
  const apiUrl = useRep('API_URL');
  const flags = useRep('FEATURE_FLAGS');
  const { value: analyticsKey, loading } = useRepSecure('ANALYTICS_KEY');

  if (!apiUrl) return <div>Configuration not loaded</div>;

  return (
    <div>
      <p>API: {apiUrl}</p>
      <p>Flags: {flags}</p>
      {loading ? <p>Loading analytics...</p> : <p>Analytics ready</p>}
    </div>
  );
}
```

### 2.2 Vue 3 (Composition API)

```typescript
// src/composables/useRep.ts
import { ref, onMounted, onUnmounted } from 'vue';
import { rep } from '@rep-protocol/sdk';

export function useRep(key: string, defaultValue?: string) {
  const value = ref(rep.get(key, defaultValue));

  let unsub: (() => void) | undefined;

  onMounted(() => {
    unsub = rep.onChange(key, (newVal) => {
      value.value = newVal;
    });
  });

  onUnmounted(() => unsub?.());

  return value;
}
```

### 2.3 Svelte

```typescript
// src/lib/rep.ts
import { readable } from 'svelte/store';
import { rep } from '@rep-protocol/sdk';

export function repStore(key: string, defaultValue?: string) {
  return readable(rep.get(key, defaultValue), (set) => {
    const unsub = rep.onChange(key, (newVal) => set(newVal));
    return unsub;
  });
}
```

```svelte
<script>
  import { repStore } from '$lib/rep';
  const apiUrl = repStore('API_URL');
</script>

<p>API URL: {$apiUrl}</p>
```

### 2.4 Angular

```typescript
// src/app/services/rep.service.ts
import { Injectable } from '@angular/core';
import { rep } from '@rep-protocol/sdk';
import { BehaviorSubject, Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class RepService {
  get(key: string, defaultValue?: string): string | undefined {
    return rep.get(key, defaultValue);
  }

  getSecure(key: string): Promise<string> {
    return rep.getSecure(key);
  }

  watch(key: string): Observable<string | undefined> {
    const subject = new BehaviorSubject(rep.get(key));
    rep.onChange(key, (newVal) => subject.next(newVal));
    return subject.asObservable();
  }
}
```

### 2.5 Vanilla JavaScript / No Framework

```html
<script type="module">
  import { rep } from '/assets/rep-sdk.esm.js';

  const apiUrl = rep.get('API_URL');
  document.getElementById('api-url').textContent = apiUrl;

  // Sensitive variable
  const key = await rep.getSecure('ANALYTICS_KEY');
  initAnalytics(key);
</script>
```

### 2.6 Development Mode (Without Gateway)

During local development, you won't have the REP gateway running. The SDK handles this gracefully:

**Option A: SDK returns undefined, use defaults.**
```typescript
const apiUrl = rep.get('API_URL', 'http://localhost:3000');
// In dev, rep.get('API_URL') returns undefined → default is used
```

**Option B: Mock the payload in your HTML.**

Add this to your `index.html` (in dev only):
```html
<!-- Only in development — remove in production Dockerfile -->
<script id="__rep__" type="application/json">
{
  "public": {
    "API_URL": "http://localhost:3000",
    "FEATURE_FLAGS": "dark-mode,debug"
  },
  "_meta": {
    "version": "0.1.0",
    "injected_at": "2026-01-01T00:00:00Z",
    "integrity": "hmac-sha256:dev-mode-no-verification",
    "ttl": 0
  }
}
</script>
```

**Option C: Use the REP CLI dev server.**
```bash
npx @rep-protocol/cli dev --env .env.local --port 8080 --proxy http://localhost:5173
```

This runs a lightweight dev gateway that reads your `.env.local` file and proxies to your Vite/Webpack dev server.

---

## 3. CI/CD Integration

### 3.1 The Key Principle

With REP, your CI/CD pipeline changes fundamentally:

**Before REP:**
```
Build (staging) → Test → Deploy to staging
Build (prod)    → Deploy to prod  ← DIFFERENT BINARY
```

**After REP:**
```
Build (once) → Test → Deploy to staging (with staging env vars)
                    → Promote SAME IMAGE to prod (with prod env vars)
```

### 3.2 GitHub Actions Example

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build frontend (environment-agnostic)
        run: |
          docker build -t myapp:${{ github.sha }} .
          # NOTE: No --build-arg for API_URL, no .env files.
          # The image is environment-agnostic.

      - name: Push to registry
        run: |
          docker push ghcr.io/myorg/myapp:${{ github.sha }}

  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Run E2E tests against staging
        run: |
          docker run -d --name test-app \
            -e REP_PUBLIC_API_URL=https://api.test.example.com \
            -e REP_PUBLIC_FEATURE_FLAGS=all \
            -p 8080:8080 \
            ghcr.io/myorg/myapp:${{ github.sha }}

          npx playwright test --base-url http://localhost:8080

  deploy-staging:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to staging
        run: |
          kubectl set image deployment/frontend \
            frontend=ghcr.io/myorg/myapp:${{ github.sha }}
          # ConfigMap provides REP_PUBLIC_* and REP_SENSITIVE_* vars

  deploy-prod:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production  # Requires manual approval
    steps:
      - name: Promote SAME IMAGE to production
        run: |
          # SAME SHA — exact same binary that was tested
          kubectl set image deployment/frontend \
            frontend=ghcr.io/myorg/myapp:${{ github.sha }}
          # Different ConfigMap provides prod REP_* vars
```

### 3.3 Validating the Manifest in CI

```yaml
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Validate REP manifest
        run: npx @rep-protocol/cli validate --manifest .rep.yaml
```

This checks that:
- All required variables have corresponding entries in the deployment config
- Types match declared schemas
- No naming collisions after prefix stripping

---

## 4. Container Patterns

### 4.1 Minimal Image (FROM scratch)

The smallest possible production image — no OS, no shell, no utilities:

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM scratch
COPY --from=build /app/dist /static
COPY --from=ghcr.io/rep-protocol/gateway:latest /usr/local/bin/rep-gateway /rep-gateway

EXPOSE 8080
USER 65534:65534
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static", "--port", "8080", "--strict"]
```

**Result:** A container with exactly two things: your static files and the REP gateway binary. Attack surface: near zero.

### 4.2 Nginx + REP Sidecar (Kubernetes)

When you want to keep nginx as your file server:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          volumeMounts:
            - name: static-files
              mountPath: /usr/share/nginx/html
              readOnly: true
          ports:
            - containerPort: 80

        - name: rep-gateway
          image: ghcr.io/rep-protocol/gateway:latest
          args:
            - "--upstream=localhost:80"
            - "--port=8080"
            - "--strict"
            - "--hot-reload"
            - "--hot-reload-mode=file_watch"
            - "--watch-path=/config"
          envFrom:
            - configMapRef:
                name: frontend-public-config
            - secretRef:
                name: frontend-sensitive-config
          volumeMounts:
            - name: config-volume
              mountPath: /config
              readOnly: true
          ports:
            - containerPort: 8080
          livenessProbe:
            httpGet:
              path: /rep/health
              port: 8080
            initialDelaySeconds: 5
          readinessProbe:
            httpGet:
              path: /rep/health
              port: 8080
            initialDelaySeconds: 2

      initContainers:
        - name: copy-static
          image: ghcr.io/myorg/myapp:latest
          command: ['cp', '-r', '/app/dist/.', '/static/']
          volumeMounts:
            - name: static-files
              mountPath: /static

      volumes:
        - name: static-files
          emptyDir: {}
        - name: config-volume
          configMap:
            name: frontend-dynamic-config

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: frontend-public-config
data:
  REP_PUBLIC_API_URL: "https://api.example.com"
  REP_PUBLIC_FEATURE_FLAGS: "dark-mode,new-checkout"
  REP_PUBLIC_APP_VERSION: "2.4.1"

---
apiVersion: v1
kind: Secret
metadata:
  name: frontend-sensitive-config
type: Opaque
stringData:
  REP_SENSITIVE_ANALYTICS_KEY: "UA-XXXXX-1"

---
apiVersion: v1
kind: Service
metadata:
  name: frontend
spec:
  selector:
    app: frontend
  ports:
    - port: 80
      targetPort: 8080  # Route through REP gateway
```

### 4.3 Docker Compose (Local Development)

```yaml
version: "3.8"

services:
  frontend:
    build: .
    ports:
      - "8080:8080"
    environment:
      REP_PUBLIC_API_URL: "http://localhost:3001"
      REP_PUBLIC_FEATURE_FLAGS: "dark-mode,debug,experimental"
      REP_PUBLIC_ENV_NAME: "development"
      REP_SENSITIVE_ANALYTICS_KEY: "UA-DEV-1"
      REP_SERVER_INTERNAL_TOKEN: "dev-only-token"

  api:
    image: myapi:latest
    ports:
      - "3001:3001"
```

---

## 5. Testing

### 5.1 Unit Testing (Without Gateway)

The SDK gracefully handles the absence of the REP payload:

```typescript
// In your test setup (e.g., vitest.setup.ts)
import { JSDOM } from 'jsdom';

function mockRepPayload(vars: Record<string, string>) {
  const script = document.createElement('script');
  script.id = '__rep__';
  script.type = 'application/json';
  script.textContent = JSON.stringify({
    public: vars,
    _meta: {
      version: '0.1.0',
      injected_at: new Date().toISOString(),
      integrity: 'hmac-sha256:test-mode',
      ttl: 0,
    },
  });
  document.head.appendChild(script);
}

// In your test:
beforeEach(() => {
  mockRepPayload({
    API_URL: 'https://api.test.example.com',
    FEATURE_FLAGS: 'test-flag',
  });
});
```

### 5.2 Integration Testing (With Gateway)

```yaml
# docker-compose.test.yml
services:
  app:
    build: .
    environment:
      REP_PUBLIC_API_URL: "https://api.test.example.com"
      REP_PUBLIC_FEATURE_FLAGS: "all"
    ports:
      - "8080:8080"

  playwright:
    image: mcr.microsoft.com/playwright:latest
    command: npx playwright test
    environment:
      BASE_URL: "http://app:8080"
    depends_on:
      - app
```

### 5.3 Gateway Health Check Testing

```bash
# Verify gateway is running and configuration is correct
curl -s http://localhost:8080/rep/health | jq .

# Expected output:
# {
#   "status": "healthy",
#   "version": "0.1.0",
#   "variables": {
#     "public": 3,
#     "sensitive": 1,
#     "server": 1
#   },
#   "guardrails": {
#     "warnings": 0,
#     "blocked": 0
#   },
#   "uptime_seconds": 42
# }
```

---

## 6. Migration Checklist

### Phase 1: Infrastructure (No Code Changes)

- [ ] Add REP gateway binary to your Dockerfile
- [ ] Configure gateway as entrypoint
- [ ] Add `REP_PUBLIC_*` environment variables mirroring your existing build-time vars
- [ ] Deploy — at this point both build-time and runtime vars work simultaneously

### Phase 2: SDK Adoption (Gradual Code Changes)

- [ ] Install `@rep-protocol/sdk`
- [ ] Create a `src/config.ts` module centralising all config access
- [ ] Replace `import.meta.env.VITE_X` with `rep.get('X')` in `src/config.ts`
- [ ] Update imports across your codebase to use `src/config.ts`
- [ ] Run your test suite — everything should pass with both sources

### Phase 3: Build Cleanup (Remove Build-Time Vars)

- [ ] Remove `VITE_*` / `REACT_APP_*` from your CI build steps
- [ ] Remove `.env.production`, `.env.staging` files from source control
- [ ] Verify your build works without any env vars (produces a generic bundle)
- [ ] Verify the bundle works in all environments via REP only

### Phase 4: Harden (Optional)

- [ ] Add `.rep.yaml` manifest
- [ ] Enable `--strict` mode
- [ ] Classify sensitive variables under `REP_SENSITIVE_*`
- [ ] Move server-only secrets to `REP_SERVER_*`
- [ ] Generate TypeScript types from manifest
- [ ] Configure CSP headers
- [ ] Set up monitoring and alerting on gateway logs

---

*End of REP Integration Guide*
