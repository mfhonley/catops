package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"catops/internal/logger"
	"catops/internal/metrics"
)

// Collector собирает метрики из Kubernetes
type Collector struct {
	client      *Client
	backendURL  string
	authToken   string
	nodeName    string
	namespace   string
	version     string
}

// CollectorConfig конфигурация для Collector
type CollectorConfig struct {
	BackendURL string
	AuthToken  string
	NodeName   string
	Namespace  string
}

// NewCollector создает новый Collector
func NewCollector(client *Client, config interface{}, version string) *Collector {
	// Type assertion для получения конфигурации
	cfg := config.(interface {
		GetBackendURL() string
		GetAuthToken() string
		GetNodeName() string
		GetNamespace() string
	})

	return &Collector{
		client:     client,
		backendURL: cfg.GetBackendURL(),
		authToken:  cfg.GetAuthToken(),
		nodeName:   cfg.GetNodeName(),
		namespace:  cfg.GetNamespace(),
		version:    version,
	}
}

// K8sMetrics метрики Kubernetes
type K8sMetrics struct {
	Timestamp string        `json:"timestamp"`
	NodeName  string        `json:"node_name"`
	Namespace string        `json:"namespace"`

	// Node metrics (переиспользуем существующий код)
	Node *metrics.Metrics `json:"node_metrics"`

	// K8s-specific metrics
	Pods      []PodMetric     `json:"pods"`
	Cluster   *ClusterMetrics `json:"cluster"`

	// JWT token для backend
	UserToken string `json:"user_token"`
}

// PodMetric метрики пода
type PodMetric struct {
	Name          string  `json:"name"`
	Namespace     string  `json:"namespace"`
	PodIP         string  `json:"pod_ip"`
	HostIP        string  `json:"host_ip"`
	Phase         string  `json:"phase"`
	CPUUsage      float64 `json:"cpu_usage_cores"`
	MemoryUsage   int64   `json:"memory_usage_bytes"`
	RestartCount  int32   `json:"restart_count"`
	ContainerCount int    `json:"container_count"`
}

// ClusterMetrics метрики кластера
type ClusterMetrics struct {
	TotalNodes      int `json:"total_nodes"`
	ReadyNodes      int `json:"ready_nodes"`
	TotalPods       int `json:"total_pods"`
	RunningPods     int `json:"running_pods"`
	PendingPods     int `json:"pending_pods"`
	FailedPods      int `json:"failed_pods"`
}

// CollectAndSend собирает метрики и отправляет в backend
func (c *Collector) CollectAndSend(ctx context.Context) error {
	startTime := time.Now()

	logger.Info("📊 Collecting metrics...")

	// 1. Собираем node metrics (переиспользуем существующий код!)
	nodeMetrics, err := c.collectNodeMetrics()
	if err != nil {
		return fmt.Errorf("failed to collect node metrics: %w", err)
	}

	// 2. Собираем pod metrics для текущей ноды
	podMetrics, err := c.collectPodMetrics(ctx)
	if err != nil {
		logger.Warning("Failed to collect pod metrics: %v", err)
		podMetrics = []PodMetric{} // продолжаем с пустым списком
	}

	// 3. Собираем cluster metrics (только с первой ноды, чтобы не дублировать)
	var clusterMetrics *ClusterMetrics
	if c.shouldCollectClusterMetrics() {
		clusterMetrics, err = c.collectClusterMetrics(ctx)
		if err != nil {
			logger.Warning("Failed to collect cluster metrics: %v", err)
		}
	}

	// 4. Собираем всё в одну структуру
	k8sMetrics := &K8sMetrics{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		NodeName:  c.nodeName,
		Namespace: c.namespace,
		Node:      nodeMetrics,
		Pods:      podMetrics,
		Cluster:   clusterMetrics,
		UserToken: c.authToken,
	}

	// 5. Отправляем в backend
	if err := c.sendMetrics(k8sMetrics); err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}

	duration := time.Since(startTime)
	logger.Info("✅ Metrics collected and sent successfully (took %v)", duration)
	logger.Info("   Node metrics: CPU=%.1f%%, Memory=%.1f%%, Disk=%.1f%%",
		nodeMetrics.CPUUsage, nodeMetrics.MemoryUsage, nodeMetrics.DiskUsage)
	logger.Info("   Pods on this node: %d", len(podMetrics))

	if clusterMetrics != nil {
		logger.Info("   Cluster: %d/%d nodes ready, %d/%d pods running",
			clusterMetrics.ReadyNodes, clusterMetrics.TotalNodes,
			clusterMetrics.RunningPods, clusterMetrics.TotalPods)
	}

	fmt.Println()

	return nil
}

