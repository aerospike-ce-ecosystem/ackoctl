package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
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

// ListK8sClusterEvents fetches Kubernetes events for an AerospikeCluster CR.
// The server returns a bare JSON array (not an envelope), already filtered by
// the involvedObject field selector and capped at limit. The optional
// category filter (e.g. "Scaling", "Lifecycle") is applied server-side.
func (c *BaseClient) ListK8sClusterEvents(ctx context.Context, namespace, name string, limit int, category string) ([]K8sClusterEvent, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if category != "" {
		q.Set("category", category)
	}
	var out []K8sClusterEvent
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/events"
	if err := c.Do(ctx, http.MethodGet, path, nil, q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
