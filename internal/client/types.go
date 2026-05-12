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

// RecordKey mirrors cluster-manager's RecordKey.
type RecordKey struct {
	Namespace string `json:"namespace"`
	Set       string `json:"set,omitempty"`
	PK        string `json:"pk,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

// RecordMeta mirrors cluster-manager's RecordMeta.
type RecordMeta struct {
	Generation   int    `json:"generation"`
	TTL          int    `json:"ttl"`
	LastUpdateMs *int64 `json:"lastUpdateMs,omitempty"`
}

// AerospikeRecord mirrors cluster-manager's AerospikeRecord.
type AerospikeRecord struct {
	Key  RecordKey      `json:"key"`
	Meta RecordMeta     `json:"meta"`
	Bins map[string]any `json:"bins"`
	Note string         `json:"note,omitempty"`
}

// RecordListResponse mirrors RecordListResponse from cluster-manager.
type RecordListResponse struct {
	Records        []AerospikeRecord `json:"records"`
	Total          int               `json:"total"`
	Page           int               `json:"page"`
	PageSize       int               `json:"pageSize"`
	HasMore        bool              `json:"hasMore"`
	TotalEstimated bool              `json:"totalEstimated"`
}

// RecordWriteRequest mirrors RecordWriteRequest. cluster-manager accepts
// either snake_case or camelCase keys; we send camelCase to match the
// canonical alias.
type RecordWriteRequest struct {
	Key    RecordKey      `json:"key"`
	Bins   map[string]any `json:"bins"`
	TTL    *int           `json:"ttl,omitempty"`
	PKType string         `json:"pkType,omitempty"`
}

// FilteredQueryRequest mirrors the most common fields of the cluster-manager
// FilteredQueryRequest. Filters and Predicate are passed through as raw maps
// because the server's expression DSL is rich and evolves; CLI users supply
// these via a single --filter JSON flag.
type FilteredQueryRequest struct {
	Namespace   string         `json:"namespace"`
	Set         string         `json:"set,omitempty"`
	PrimaryKey  string         `json:"primaryKey,omitempty"`
	PKPattern   string         `json:"pkPattern,omitempty"`
	PKMatchMode string         `json:"pkMatchMode,omitempty"`
	PKType      string         `json:"pkType,omitempty"`
	Page        int            `json:"page,omitempty"`
	PageSize    int            `json:"pageSize,omitempty"`
	MaxRecords  int            `json:"maxRecords,omitempty"`
	SelectBins  []string       `json:"selectBins,omitempty"`
	Filters     map[string]any `json:"filters,omitempty"`
	Predicate   map[string]any `json:"predicate,omitempty"`
}

// FilteredQueryResponse mirrors FilteredQueryResponse from cluster-manager.
type FilteredQueryResponse struct {
	Records         []AerospikeRecord `json:"records"`
	Total           int               `json:"total"`
	Page            int               `json:"page"`
	PageSize        int               `json:"pageSize"`
	HasMore         bool              `json:"hasMore"`
	ExecutionTimeMs int               `json:"executionTimeMs"`
	ScannedRecords  int               `json:"scannedRecords"`
	ReturnedRecords int               `json:"returnedRecords"`
	TotalEstimated  bool              `json:"totalEstimated"`
}
