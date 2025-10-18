package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client обертка над Kubernetes clients
type Client struct {
	Clientset        *kubernetes.Clientset
	MetricsClientset *metricsv.Clientset
}

// NewClient создает новый Kubernetes client
// Автоматически определяет: in-cluster или local kubeconfig
func NewClient() (*Client, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Создаем основной clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Создаем metrics clientset
	metricsClientset, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics clientset: %w", err)
	}

	return &Client{
		Clientset:        clientset,
		MetricsClientset: metricsClientset,
	}, nil
}

// getKubeConfig получает Kubernetes config
// Сначала пытается использовать in-cluster config (когда запущен в поде)
// Затем пытается использовать ~/.kube/config (для локальной разработки)
func getKubeConfig() (*rest.Config, error) {
	// 1. Пытаемся использовать in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// 2. Пытаемся использовать kubeconfig из environment variable
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err == nil {
			return config, nil
		}
	}

	// 3. Пытаемся использовать ~/.kube/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// HealthCheck проверяет доступность Kubernetes API
func (c *Client) HealthCheck(ctx context.Context) error {
	// Пытаемся получить версию сервера
	_, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("kubernetes API is not accessible: %w", err)
	}

	// Проверяем доступность metrics API
	_, err = c.MetricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("metrics API is not accessible (is metrics-server installed?): %w", err)
	}

	return nil
}

// UpdateAuthTokenSecret обновляет Secret с permanent token
// Используется для сохранения permanent token после первого запроса к backend
func (c *Client) UpdateAuthTokenSecret(ctx context.Context, namespace, secretName, permanentToken string) error {
	// Get existing secret
	secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Update auth-token field
	if secret.StringData == nil {
		secret.StringData = make(map[string]string)
	}
	secret.StringData["auth-token"] = permanentToken

	// Update secret in Kubernetes
	_, err = c.Clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret %s/%s: %w", namespace, secretName, err)
	}

	return nil
}
