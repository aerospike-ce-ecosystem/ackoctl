package client

// Connection mirrors cluster-manager's ConnectionProfileResponse. Only fields
// ackoctl reads or writes are included; the password is never sent back from
// the server.
type Connection struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Hosts       []string          `json:"hosts"`
	Port        int               `json:"port"`
	ClusterName string            `json:"clusterName,omitempty"`
	Username    string            `json:"username,omitempty"`
	Color       string            `json:"color,omitempty"`
	Note        string            `json:"note,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	WorkspaceID string            `json:"workspaceId,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
}

// CreateConnectionRequest matches cluster-manager's CreateConnectionRequest.
// Pointer fields are used so we can omit unset values from the JSON body.
type CreateConnectionRequest struct {
	Name        string            `json:"name,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"`
	Port        int               `json:"port,omitempty"`
	ClusterName string            `json:"clusterName,omitempty"`
	Username    string            `json:"username,omitempty"`
	Password    string            `json:"password,omitempty"`
	Color       string            `json:"color,omitempty"`
	Note        string            `json:"note,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	WorkspaceID string            `json:"workspaceId,omitempty"`
}

// UpdateConnectionRequest mirrors cluster-manager's UpdateConnectionRequest.
// All fields are optional pointers so callers can patch individual values.
type UpdateConnectionRequest struct {
	Name        *string           `json:"name,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"`
	Port        *int              `json:"port,omitempty"`
	ClusterName *string           `json:"clusterName,omitempty"`
	Username    *string           `json:"username,omitempty"`
	Password    *string           `json:"password,omitempty"`
	Color       *string           `json:"color,omitempty"`
	Note        *string           `json:"note,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	WorkspaceID *string           `json:"workspaceId,omitempty"`
}

// ConnectionStatus mirrors cluster-manager's ConnectionStatus. /health always
// returns 200 — Connected=false on unreachable clusters.
type ConnectionStatus struct {
	Connected       bool   `json:"connected"`
	NodeCount       int    `json:"nodeCount"`
	NamespaceCount  int    `json:"namespaceCount"`
	Build           string `json:"build,omitempty"`
	Edition         string `json:"edition,omitempty"`
	MemoryUsed      int64  `json:"memoryUsed,omitempty"`
	MemoryTotal     int64  `json:"memoryTotal,omitempty"`
	DiskUsed        int64  `json:"diskUsed,omitempty"`
	DiskTotal       int64  `json:"diskTotal,omitempty"`
	TendHealthy     *bool  `json:"tendHealthy,omitempty"`
	Error           string `json:"error,omitempty"`
	ErrorType       string `json:"errorType,omitempty"`
}

// MessageResponse mirrors cluster-manager's MessageResponse used by several
// write endpoints.
type MessageResponse struct {
	Message string `json:"message"`
}

// ClusterInfo and K8sCluster shapes are passed through as raw maps so the CLI
// stays robust against cluster-manager evolving the response. JSON/YAML
// output preserves the server's schema verbatim; table rendering is best
// effort.
type ClusterInfo = map[string]any
type K8sCluster = map[string]any

// ConfigureNamespaceRequest is a minimal contract — the cluster-manager
// CreateNamespaceRequest body accepts a namespace name plus dynamic config
// key/value pairs. We pass it through as a map to avoid drifting against the
// server's evolving knobs.
type ConfigureNamespaceRequest map[string]any
