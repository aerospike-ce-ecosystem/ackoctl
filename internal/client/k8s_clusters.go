package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *BaseClient) ListK8sClusters(ctx context.Context) ([]K8sCluster, error) {
	var out []K8sCluster
	if err := c.Do(ctx, http.MethodGet, "/k8s/clusters", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
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
