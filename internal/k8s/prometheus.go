package k8s

import (
	"context"
	"fmt"
	"time"

	"catops/internal/logger"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusClient wraps Prometheus API client for querying metrics
type PrometheusClient struct {
	api      v1.API
	nodeName string
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(prometheusURL, nodeName string) (*PrometheusClient, error) {
	if prometheusURL == "" {
		return nil, fmt.Errorf("prometheus URL is empty")
	}

	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:      v1.NewAPI(client),
		nodeName: nodeName,
	}, nil
}

// ExtendedNodeMetrics contains additional node metrics from Prometheus
type ExtendedNodeMetrics struct {
	CPUPerCore          map[string]float64           `json:"cpu_per_core,omitempty"`
	MemoryBreakdown     *MemoryBreakdown             `json:"memory_breakdown,omitempty"`
	DiskIOPerDevice     map[string]*DiskIO           `json:"disk_io_per_device,omitempty"`
	NetworkPerInterface map[string]*NetworkInterface `json:"network_per_interface,omitempty"`
}

// MemoryBreakdown contains detailed memory information
type MemoryBreakdown struct {
	TotalBytes   int64 `json:"total_bytes"`
	FreeBytes    int64 `json:"free_bytes"`
	BuffersBytes int64 `json:"buffers_bytes"`
	CachedBytes  int64 `json:"cached_bytes"`
}

// DiskIO contains disk I/O metrics
type DiskIO struct {
	ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
	WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
}

// NetworkInterface contains network metrics per interface
type NetworkInterface struct {
	ReceiveBytesPerSec  float64 `json:"receive_bytes_per_sec"`
	TransmitBytesPerSec float64 `json:"transmit_bytes_per_sec"`
}

// ExtendedPodMetrics contains additional pod metrics from Prometheus
type ExtendedPodMetrics struct {
	Labels     map[string]string `json:"labels,omitempty"`
	OwnerKind  string            `json:"owner_kind,omitempty"`
	OwnerName  string            `json:"owner_name,omitempty"`
	Containers []ContainerDetail `json:"containers,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
}

// ContainerDetail contains per-container information
type ContainerDetail struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
}

// QueryNodeMetrics queries enhanced node metrics from Prometheus
func (p *PrometheusClient) QueryNodeMetrics(ctx context.Context) (*ExtendedNodeMetrics, error) {
	extended := &ExtendedNodeMetrics{
		CPUPerCore:          make(map[string]float64),
		DiskIOPerDevice:     make(map[string]*DiskIO),
		NetworkPerInterface: make(map[string]*NetworkInterface),
	}

	// Query CPU per core
	if err := p.queryCPUPerCore(ctx, extended); err != nil {
		logger.Warning("Failed to query CPU per core: %v", err)
	}

	// Query memory breakdown
	if err := p.queryMemoryBreakdown(ctx, extended); err != nil {
		logger.Warning("Failed to query memory breakdown: %v", err)
	}

	// Query disk I/O
	if err := p.queryDiskIO(ctx, extended); err != nil {
		logger.Warning("Failed to query disk I/O: %v", err)
	}

	// Query network interfaces
	if err := p.queryNetwork(ctx, extended); err != nil {
		logger.Warning("Failed to query network: %v", err)
	}

	return extended, nil
}

// queryCPUPerCore queries CPU usage per core
func (p *PrometheusClient) queryCPUPerCore(ctx context.Context, extended *ExtendedNodeMetrics) error {
	query := fmt.Sprintf(`avg by (cpu) (rate(node_cpu_seconds_total{instance=~".*%s.*", mode!="idle"}[5m]))`, p.nodeName)

	result, warnings, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		logger.Warning("Prometheus warnings: %v", warnings)
	}

	if vector, ok := result.(model.Vector); ok {
		for _, sample := range vector {
			cpu := string(sample.Metric["cpu"])
			usage := float64(sample.Value)
			extended.CPUPerCore[cpu] = usage * 100 // Convert to percentage
		}
	}

	return nil
}

// queryMemoryBreakdown queries detailed memory information
func (p *PrometheusClient) queryMemoryBreakdown(ctx context.Context, extended *ExtendedNodeMetrics) error {
	queries := map[string]string{
		"total":   fmt.Sprintf(`node_memory_MemTotal_bytes{instance=~".*%s.*"}`, p.nodeName),
		"free":    fmt.Sprintf(`node_memory_MemFree_bytes{instance=~".*%s.*"}`, p.nodeName),
		"buffers": fmt.Sprintf(`node_memory_Buffers_bytes{instance=~".*%s.*"}`, p.nodeName),
		"cached":  fmt.Sprintf(`node_memory_Cached_bytes{instance=~".*%s.*"}`, p.nodeName),
	}

	breakdown := &MemoryBreakdown{}

	for metric, query := range queries {
		result, _, err := p.api.Query(ctx, query, time.Now())
		if err != nil {
			logger.Warning("Failed to query memory %s: %v", metric, err)
			continue
		}

		if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
			value := int64(vector[0].Value)
			switch metric {
			case "total":
				breakdown.TotalBytes = value
			case "free":
				breakdown.FreeBytes = value
			case "buffers":
				breakdown.BuffersBytes = value
			case "cached":
				breakdown.CachedBytes = value
			}
		}
	}

	if breakdown.TotalBytes > 0 {
		extended.MemoryBreakdown = breakdown
	}

	return nil
}

// queryDiskIO queries disk I/O per device
func (p *PrometheusClient) queryDiskIO(ctx context.Context, extended *ExtendedNodeMetrics) error {
	// Read bytes
	readQuery := fmt.Sprintf(`rate(node_disk_read_bytes_total{instance=~".*%s.*"}[5m])`, p.nodeName)
	readResult, _, err := p.api.Query(ctx, readQuery, time.Now())
	if err != nil {
		return err
	}

	// Write bytes
	writeQuery := fmt.Sprintf(`rate(node_disk_written_bytes_total{instance=~".*%s.*"}[5m])`, p.nodeName)
	writeResult, _, err := p.api.Query(ctx, writeQuery, time.Now())
	if err != nil {
		return err
	}

	// Parse read results
	if vector, ok := readResult.(model.Vector); ok {
		for _, sample := range vector {
			device := string(sample.Metric["device"])
			if _, exists := extended.DiskIOPerDevice[device]; !exists {
				extended.DiskIOPerDevice[device] = &DiskIO{}
			}
			extended.DiskIOPerDevice[device].ReadBytesPerSec = float64(sample.Value)
		}
	}

	// Parse write results
	if vector, ok := writeResult.(model.Vector); ok {
		for _, sample := range vector {
			device := string(sample.Metric["device"])
			if _, exists := extended.DiskIOPerDevice[device]; !exists {
				extended.DiskIOPerDevice[device] = &DiskIO{}
			}
			extended.DiskIOPerDevice[device].WriteBytesPerSec = float64(sample.Value)
		}
	}

	return nil
}

// queryNetwork queries network metrics per interface
func (p *PrometheusClient) queryNetwork(ctx context.Context, extended *ExtendedNodeMetrics) error {
	// Receive bytes
	rxQuery := fmt.Sprintf(`rate(node_network_receive_bytes_total{instance=~".*%s.*"}[5m])`, p.nodeName)
	rxResult, _, err := p.api.Query(ctx, rxQuery, time.Now())
	if err != nil {
		return err
	}

	// Transmit bytes
	txQuery := fmt.Sprintf(`rate(node_network_transmit_bytes_total{instance=~".*%s.*"}[5m])`, p.nodeName)
	txResult, _, err := p.api.Query(ctx, txQuery, time.Now())
	if err != nil {
		return err
	}

	// Parse receive results
	if vector, ok := rxResult.(model.Vector); ok {
		for _, sample := range vector {
			iface := string(sample.Metric["device"])
			if _, exists := extended.NetworkPerInterface[iface]; !exists {
				extended.NetworkPerInterface[iface] = &NetworkInterface{}
			}
			extended.NetworkPerInterface[iface].ReceiveBytesPerSec = float64(sample.Value)
		}
	}

	// Parse transmit results
	if vector, ok := txResult.(model.Vector); ok {
		for _, sample := range vector {
			iface := string(sample.Metric["device"])
			if _, exists := extended.NetworkPerInterface[iface]; !exists {
				extended.NetworkPerInterface[iface] = &NetworkInterface{}
			}
			extended.NetworkPerInterface[iface].TransmitBytesPerSec = float64(sample.Value)
		}
	}

	return nil
}

// QueryPodMetrics queries enhanced pod metrics from kube-state-metrics
func (p *PrometheusClient) QueryPodMetrics(ctx context.Context) (map[string]*ExtendedPodMetrics, error) {
	extendedPods := make(map[string]*ExtendedPodMetrics)

	// Query pod labels
	if err := p.queryPodLabels(ctx, extendedPods); err != nil {
		logger.Warning("Failed to query pod labels: %v", err)
	}

	// Query owner references
	if err := p.queryOwnerReferences(ctx, extendedPods); err != nil {
		logger.Warning("Failed to query owner references: %v", err)
	}

	// Query container info
	if err := p.queryContainerInfo(ctx, extendedPods); err != nil {
		logger.Warning("Failed to query container info: %v", err)
	}

	return extendedPods, nil
}

// queryPodLabels queries all pod labels
func (p *PrometheusClient) queryPodLabels(ctx context.Context, extendedPods map[string]*ExtendedPodMetrics) error {
	// Use kube_pod_info instead of kube_pod_labels (which doesn't exist in kube-state-metrics by default)
	query := fmt.Sprintf(`kube_pod_info{node="%s"}`, p.nodeName)

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return err
	}

	if vector, ok := result.(model.Vector); ok {
		for _, sample := range vector {
			pod := string(sample.Metric["pod"])
			namespace := string(sample.Metric["namespace"])
			key := fmt.Sprintf("%s/%s", namespace, pod)

			if _, exists := extendedPods[key]; !exists {
				extendedPods[key] = &ExtendedPodMetrics{
					Labels: make(map[string]string),
				}
			}

			// Extract Kubernetes labels from Prometheus metric labels
			// Common label prefixes: app_, helm_, k8s_, kubernetes_
			labelCount := 0
			for k, v := range sample.Metric {
				labelKey := string(k)
				// Skip internal Prometheus labels and kube_pod_info specific fields
				if labelKey == "__name__" || labelKey == "pod" || labelKey == "namespace" ||
				   labelKey == "node" || labelKey == "host_ip" || labelKey == "pod_ip" ||
				   labelKey == "uid" || labelKey == "created_by_kind" || labelKey == "created_by_name" ||
				   labelKey == "host_network" || labelKey == "instance" || labelKey == "job" || labelKey == "service" {
					continue
				}
				// Store all remaining labels (these are the actual pod labels)
				extendedPods[key].Labels[labelKey] = string(v)
				labelCount++
			}
		}
	}

	return nil
}

// queryOwnerReferences queries pod owner information
func (p *PrometheusClient) queryOwnerReferences(ctx context.Context, extendedPods map[string]*ExtendedPodMetrics) error {
	query := fmt.Sprintf(`kube_pod_owner{node="%s"}`, p.nodeName)

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return err
	}

	if vector, ok := result.(model.Vector); ok {
		for _, sample := range vector {
			pod := string(sample.Metric["pod"])
			namespace := string(sample.Metric["namespace"])
			key := fmt.Sprintf("%s/%s", namespace, pod)

			if _, exists := extendedPods[key]; !exists {
				extendedPods[key] = &ExtendedPodMetrics{
					Labels: make(map[string]string),
				}
			}

			extendedPods[key].OwnerKind = string(sample.Metric["owner_kind"])
			extendedPods[key].OwnerName = string(sample.Metric["owner_name"])
		}
	}

	return nil
}

// queryContainerInfo queries container details
func (p *PrometheusClient) queryContainerInfo(ctx context.Context, extendedPods map[string]*ExtendedPodMetrics) error {
	query := fmt.Sprintf(`kube_pod_container_info{node="%s"}`, p.nodeName)

	result, _, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return err
	}

	if vector, ok := result.(model.Vector); ok {
		for _, sample := range vector {
			pod := string(sample.Metric["pod"])
			namespace := string(sample.Metric["namespace"])
			key := fmt.Sprintf("%s/%s", namespace, pod)

			if _, exists := extendedPods[key]; !exists {
				extendedPods[key] = &ExtendedPodMetrics{
					Labels: make(map[string]string),
				}
			}

			container := ContainerDetail{
				Name:   string(sample.Metric["container"]),
				Image:  string(sample.Metric["image"]),
				Status: "running", // kube_pod_container_info only shows running containers
				Ready:  true,      // Assume ready if it exists in this metric
			}

			extendedPods[key].Containers = append(extendedPods[key].Containers, container)
		}
	}

	return nil
}

// IsAvailable checks if Prometheus is reachable
func (p *PrometheusClient) IsAvailable(ctx context.Context) bool {
	_, _, err := p.api.Query(ctx, "up", time.Now())
	return err == nil
}
