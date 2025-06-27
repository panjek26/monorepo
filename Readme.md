# Monorepo: Go & Node.js Services with Kubernetes Deployment

## 📦 Project Structure

```
monorepo/
├── go-services/
│   └── main.go
├── node-services/
│   └── index.js
├── k8s/
│   └── helm/
│       ├── go-service/
│       └── node-service/
├── .github/
│   └── workflows/
│       └── ci-cd.yaml
└── README.md
```

## 🚀 Overview

This project demonstrates an SRE-friendly monorepo that includes:

- Two services:
  - `go-service` written in Go
  - `node-service` written in Node.js
- CI/CD pipeline using GitHub Actions
- Kubernetes manifests using Helm charts
- Monitoring with OpenTelemetry + Prometheus
- Ingress exposure using dynamic DNS (nip.io)
- Security best practices (distroless images, non-root containers, proper health checks)

---

## ⚙️ CI/CD Pipeline

Implemented via GitHub Actions:

1. **Build & Test**
   - Optionally runs unit tests (if added).
2. **Docker Image Build**
   - Go and Node.js services are built into distroless images.
3. **Push to Registry**
   - Images are pushed to GitHub Container Registry (`ghcr.io`).
4. **Deploy to Kubernetes**
   - Uses Helm charts in `k8s/helm/*`.
5. **GitOps-style Manifest Update**
   - GitHub Action step (`yaml-update-action`) automatically patches Helm `values.yaml` with new image tag.

GitHub Actions config: `.github/workflows/ci-cd.yaml`

---

## ☘️ Kubernetes Deployment

Each service is packaged as a Helm chart:

```
k8s/helm/
├── go-service/
│   ├── templates/
│   └── values.yaml
└── node-service/
    ├── templates/
    └── values.yaml
```

### 🔒 Security Best Practices

- **Distroless base image** (`gcr.io/distroless/static`)
- **Run as non-root** (`runAsNonRoot: true`, `runAsUser: 1000`)
- **Readiness and liveness probes** on `/healthz`
- **Database connection** via **Cloud SQL Auth Proxy** (for PostgreSQL):
  ```yaml
  containers:
    - name: cloud-sql-proxy
      image: gcr.io/cloudsql-docker/gce-proxy:1.33.6
      args:
        - "/cloud_sql_proxy"
        - "-instances=project:region:instance=tcp:5432"
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
  ```

---

## 📊 Monitoring & Logging

### 📈 Metrics (Prometheus + OpenTelemetry)

- Go service exposes metrics via `/metrics` using [`promhttp`](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp)
- Node.js service exposes metrics via `/metrics` on port `9464` using `@opentelemetry/exporter-prometheus`

Example custom metric:
- `login_requests_total` counter on `/login` endpoint

Both endpoints are scraped by Prometheus which is configured to target these metrics endpoints from each pod or via service.

### 🪵 Logging

- Both services use `console.log()` or equivalent logging middleware that outputs structured JSON logs.
- These logs are collected by Kubernetes (stdout), and can be integrated with tools like:
  - **Fluentd / Fluent Bit** to forward logs
  - **Loki** for log aggregation
  - **Elastic Stack (ELK)**

### 🔧 Monitoring Stack (Visualized)

```
+-------------+        +----------------------+        +--------------+
|  Node/Go    | -----> | Prometheus Exporter  | <----- |  Prometheus  |
|  Services   |        |  (/metrics endpoint) |        |              |
+-------------+        +----------------------+        +--------------+
        |                          ↑                          ↑
        |                          |                          |
        |                +----------------+           +-------------+
        |                | Structured Log | --------> |  FluentBit  |
        |                |     Output     |           |     / Loki  |
        +-----------------------------------------------------------+
```

You can visualize metrics in **Grafana**, and logs in **Grafana Loki** or Kibana.

---

## 🌐 Public Exposure via Ingress

Using **nip.io** for public access:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rewrite-target: /
  hosts:
    - host: go-service.35.244.201.179.nip.io
      paths:
        - path: /
          pathType: Prefix
  tls:
    - hosts:
        - go-service.35.244.201.179.nip.io
      secretName: go-service-tls
```

Ensure:

- You use a public IP (e.g., GCP LoadBalancer).
- Cert-Manager + Let's Encrypt is configured to issue TLS certificates.

---

## 🧰 How to Run Locally

### Go Service

```bash
cd go-services
go run main.go
```

### Node.js Service

```bash
cd node-services
npm install
npm start
```

### Access endpoints:

- `GET /login` – dummy login
- `GET /products` – list products from DB
- `GET /healthz` – readiness probe
- `GET /metrics` – Prometheus endpoint

---

## ✅ Summary

| Feature                | Status |
| ---------------------- | ------ |
| CI/CD                  | ✅      |
| Docker & Helm          | ✅      |
| Monitoring & Metrics   | ✅      |
| TLS + Public Exposure  | ✅      |
| Security Best Practice | ✅      |

---

## 🧠 Notes

- You can switch sslip.io as an alternative to nip.io.
- Use `kubectl port-forward` or `kubectl proxy` for local testing without public ingress.

