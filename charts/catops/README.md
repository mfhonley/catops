# CatOps Kubernetes Connector

Lightweight monitoring agent for Kubernetes clusters. Automatically collects metrics from all nodes and pods.

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster 1.19+
- Helm 3.0+
- `metrics-server` installed in your cluster ([installation guide](https://github.com/kubernetes-sigs/metrics-server#installation))

### Installation

1. **Get your CatOps auth token** at [https://app.catops.io/settings/integrations](https://app.catops.io/settings/integrations)

2. **Install via Helm:**

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --set auth.token=YOUR_AUTH_TOKEN \
  --namespace catops-system \
  --create-namespace
```

**That's it!** 🎉 Metrics will appear in your CatOps dashboard within 1 minute.

---

## 📋 Configuration

### Required Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `auth.token` | Your CatOps authentication token | `abc123xyz789` |

### Optional Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `backend.url` | CatOps backend URL | `https://api.catops.io` |
| `collection.interval` | Metrics collection interval (seconds) | `60` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

### Advanced Configuration

#### Custom Backend URL

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --set auth.token=YOUR_TOKEN \
  --set backend.url=https://custom-backend.example.com
```

#### Resource Limits

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --set auth.token=YOUR_TOKEN \
  --set resources.requests.cpu=200m \
  --set resources.requests.memory=256Mi
```

#### Node Selector (specific nodes only)

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --set auth.token=YOUR_TOKEN \
  --set nodeSelector.workload=monitoring
```

---

## 🔍 What Metrics Are Collected?

### Node Metrics (per node)
- CPU usage %
- Memory usage %
- Disk usage %
- Network I/O (inbound/outbound bytes/sec)
- IOPS (read/write operations)

### Pod Metrics (per pod)
- CPU usage (cores)
- Memory usage (bytes)
- Restart count
- Phase (Running, Pending, Failed)

### Cluster Metrics (aggregated)
- Total nodes / Ready nodes
- Total pods / Running pods
- Pending / Failed pods count

---

## 🛠️ Troubleshooting

### Metrics not appearing?

**1. Check if pods are running:**

```bash
kubectl get pods -n catops-system
```

Expected output:
```
NAME                   READY   STATUS    RESTARTS   AGE
catops-xxxxx          1/1     Running   0          1m
catops-yyyyy          1/1     Running   0          1m
```

**2. Check pod logs:**

```bash
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=50
```

**3. Verify metrics-server is installed:**

```bash
kubectl top nodes
```

If this fails, install metrics-server:
```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

**4. Check authentication:**

```bash
kubectl get secret -n catops-system
```

Ensure `catops` secret exists with correct token.

---

## 🔐 RBAC Permissions

CatOps Kubernetes Connector requires the following permissions:

- **Read-only** access to:
  - Nodes
  - Pods
  - Namespaces
  - Metrics (via metrics-server API)
  - Events (optional)

**No write permissions** are granted. The connector cannot modify any resources.

---

## 🔄 Updating

```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --set auth.token=YOUR_TOKEN \
  --namespace catops-system
```

---

## 🗑️ Uninstallation

```bash
helm uninstall catops -n catops-system
kubectl delete namespace catops-system
```

---

## 📊 Dashboard

After installation, view your cluster metrics at:
👉 [https://app.catops.io/kubernetes](https://app.catops.io/kubernetes)

---

## 🐛 Support

- 📧 Email: support@catops.io
- 💬 Telegram: [@mfhonley](https://t.me/mfhonley)
- 🐛 Issues: [GitHub Issues](https://github.com/catops/cli/issues)

---

## 📜 License

MIT License - see [LICENSE](../../LICENSE) for details
