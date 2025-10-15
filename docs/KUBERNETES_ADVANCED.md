# Kubernetes Advanced Configuration

This guide covers advanced Kubernetes topics, Prometheus integration details, and technical architecture for CatOps.

---

## Table of Contents

- [Extended Metrics with Prometheus](#extended-metrics-with-prometheus)
- [Advanced Configuration](#advanced-configuration)
- [Architecture & Data Flow](#architecture--data-flow)
- [ClickHouse Queries](#clickhouse-queries)
- [Performance Tuning](#performance-tuning)
- [Security & RBAC](#security--rbac)

---

## Extended Metrics with Prometheus

### What is Prometheus?

Prometheus is an optional component that provides 200+ extended metrics beyond basic Kubernetes monitoring. When enabled, you get:
- Pod labels (for filtering and grouping)
- Owner information (which Deployment/StatefulSet owns each pod)
- Container details (images, status, ready state)
- Pod age and creation timestamps
- Enhanced resource metrics

### How to Enable

```bash
# Enable Prometheus during installation
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_AUTH_TOKEN \
  --set prometheus.enabled=true \
  --set kubeStateMetrics.enabled=true \
  --set nodeExporter.enabled=true
```

### Extended Pod Metrics Schema

When Prometheus is enabled, each pod includes:

```json
{
  "name": "nginx-abc123",
  "namespace": "default",
  "phase": "Running",
  "cpu_usage_cores": 0.05,
  "memory_usage_bytes": 52428800,
  "labels": {
    "app": "nginx",
    "version": "1.21",
    "env": "production",
    "helm.sh/chart": "nginx-1.0.0"
  },
  "owner_kind": "Deployment",
  "owner_name": "nginx-deployment",
  "containers": [
    {
      "name": "nginx",
      "image": "nginx:1.21",
      "ready": true,
      "status": "running"
    }
  ],
  "pod_age_seconds": 3600,
  "created_at": "2025-01-01T12:00:00Z"
}
```

### Components Installed

**1. Prometheus Server**
- Time-series database and query engine
- Short-term storage (1h retention)
- No persistent volume (data stored in CatOps backend long-term)

**2. kube-state-metrics**
- Exposes Kubernetes cluster state
- Provides pod labels, owner info, metadata
- Deployed as single Deployment per cluster

**3. node-exporter** (optional)
- Exposes node-level metrics
- CPU, memory, disk, network stats
- DaemonSet on each node

### Prometheus Configuration

Default Prometheus configuration in `values.yaml`:

```yaml
prometheus:
  enabled: true
  url: "http://{{ .Release.Name }}-prometheus-server:80"

  server:
    retention: "1h"  # Short retention
    persistentVolume:
      enabled: false  # No persistent storage
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        cpu: 500m
        memory: 512Mi

  alertmanager:
    enabled: false

  prometheus-pushgateway:
    enabled: false
```

### Disable Prometheus

```bash
# Disable Prometheus (keeps basic metrics)
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=false \
  --set kubeStateMetrics.enabled=false \
  --set nodeExporter.enabled=false

# Verify Prometheus pods are removed
kubectl get pods -n catops-system -w
```

**Note**: After disabling, CatOps continues working with basic metrics only (no labels, owner info, or container details).

---

## Advanced Configuration

### Custom Resource Limits

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set resources.limits.cpu=300m \
  --set resources.limits.memory=512Mi \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=256Mi
```

### Custom Collection Interval

```bash
# Collect every 30 seconds (more frequent)
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set collection.interval=30

# Collect every 2 minutes (less frequent, saves resources)
--set collection.interval=120
```

### Node Selector (Run on Specific Nodes)

```bash
# Only run on nodes with monitoring=enabled label
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set nodeSelector.monitoring=enabled

# Then label nodes
kubectl label nodes node1 monitoring=enabled
kubectl label nodes node2 monitoring=enabled
```

### Tolerations (Run on Tainted Nodes)

Default tolerations allow CatOps to run on control-plane nodes:

```yaml
tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
```

Add custom tolerations:

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set-json 'tolerations=[{"key":"special","operator":"Equal","value":"true","effect":"NoSchedule"}]'
```

### Custom Backend URL (Self-hosted)

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --set auth.token=YOUR_TOKEN \
  --set backend.url=https://your-backend.com
```

### Using custom values.yaml

Create `custom-values.yaml`:

```yaml
auth:
  token: YOUR_AUTH_TOKEN

prometheus:
  enabled: true

kubeStateMetrics:
  enabled: true

nodeExporter:
  enabled: false  # Disable for Docker Desktop

collection:
  interval: 60

resources:
  limits:
    cpu: 300m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 256Mi

nodeSelector:
  monitoring: enabled

tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
```

Install with custom values:

```bash
helm install catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --create-namespace \
  --values custom-values.yaml
```

---

## Architecture & Data Flow

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Node 1     │  │   Node 2     │  │   Node 3     │      │
│  │              │  │              │  │              │      │
│  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │      │
│  │  │CatOps  │  │  │  │CatOps  │  │  │  │CatOps  │  │      │
│  │  │Pod     │──┼──┼──│Pod     │──┼──┼──│Pod     │  │      │
│  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │      │
│  │      │       │  │      │       │  │      │       │      │
│  └──────┼───────┘  └──────┼───────┘  └──────┼───────┘      │
│         │                 │                 │              │
│         └─────────────────┼─────────────────┘              │
│                           │                                │
│  ┌────────────────────────▼──────────────────────────┐     │
│  │            Prometheus (Optional)                   │     │
│  │  • kube-state-metrics (pod labels, owners)        │     │
│  │  • node-exporter (node metrics)                   │     │
│  │  • 200+ metrics collection                        │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ HTTPS
                           ▼
                ┌──────────────────────┐
                │   CatOps Backend     │
                │   api.catops.io      │
                └──────────────────────┘
                           │
                           ▼
                ┌──────────────────────┐
                │   ClickHouse DB      │
                │   (Long-term storage)│
                └──────────────────────┘
                           │
                           ▼
                ┌──────────────────────┐
                │   Web Dashboard      │
                │   catops.app         │
                └──────────────────────┘
```

### Data Collection Flow

1. **CatOps Pod (DaemonSet on each node)**:
   - Collects node metrics via Kubernetes API
   - Queries metrics-server for pod CPU/memory
   - Queries Prometheus (if enabled) for extended metrics
   - Aggregates data every 60 seconds (configurable)

2. **Prometheus Query** (if enabled):
   - Fetches pod labels from kube-state-metrics
   - Fetches owner info (Deployment/StatefulSet names)
   - Fetches container details (images, status)
   - Enriches pod data with metadata

3. **Data Transmission**:
   - CatOps pod sends metrics to `api.catops.io/api/kubernetes/*`
   - Authenticated with `auth_token` (from `auth.token`)
   - Metrics stored in ClickHouse database
   - Available immediately in web dashboard

4. **Dashboard View**:
   - Real-time metrics displayed at catops.app
   - Historical data available for trends
   - Nodes appear with ☸️ icon
   - Labeled "Kubernetes Node"

### Metrics Collection Intervals

- **Node metrics**: Every 60 seconds (configurable)
- **Pod metrics**: Every 60 seconds (configurable)
- **Cluster overview**: Every 60 seconds
- **Prometheus queries**: On-demand during collection

---

## ClickHouse Queries

CatOps stores Kubernetes metrics in ClickHouse for long-term analysis. Here are useful queries:

### Basic Queries

**View recent node metrics:**
```sql
SELECT
    formatDateTime(timestamp, '%Y-%m-%d %H:%M:%S') as time,
    node_name,
    cpu_usage,
    memory_usage,
    disk_usage,
    pod_count
FROM k8s_node_metrics
ORDER BY timestamp DESC
LIMIT 10;
```

**View recent pod metrics:**
```sql
SELECT
    formatDateTime(timestamp, '%Y-%m-%d %H:%M:%S') as time,
    pod_name,
    namespace,
    phase,
    cpu_usage_cores,
    memory_usage_mb,
    restart_count
FROM k8s_pod_metrics
ORDER BY timestamp DESC
LIMIT 10;
```

**View cluster health:**
```sql
SELECT
    formatDateTime(timestamp, '%Y-%m-%d %H:%M:%S') as time,
    total_nodes,
    ready_nodes,
    total_pods,
    running_pods,
    pending_pods,
    failed_pods,
    cluster_health_percentage
FROM k8s_cluster_metrics
ORDER BY timestamp DESC
LIMIT 5;
```

### Extended Metrics Queries (with Prometheus)

**View pods with labels and owner information:**
```sql
SELECT
    formatDateTime(timestamp, '%Y-%m-%d %H:%M:%S') as time,
    pod_name,
    namespace,
    phase,
    length(labels) as labels_count,
    owner_kind,
    owner_name,
    pod_age_seconds,
    round(pod_age_seconds / 3600, 2) as pod_age_hours
FROM k8s_pod_metrics
WHERE timestamp > now() - INTERVAL 5 MINUTE
ORDER BY timestamp DESC
LIMIT 20;
```

**Count pods by owner type:**
```sql
SELECT
    owner_kind,
    count() as pod_count,
    avg(cpu_usage_cores) as avg_cpu,
    avg(memory_usage_mb) as avg_memory
FROM k8s_pod_metrics
WHERE timestamp > now() - INTERVAL 5 MINUTE
GROUP BY owner_kind
ORDER BY pod_count DESC;
```

**Find pods with specific label:**
```sql
SELECT
    pod_name,
    namespace,
    labels,
    owner_name,
    cpu_usage_cores,
    memory_usage_mb
FROM k8s_pod_metrics
WHERE JSONExtractString(labels, 'app') = 'nginx'
  AND timestamp > now() - INTERVAL 5 MINUTE
ORDER BY timestamp DESC;
```

**Pod age statistics by namespace:**
```sql
SELECT
    namespace,
    count() as pod_count,
    round(avg(pod_age_seconds) / 3600, 2) as avg_age_hours,
    round(max(pod_age_seconds) / 3600, 2) as max_age_hours,
    round(min(pod_age_seconds) / 3600, 2) as min_age_hours
FROM k8s_pod_metrics
WHERE timestamp > now() - INTERVAL 1 HOUR
GROUP BY namespace
ORDER BY avg_age_hours DESC;
```

**Top pods by restart count:**
```sql
SELECT
    pod_name,
    namespace,
    restart_count,
    owner_kind,
    owner_name,
    phase
FROM k8s_pod_metrics
WHERE timestamp > now() - INTERVAL 5 MINUTE
  AND restart_count > 0
ORDER BY restart_count DESC
LIMIT 20;
```

**Container image usage:**
```sql
SELECT
    JSONExtractString(arrayJoin(JSONExtractArrayRaw(containers)), 'image') as image,
    count() as usage_count
FROM k8s_pod_metrics
WHERE timestamp > now() - INTERVAL 1 HOUR
  AND containers != '[]'
GROUP BY image
ORDER BY usage_count DESC
LIMIT 20;
```

---

## Performance Tuning

### Reduce Resource Usage

**1. Disable Prometheus** (saves ~500 MB RAM):
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=false
```

**2. Reduce Collection Frequency** (saves ~40% CPU):
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set collection.interval=120  # 2 minutes instead of 1
```

**3. Lower Resource Limits** (for small clusters):
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set resources.limits.memory=128Mi \
  --set resources.requests.memory=64Mi
```

**4. Run on Specific Nodes Only:**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set nodeSelector.monitoring=enabled

# Label only nodes you want to monitor
kubectl label nodes node1 monitoring=enabled
```

### Monitor Resource Usage

```bash
# Check CatOps pod resource usage
kubectl top pods -n catops-system

# Check detailed resource stats
kubectl describe pod -n catops-system -l app.kubernetes.io/name=catops
```

---

## Security & RBAC

### RBAC Permissions

CatOps requires minimal read-only permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: catops
rules:
  # Node metrics
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list"]

  # Pod metrics
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]

  # Metrics API
  - apiGroups: ["metrics.k8s.io"]
    resources: ["nodes", "pods"]
    verbs: ["get", "list"]
```

**Security Features:**
- ✅ Read-only access (no modifications)
- ✅ Non-root user (UID 65534)
- ✅ Read-only root filesystem
- ✅ No privilege escalation
- ✅ All capabilities dropped

### Pod Security Context

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534  # nobody user
  fsGroup: 65534

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

### Network Policy (Optional)

Restrict CatOps network access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: catops-network-policy
  namespace: catops-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: catops
  policyTypes:
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow Kubernetes API
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443
  # Allow CatOps backend
  - to:
    - podSelector: {}
    ports:
    - protocol: TCP
      port: 443
```

---

## Version Compatibility

- **v0.2.0-v0.2.1**: Basic metrics only (no Prometheus)
- **v0.2.2-v0.2.5**: Prometheus integration support
- **v0.2.6**: Fixed image tag issues and better defaults
- **v0.2.7** (current): Added initContainer to wait for Prometheus startup

**Backward Compatibility**: Old CLI agents (v0.2.1) continue to work with the backend - extended fields are optional.

---

## Support

For advanced technical support:
- 📧 Email: me@thehonley.org
- 🐛 GitHub Issues: [github.com/mfhonley/catops/issues](https://github.com/mfhonley/catops/issues)
- 💬 Discussions: [github.com/mfhonley/catops/discussions](https://github.com/mfhonley/catops/discussions)
