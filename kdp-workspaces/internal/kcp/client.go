/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kcp

import (
	"context"
	"fmt"

	kcpclientset "github.com/kcp-dev/sdk/client/clientset/versioned/cluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config holds KCP connection configuration
type Config struct {
	KubeconfigData []byte
	WorkspacePath  string
	WorkspaceType  string
}

// Client wraps the KCP cluster-aware client
type Client struct {
	clusterClient kcpclientset.ClusterInterface
	config        *Config
}

// NewClient creates a new KCP client
func NewClient(config *Config) (*Client, error) {
	log := ctrl.Log.WithName("kcp-client")

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(config.KubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	log.Info("KCP client configuration loaded",
		"serverURL", restConfig.Host,
		"workspacePath", config.WorkspacePath,
		"workspaceType", config.WorkspaceType)

	clusterClient, err := kcpclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP client: %w", err)
	}

	return &Client{
		clusterClient: clusterClient,
		config:        config,
	}, nil
}

// LoadConfigFromCluster loads KCP configuration from Kubernetes ConfigMap and Secret
func LoadConfigFromCluster(ctx context.Context, k8sClient client.Client,
	configMapName, secretName, namespace string) (*Config, error) {

	// Load ConfigMap
	configMap := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      configMapName,
		Namespace: namespace,
	}, configMap); err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configMapName, err)
	}

	// Load Secret
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: namespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get Secret %s/%s: %w", namespace, secretName, err)
	}

	kubeconfigData, ok := secret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig not found in Secret %s/%s", namespace, secretName)
	}

	return &Config{
		KubeconfigData: kubeconfigData,
		WorkspacePath:  configMap.Data["kcp-workspace-path"],
		WorkspaceType:  configMap.Data["workspace-type"],
	}, nil
}

// GetClusterClient returns the underlying cluster-aware client
func (c *Client) GetClusterClient() kcpclientset.ClusterInterface {
	return c.clusterClient
}
