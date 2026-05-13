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

// K8sClusterListResponse mirrors cluster-manager's wrapper for
// GET /k8s/clusters: { "items": [ ... ] }. The CLI flattens it back to
// a plain slice for callers.
type K8sClusterListResponse struct {
	Items []K8sCluster `json:"items"`
}

// K8sClusterEvent mirrors cluster-manager's K8sClusterEvent. Captures the
// fields ackoctl renders or filters on. Timestamps are RFC3339 strings as
// produced by the Kubernetes API.
type K8sClusterEvent struct {
	Type           string `json:"type,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Message        string `json:"message,omitempty"`
	Count          int    `json:"count,omitempty"`
	FirstTimestamp string `json:"firstTimestamp,omitempty"`
	LastTimestamp  string `json:"lastTimestamp,omitempty"`
	Source         string `json:"source,omitempty"`
	Category       string `json:"category,omitempty"`
}

// K8sLogsOptions carries the optional query parameters for
// GetK8sPodLogs. Zero values mean "not set": Tail==0 uses the server default,
// empty Container omits the param, SinceSeconds==0 omits the param.
type K8sLogsOptions struct {
	Container    string
	Tail         int
	SinceSeconds int
}

// K8sPodLogs mirrors the cluster-manager response envelope from
// GET /k8s/clusters/{ns}/{name}/pods/{pod}/logs. Logs is the raw concatenated
// log string; SinceSeconds is a pointer so callers can distinguish
// "server omitted this field" (nil) from "server returned 0" (rare but
// technically possible if the server ever lowers the floor).
type K8sPodLogs struct {
	Pod          string `json:"pod"`
	Logs         string `json:"logs"`
	TailLines    int    `json:"tailLines"`
	SinceSeconds *int   `json:"sinceSeconds,omitempty"`
}

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

// QueryPredicate mirrors cluster-manager's QueryPredicate.
// Operator: equals | between | contains | geo_within_region | geo_contains_point.
type QueryPredicate struct {
	Bin      string `json:"bin"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
	Value2   any    `json:"value2,omitempty"`
}

// QueryRequest mirrors cluster-manager's QueryRequest. Either Predicate or
// PrimaryKey is typically supplied; an empty request triggers a full scan.
type QueryRequest struct {
	Namespace  string          `json:"namespace"`
	Set        string          `json:"set,omitempty"`
	Predicate  *QueryPredicate `json:"predicate,omitempty"`
	SelectBins []string        `json:"selectBins,omitempty"`
	Expression string          `json:"expression,omitempty"`
	MaxRecords int             `json:"maxRecords,omitempty"`
	PrimaryKey string          `json:"primaryKey,omitempty"`
	PKType     string          `json:"pkType,omitempty"`
}

// QueryResponse mirrors cluster-manager's QueryResponse.
type QueryResponse struct {
	Records         []AerospikeRecord `json:"records"`
	ExecutionTimeMs int               `json:"executionTimeMs"`
	ScannedRecords  int               `json:"scannedRecords"`
	ReturnedRecords int               `json:"returnedRecords"`
}

// SecondaryIndex mirrors cluster-manager's SecondaryIndex.
// Type: numeric | string | geo2dsphere. State: ready | building | error.
type SecondaryIndex struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Set       string `json:"set"`
	Bin       string `json:"bin"`
	Type      string `json:"type"`
	State     string `json:"state"`
}

// CreateIndexRequest mirrors CreateIndexRequest.
type CreateIndexRequest struct {
	Namespace string `json:"namespace"`
	Set       string `json:"set"`
	Bin       string `json:"bin"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

// SetNote mirrors cluster-manager's SetNote — a free-text operator memo
// attached to (connectionId, namespace, setName). Notes live in
// cluster-manager's metaDB, not in Aerospike. ``updatedBy`` is the OIDC
// ``sub`` of the most recent writer, or empty when running under bearer
// token / anonymous auth.
type SetNote struct {
	ConnectionID string `json:"connectionId"`
	Namespace    string `json:"namespace"`
	SetName      string `json:"setName"`
	Note         string `json:"note"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	UpdatedBy    string `json:"updatedBy,omitempty"`
}

