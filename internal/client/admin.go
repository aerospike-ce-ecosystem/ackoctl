package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Admin endpoints front cluster-manager's Aerospike Admin API surface
// (``/admin/{conn_id}/users`` and ``/admin/{conn_id}/roles``). These rely on
// Aerospike security being enabled in ``aerospike.conf``; CE does not ship
// the security module, so calls against a CE target return a 5xx with
// "security not enabled". The CLI still ships because cluster-manager can be
// pointed at Enterprise clusters via the same workspace.

// ListAdminUsers returns every Aerospike user visible to the operator
// credentials configured on the connection profile.
func (c *BaseClient) ListAdminUsers(ctx context.Context, connID string) ([]AerospikeUser, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out []AerospikeUser
	path := "/admin/" + url.PathEscape(connID) + "/users"
	if err := c.Do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAdminUser creates a new Aerospike user. The server returns 201 with
// the created user echoed back. ``roles`` may be nil — the wire body omits
// the field so the server defers to its default ("no roles").
func (c *BaseClient) CreateAdminUser(ctx context.Context, connID string, req CreateUserRequest) (*AerospikeUser, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	var out AerospikeUser
	path := "/admin/" + url.PathEscape(connID) + "/users"
	if err := c.Do(ctx, http.MethodPost, path, req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChangeAdminUserPassword swaps the password for an existing user. The
// PATCH endpoint is password-only — it does not mutate roles or quotas.
func (c *BaseClient) ChangeAdminUserPassword(ctx context.Context, connID string, req ChangePasswordRequest) (*MessageResponse, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}
	var out MessageResponse
	path := "/admin/" + url.PathEscape(connID) + "/users"
	if err := c.Do(ctx, http.MethodPatch, path, req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAdminUser drops a user. The endpoint returns 204 on success.
// ``username`` is sent as a query parameter to match the server contract
// (``username: str = Query(..., min_length=1)``).
func (c *BaseClient) DeleteAdminUser(ctx context.Context, connID, username string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}
	q := url.Values{}
	q.Set("username", username)
	path := "/admin/" + url.PathEscape(connID) + "/users"
	return c.Do(ctx, http.MethodDelete, path, nil, q, nil)
}

// ListAdminRoles returns every Aerospike role and its privilege list.
func (c *BaseClient) ListAdminRoles(ctx context.Context, connID string) ([]AerospikeRole, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	var out []AerospikeRole
	path := "/admin/" + url.PathEscape(connID) + "/roles"
	if err := c.Do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAdminRole creates a new Aerospike role. The server validates
// privilege codes against the canonical name table and returns 422 for
// unknown codes — that detail surfaces through APIError.
func (c *BaseClient) CreateAdminRole(ctx context.Context, connID string, req CreateRoleRequest) (*AerospikeRole, error) {
	if connID == "" {
		return nil, fmt.Errorf("connID is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(req.Privileges) == 0 {
		return nil, fmt.Errorf("at least one privilege is required")
	}
	var out AerospikeRole
	path := "/admin/" + url.PathEscape(connID) + "/roles"
	if err := c.Do(ctx, http.MethodPost, path, req, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAdminRole drops a role. The endpoint returns 204 on success and
// surfaces the role name as a query parameter (``name: str = Query(...)``).
func (c *BaseClient) DeleteAdminRole(ctx context.Context, connID, name string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	q := url.Values{}
	q.Set("name", name)
	path := "/admin/" + url.PathEscape(connID) + "/roles"
	return c.Do(ctx, http.MethodDelete, path, nil, q, nil)
}
