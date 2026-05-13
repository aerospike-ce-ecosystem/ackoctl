package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *BaseClient) ListK8sClusters(ctx context.Context) ([]K8sCluster, error) {
	var out K8sClusterListResponse
	if err := c.Do(ctx, http.MethodGet, "/k8s/clusters", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *BaseClient) GetK8sCluster(ctx context.Context, namespace, name string) (K8sCluster, error) {
	out := K8sCluster{}
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name)
	if err := c.Do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *BaseClient) ForceReconcileK8sCluster(ctx context.Context, namespace, name string) (K8sCluster, error) {
	out := K8sCluster{}
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/force-reconcile"
	if err := c.Do(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScaleK8sCluster patches spec.size on the AerospikeCluster CR via
// POST /k8s/clusters/{ns}/{name}/scale. The server validates the CE cap
// (1..8); we mirror the same bound on the CLI side for fast feedback.
func (c *BaseClient) ScaleK8sCluster(ctx context.Context, namespace, name string, size int) (K8sCluster, error) {
	out := K8sCluster{}
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/scale"
	body := map[string]int{"size": size}
	if err := c.Do(ctx, http.MethodPost, path, body, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