// RecordNote mirrors cluster-manager's RecordNote — adds the primary key
// fields to SetNote. The persisted ``pkType`` is the resolved type
// (``string|int|bytes``); ``auto`` is a request-time hint only.
// ``digestHex`` is verification-only — derived from (set, pk), never used
// as a join key.
type RecordNote struct {
	ConnectionID string `json:"connectionId"`
	Namespace    string `json:"namespace"`
	SetName      string `json:"setName"`
	PKText       string `json:"pkText"`
	PKType       string `json:"pkType"`
	DigestHex    string `json:"digestHex,omitempty"`
	Note         string `json:"note"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	UpdatedBy    string `json:"updatedBy,omitempty"`
}

// UpsertSetNoteRequest mirrors cluster-manager's UpsertSetNoteRequest body
// for ``PUT /notes/sets/...``. The server rejects empty / whitespace-only
// notes (``min_length=1``); use the DELETE endpoint to remove.
type UpsertSetNoteRequest struct {
	Note string `json:"note"`
}

// UpsertRecordNoteRequest mirrors cluster-manager's UpsertRecordNoteRequest
// body for ``PUT /notes/records/...``. ``PKType`` defaults to ``auto`` on
// the server when omitted; we pass it through verbatim. The wire key is the
// canonical Pydantic alias ``pk_type`` so this keeps working if the server
// disables ``populate_by_name`` (Pydantic v3 default).
type UpsertRecordNoteRequest struct {
	Note   string `json:"note"`
	PKType string `json:"pk_type,omitempty"`
}

// SetNotesListResponse mirrors the {"notes": [...]} envelope returned by
// ``GET /notes/sets/{conn_id}``.
type SetNotesListResponse struct {
	Notes []SetNote `json:"notes"`
}

// RecordNotesListResponse mirrors the {"notes": [...]} envelope returned by
// ``GET /notes/records/{conn_id}``.
type RecordNotesListResponse struct {
	Notes []RecordNote `json:"notes"`
}

// UDFModule mirrors cluster-manager's UDFModule from
// ``aerospike_cluster_manager_api.models.udf``. Type is fixed to ``"LUA"``
// today (the only language Aerospike CE supports). ``Content`` is only set on
// the upload response when the post-upload re-fetch can't find the module,
// otherwise it is omitted.
type UDFModule struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	Hash     string `json:"hash"`
	Content  string `json:"content,omitempty"`
}

// UploadUDFRequest mirrors cluster-manager's UploadUDFRequest body for
// ``POST /udfs/{conn_id}``. ``Content`` is the raw Lua source as a JSON
// string — the server writes it to disk and registers via aerospike-py's
// ``udf_put``. ``Filename`` is validated server-side against
// ``^[a-zA-Z0-9_.-]{1,255}$``.
type UploadUDFRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// AerospikeUser mirrors cluster-manager's AerospikeUser. Returned by
// ``GET /admin/{conn_id}/users``. Quota and connection counters are pointers
// so we can distinguish "server omitted the field" (older builds) from
// "explicit zero". Older Pydantic models always serialised the integer, but
// the optional shape keeps ackoctl resilient if that ever changes.
type AerospikeUser struct {
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	ReadQuota   *int     `json:"readQuota,omitempty"`
	WriteQuota  *int     `json:"writeQuota,omitempty"`
	Connections *int     `json:"connections,omitempty"`
}

// CreateUserRequest mirrors cluster-manager's CreateUserRequest body for
// ``POST /admin/{conn_id}/users``. ``Roles`` is optional and omitted from
// the wire when nil to match the Pydantic ``list[str] | None`` field.
type CreateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Roles    []string `json:"roles,omitempty"`
}

// ChangePasswordRequest mirrors cluster-manager's ChangePasswordRequest body
// for ``PATCH /admin/{conn_id}/users``. The PATCH endpoint is password-only;
// it does not mutate roles or quotas.
type ChangePasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RolePrivilege mirrors cluster-manager's Privilege model. ``Namespace`` and
// ``Set`` are omitted from the wire when empty to match the
// ``str | None`` shape — the server treats absent and null identically.
type RolePrivilege struct {
	Code      string `json:"code"`
	Namespace string `json:"namespace,omitempty"`
	Set       string `json:"set,omitempty"`
}

// AerospikeRole mirrors cluster-manager's AerospikeRole. Returned by
// ``GET /admin/{conn_id}/roles``. Quotas are pointers for the same reason
// as AerospikeUser.
type AerospikeRole struct {
	Name       string          `json:"name"`
	Privileges []RolePrivilege `json:"privileges"`
	Whitelist  []string        `json:"whitelist,omitempty"`
	ReadQuota  *int            `json:"readQuota,omitempty"`
	WriteQuota *int            `json:"writeQuota,omitempty"`
}

// CreateRoleRequest mirrors cluster-manager's CreateRoleRequest body for
// ``POST /admin/{conn_id}/roles``. Quotas and whitelist are nillable so
// unset values are omitted from the JSON body and the server applies its
// own defaults rather than seeing an explicit 0 / empty list.
type CreateRoleRequest struct {
	Name       string          `json:"name"`
	Privileges []RolePrivilege `json:"privileges"`
	Whitelist  []string        `json:"whitelist,omitempty"`
	ReadQuota  *int            `json:"readQuota,omitempty"`
	WriteQuota *int            `json:"writeQuota,omitempty"`
}
