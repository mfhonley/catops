# CatOps Kubernetes Integration - Developer Guide

## 📁 Project Structure

```
cli/
├── cmd/
│   ├── catops/              # CLI agent (standalone servers)
│   └── kubernetes/          # K8s connector (DaemonSet)
│       └── main.go
│
├── internal/
│   ├── k8s/                 # Kubernetes-specific code
│   │   ├── client.go        # K8s API client
│   │   ├── collector.go     # Metrics collector
│   │   └── helpers.go       # Helper functions
│   └── metrics/             # Shared metrics code (used by both CLI and K8s)
│       ├── collector.go
│       └── network.go
│
├── charts/catops/           # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── templates/
│   │   ├── daemonset.yaml
│   │   ├── serviceaccount.yaml
│   │   ├── clusterrole.yaml
│   │   ├── clusterrolebinding.yaml
│   │   ├── secret.yaml
│   │   └── _helpers.tpl
│   └── README.md
│
├── Dockerfile               # CLI agent
├── Dockerfile.k8s           # K8s connector
└── .github/workflows/
    └── kubernetes.yml       # CI/CD for K8s connector
```

---

## 🔧 Development Setup

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster (для тестирования)
- Helm 3.0+
- kubectl

### Local Development

**1. Install dependencies:**

```bash
cd cli
go mod download
```

**2. Build K8s connector locally:**

```bash
go build -o catops-k8s ./cmd/kubernetes
```

**3. Test locally (requires kubeconfig):**

```bash
export CATOPS_BACKEND_URL="http://localhost:8000"
export CATOPS_AUTH_TOKEN="test-token"
export NODE_NAME=$(hostname)
export NAMESPACE="default"
export COLLECTION_INTERVAL="60"

./catops-k8s
```

---

## 🐳 Building Docker Image

### Method 1: Local Build

```bash
# Build for your platform
docker build -f Dockerfile.k8s -t catops/kubernetes-connector:dev .

# Multi-platform build (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.k8s \
  -t catops/kubernetes-connector:dev \
  --push .
```

### Method 2: GitHub Actions (Automated)

Просто push в `main` ветку:

```bash
git add .
git commit -m "feat: update kubernetes connector"
git push origin main
```

GitHub Actions автоматически:
1. Соберет Docker образ для amd64 и arm64
2. Опубликует в GitHub Container Registry (ghcr.io)
3. Создаст теги: `latest`, `sha-xxxxx`, `main`

---

## 📦 Testing Helm Chart

### Lint Chart

```bash
helm lint charts/catops
```

### Dry Run (template rendering)

```bash
helm template test charts/catops \
  --set auth.token=test-token \
  --namespace catops-system
```

### Install Locally

**With local Docker image:**

```bash
# 1. Build and load image to kind/minikube
docker build -f Dockerfile.k8s -t catops/kubernetes-connector:dev .
kind load docker-image catops/kubernetes-connector:dev  # or minikube image load

# 2. Install Helm chart
helm install catops charts/catops \
  --set auth.token=YOUR_TOKEN \
  --set image.repository=catops/kubernetes-connector \
  --set image.tag=dev \
  --set image.pullPolicy=IfNotPresent \
  --namespace catops-system \
  --create-namespace

# 3. Check pods
kubectl get pods -n catops-system

# 4. Check logs
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=50 -f
```

### Debugging

**Check DaemonSet:**
```bash
kubectl describe daemonset -n catops-system catops
```

**Check RBAC:**
```bash
kubectl get clusterrole catops-catops -o yaml
kubectl get clusterrolebinding catops-catops -o yaml
```

**Check Secret:**
```bash
kubectl get secret -n catops-system catops -o yaml
```

**Exec into pod:**
```bash
POD=$(kubectl get pods -n catops-system -l app.kubernetes.io/name=catops -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n catops-system $POD -- sh
```

---

## 🚀 Deployment

### Production Deployment

**1. Build and push Docker image:**

```bash
# Tag for production
docker tag catops/kubernetes-connector:dev ghcr.io/catops/cli/kubernetes-connector:1.0.0
docker push ghcr.io/catops/cli/kubernetes-connector:1.0.0
```

**2. Update Helm chart version:**

Edit `charts/catops/Chart.yaml`:
```yaml
version: 1.0.0
appVersion: "1.0.0"
```

**3. Package Helm chart:**

```bash
helm package charts/catops
# Creates: catops-1.0.0.tgz
```

**4. Publish Helm chart:**

```bash
# Upload to Helm repository (например, GitHub Pages)
helm repo index .
```