// collectNodeMetrics собирает метрики текущей ноды
// ПЕРЕИСПОЛЬЗУЕМ существующий код из cli/internal/metrics!
func (c *Collector) collectNodeMetrics() (*metrics.Metrics, error) {
	// Используем существующую функцию GetMetrics()
	nodeMetrics, err := metrics.GetMetrics()
	if err != nil {
		return nil, err
	}

	return nodeMetrics, nil
}

// collectPodMetrics собирает метрики подов на текущей ноде
func (c *Collector) collectPodMetrics(ctx context.Context) ([]PodMetric, error) {
	pods, err := c.client.GetPodsOnNode(ctx, c.nodeName)
	if err != nil {
		return nil, err
	}

	var podMetrics []PodMetric
	for _, pod := range pods {
		metric := PodMetric{
			Name:           pod.Name,
			Namespace:      pod.Namespace,
			PodIP:          pod.Status.PodIP,
			HostIP:         pod.Status.HostIP,
			Phase:          string(pod.Status.Phase),
			ContainerCount: len(pod.Spec.Containers),
		}

		// Считаем restart count
		for _, containerStatus := range pod.Status.ContainerStatuses {
			metric.RestartCount += containerStatus.RestartCount
		}

		// Получаем resource usage через metrics API
		usage, err := c.client.GetPodMetrics(ctx, pod.Namespace, pod.Name)
		if err == nil && usage != nil {
			metric.CPUUsage = usage.CPUUsage
			metric.MemoryUsage = usage.MemoryUsage
		}

		podMetrics = append(podMetrics, metric)
	}

	return podMetrics, nil
}

// collectClusterMetrics собирает метрики всего кластера
func (c *Collector) collectClusterMetrics(ctx context.Context) (*ClusterMetrics, error) {
	metrics := &ClusterMetrics{}

	// Получаем все ноды
	nodes, err := c.client.GetAllNodes(ctx)
	if err != nil {
		return nil, err
	}

	metrics.TotalNodes = len(nodes)
	for _, node := range nodes {
		// Проверяем статус ноды
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				metrics.ReadyNodes++
				break
			}
		}
	}

	// Получаем все поды
	pods, err := c.client.GetAllPods(ctx)
	if err != nil {
		return nil, err
	}

	metrics.TotalPods = len(pods)
	for _, pod := range pods {
		switch pod.Status.Phase {
		case "Running":
			metrics.RunningPods++
		case "Pending":
			metrics.PendingPods++
		case "Failed":
			metrics.FailedPods++
		}
	}

	return metrics, nil
}

// shouldCollectClusterMetrics определяет, нужно ли собирать cluster metrics
// Собираем только с одной ноды, чтобы не дублировать данные
func (c *Collector) shouldCollectClusterMetrics() bool {
	// Простая стратегия: собираем только если node name лексикографически первый
	// В production можно использовать leader election
	// TODO: implement leader election
	return true // пока собираем со всех (backend должен дедуплицировать)
}

// sendMetrics отправляет метрики в backend
func (c *Collector) sendMetrics(metrics *K8sMetrics) error {
	// Сериализуем в JSON
	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	url := fmt.Sprintf("%s/api/cli/kubernetes/metrics", c.backendURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("User-Agent", "CatOps-CLI/1.0.0")
	req.Header.Set("X-Platform", "linux")
	req.Header.Set("X-Version", c.version)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("backend returned error: %s", resp.Status)
	}

	return nil
}
