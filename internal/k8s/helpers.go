package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodMetrics метрики ресурсов пода
type PodMetrics struct {
	CPUUsage    float64 // cores
	MemoryUsage int64   // bytes
}

// GetPodsOnNode получает все поды на указанной ноде
func (c *Client) GetPodsOnNode(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	// Используем field selector для фильтрации по ноде
	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	return pods.Items, nil
}

// GetAllPods получает все поды в кластере
func (c *Client) GetAllPods(ctx context.Context) ([]corev1.Pod, error) {
	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list all pods: %w", err)
	}

	return pods.Items, nil
}

// GetAllNodes получает все ноды в кластере
func (c *Client) GetAllNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return nodes.Items, nil
}

// GetPodMetrics получает метрики ресурсов пода через Metrics API
func (c *Client) GetPodMetrics(ctx context.Context, namespace, podName string) (*PodMetrics, error) {
	podMetrics, err := c.MetricsClientset.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	metrics := &PodMetrics{}

	// Суммируем метрики всех контейнеров в поде
	for _, container := range podMetrics.Containers {
		// CPU usage в миллиядрах -> cores
		cpuMillicores := container.Usage.Cpu().MilliValue()
		metrics.CPUUsage += float64(cpuMillicores) / 1000.0

		// Memory usage в байтах
		memoryBytes := container.Usage.Memory().Value()
		metrics.MemoryUsage += memoryBytes
	}

	return metrics, nil
}

// GetNodeMetrics получает метрики ресурсов ноды через Metrics API
func (c *Client) GetNodeMetrics(ctx context.Context, nodeName string) (*PodMetrics, error) {
	nodeMetrics, err := c.MetricsClientset.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	metrics := &PodMetrics{}

	// CPU usage в миллиядрах -> cores
	cpuMillicores := nodeMetrics.Usage.Cpu().MilliValue()
	metrics.CPUUsage = float64(cpuMillicores) / 1000.0

	// Memory usage в байтах
	metrics.MemoryUsage = nodeMetrics.Usage.Memory().Value()

	return metrics, nil
}