**5. Users install:**

```bash
helm repo add catops https://charts.catops.io
helm install catops catops/catops --set auth.token=XXX
```

---

## 🔍 Metrics Flow

```
┌─────────────────────────────────────────┐
│         Kubernetes Cluster              │
│                                         │
│  ┌────────────────────────────────┐    │
│  │  Node 1                         │    │
│  │  ┌──────────────────────────┐  │    │
│  │  │  CatOps Pod (DaemonSet)  │  │    │
│  │  │                          │  │    │
│  │  │  1. Collect node metrics │  │    │
│  │  │     (CPU, Mem, Disk)     │  │    │
│  │  │     via gopsutil         │  │    │
│  │  │                          │  │    │
│  │  │  2. Get pods on node     │  │    │
│  │  │     via K8s API          │  │    │
│  │  │                          │  │    │
│  │  │  3. Get pod metrics      │  │    │
│  │  │     via Metrics API      │  │    │
│  │  │                          │  │    │
│  │  │  4. Send to backend      │  │    │
│  │  │     POST /api/cli/k8s... │  │    │
│  │  └──────────────────────────┘  │    │
│  └────────────────────────────────┘    │
│                                         │
│  ┌────────────────────────────────┐    │
│  │  Node 2 (same as Node 1)       │    │
│  └────────────────────────────────┘    │
│                                         │
│  ┌────────────────────────────────┐    │
│  │  Node 3 (same as Node 1)       │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
           ↓
    CatOps Backend
    https://api.catops.io
           ↓
    Dashboard
    https://app.catops.io
```

---

## 📝 Code Reuse Strategy

**Shared code между CLI и K8s connector:**

```go
// internal/metrics/collector.go - SHARED CODE
package metrics

func GetCPUUsage() (float64, error) { ... }
func GetMemoryUsage() (float64, error) { ... }
func GetNetworkMetrics() (*NetworkMetrics, error) { ... }
```

**K8s connector использует:**

```go
// internal/k8s/collector.go
package k8s

import "catops/internal/metrics"

func (c *Collector) collectNodeMetrics() (*metrics.Metrics, error) {
    // Переиспользуем существующий код!
    return metrics.GetMetrics()
}
```

**Преимущества:**
- ✅ No code duplication
- ✅ Bugfixes apply to both CLI and K8s
- ✅ Consistent metrics format

---

## 🧪 Testing

### Unit Tests

```bash
# Test K8s collector
go test ./internal/k8s/...

# Test with coverage
go test -cover ./internal/k8s/...
```

### Integration Tests

```bash
# Requires running K8s cluster
export KUBECONFIG=~/.kube/config
go test -tags=integration ./internal/k8s/...
```

### E2E Tests

```bash
# Deploy to test cluster
./scripts/e2e-test.sh
```

---

## 🔐 Security Considerations

**RBAC Permissions:**
- ✅ **Read-only** access to nodes, pods, namespaces
- ✅ **No write** permissions
- ✅ **No secrets** access (except own Secret with auth token)

**Pod Security:**
- ✅ `runAsNonRoot: true`
- ✅ `readOnlyRootFilesystem: true`
- ✅ `allowPrivilegeEscalation: false`
- ✅ Capabilities dropped: ALL

**Network:**
- ✅ Only outbound HTTPS to backend
- ✅ No ingress/services created

---

## 📚 Resources

- **Kubernetes Client-Go**: https://github.com/kubernetes/client-go
- **Metrics API**: https://github.com/kubernetes/metrics
- **Helm Documentation**: https://helm.sh/docs/
- **DaemonSet Best Practices**: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/

---

## 🐛 Troubleshooting

### Issue: Metrics API not available

**Error**: `metrics API is not accessible (is metrics-server installed?)`

**Solution**:
```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

### Issue: Pods not scheduled

**Error**: DaemonSet has 0/3 pods running

**Solution**: Check node selector/tolerations:
```bash
kubectl describe daemonset -n catops-system catops
```

### Issue: Permission denied

**Error**: `pods is forbidden: User "system:serviceaccount:catops-system:catops" cannot list resource "pods"`

**Solution**: Check RBAC:
```bash
kubectl get clusterrolebinding catops-catops -o yaml
```

---

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/k8s-improvement`
3. Make changes
4. Test locally
5. Submit PR

**Before submitting:**
- [ ] Run `go fmt ./...`
- [ ] Run `helm lint charts/catops`
- [ ] Test with real K8s cluster
- [ ] Update documentation
