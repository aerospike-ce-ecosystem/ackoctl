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

// ListK8sPods fetches the per-pod status snapshot for an AerospikeCluster CR
// via GET /k8s/clusters/{ns}/{name}/pods. The server returns a bare JSON
// array of K8sPodStatus -- no envelope. Pods that have not yet reported
// nodeId/rackId/configHash etc. surface those fields as omitted on the wire,
// which round-trips through the optional struct fields.
func (c *BaseClient) ListK8sPods(ctx context.Context, namespace, name string) ([]K8sPodStatus, error) {
	var out []K8sPodStatus
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/pods"
	if err := c.Do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
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

// GetK8sPodLogs fetches kubelet logs for a pod owned by an ACKO-managed
// cluster via GET /k8s/clusters/{ns}/{name}/pods/{pod}/logs. The server bounds
// tail to 1..10000 and since_seconds to 1..86400; we leave numeric validation
// to the caller so a CLI flag parser can surface friendly messages. A
// zero-value Tail uses the server default (500); a zero-value Container or
// SinceSeconds omits the corresponding query param.
func (c *BaseClient) GetK8sPodLogs(ctx context.Context, namespace, name, pod string, opts K8sLogsOptions) (K8sPodLogs, error) {
	out := K8sPodLogs{}
	q := url.Values{}
	if opts.Tail > 0 {
		q.Set("tail", strconv.Itoa(opts.Tail))
	}
	if opts.Container != "" {
		q.Set("container", opts.Container)
	}
	if opts.SinceSeconds > 0 {
		q.Set("since_seconds", strconv.Itoa(opts.SinceSeconds))
	}
	path := "/k8s/clusters/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) +
		"/pods/" + url.PathEscape(pod) + "/logs"
	if err := c.Do(ctx, http.MethodGet, path, nil, q, &out); err != nil {
		return K8sPodLogs{}, err
	}
	return out, nil
}
