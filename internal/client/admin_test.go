package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAdminUsersDecodesArray(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		    {"username":"admin","roles":["user-admin","sys-admin"],"readQuota":0,"writeQuota":0,"connections":1},
		    {"username":"alice","roles":["read"],"readQuota":100,"writeQuota":50,"connections":0}
		]`))
	})
	users, err := c.ListAdminUsers(context.Background(), "conn-1")
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "admin", users[0].Username)
	assert.Equal(t, []string{"user-admin", "sys-admin"}, users[0].Roles)
	require.NotNil(t, users[1].ReadQuota)
	assert.Equal(t, 100, *users[1].ReadQuota)
}

func TestListAdminUsersRequiresConnID(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when connID is empty")
	})
	_, err := c.ListAdminUsers(context.Background(), "")
	require.Error(t, err)
}

func TestCreateAdminUserSerializesBody(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"username":"alice","roles":["read"],"readQuota":0,"writeQuota":0,"connections":0}`))
	})
	out, err := c.CreateAdminUser(context.Background(), "conn-1", CreateUserRequest{
		Username: "alice", Password: "s3cret", Roles: []string{"read"},
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", body["username"])
	assert.Equal(t, "s3cret", body["password"])
	assert.Equal(t, []any{"read"}, body["roles"])
	assert.Equal(t, "alice", out.Username)
}

func TestCreateAdminUserOmitsEmptyRoles(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"username":"alice","roles":[],"readQuota":0,"writeQuota":0,"connections":0}`))
	})
	_, err := c.CreateAdminUser(context.Background(), "conn-1", CreateUserRequest{
		Username: "alice", Password: "s3cret",
	})
	require.NoError(t, err)
	_, hasRoles := body["roles"]
	assert.False(t, hasRoles, "nil Roles must be omitted from wire body")
}

func TestCreateAdminUserValidatesRequiredFields(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when fields are missing")
	})
	_, err := c.CreateAdminUser(context.Background(), "conn-1", CreateUserRequest{Username: "alice"})
	require.Error(t, err)
	_, err = c.CreateAdminUser(context.Background(), "conn-1", CreateUserRequest{Password: "s3cret"})
	require.Error(t, err)
}

func TestChangeAdminUserPasswordSendsPatch(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Password updated"}`))
	})
	msg, err := c.ChangeAdminUserPassword(context.Background(), "conn-1", ChangePasswordRequest{
		Username: "alice", Password: "newpass",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", body["username"])
	assert.Equal(t, "newpass", body["password"])
	assert.Equal(t, "Password updated", msg.Message)
}

func TestDeleteAdminUserSendsUsernameQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		assert.Equal(t, "alice", r.URL.Query().Get("username"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteAdminUser(context.Background(), "conn-1", "alice"))
}

func TestDeleteAdminUserRequiresUsername(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when username is empty")
	})
	require.Error(t, c.DeleteAdminUser(context.Background(), "conn-1", ""))
}

func TestDeleteAdminUserSurfacesSecurityNotEnabled(t *testing.T) {
	// CE returns 5xx with "security not enabled" — verify the detail bubbles
	// through APIError so the CLI prints the server's message verbatim.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"security not enabled"}`))
	})
	err := c.DeleteAdminUser(context.Background(), "conn-1", "alice")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Contains(t, apiErr.Detail, "security")
}

func TestListAdminRolesDecodesPrivileges(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/admin/conn-1/roles", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		    {"name":"app-read","privileges":[{"code":"read","namespace":"test","set":"users"}],"whitelist":[],"readQuota":0,"writeQuota":0}
		]`))
	})
	roles, err := c.ListAdminRoles(context.Background(), "conn-1")
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, "app-read", roles[0].Name)
	require.Len(t, roles[0].Privileges, 1)
	assert.Equal(t, "read", roles[0].Privileges[0].Code)
	assert.Equal(t, "test", roles[0].Privileges[0].Namespace)
	assert.Equal(t, "users", roles[0].Privileges[0].Set)
}

func TestCreateAdminRoleSerializesPrivilegesAndQuotas(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/roles", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"app-rw","privileges":[{"code":"read-write","namespace":"test"}],"whitelist":["10.0.0.0/8"],"readQuota":100,"writeQuota":50}`))
	})
	rq, wq := 100, 50
	role, err := c.CreateAdminRole(context.Background(), "conn-1", CreateRoleRequest{
		Name: "app-rw",
		Privileges: []RolePrivilege{
			{Code: "read-write", Namespace: "test"},
		},
		Whitelist:  []string{"10.0.0.0/8"},
		ReadQuota:  &rq,
		WriteQuota: &wq,
	})
	require.NoError(t, err)
	assert.Equal(t, "app-rw", body["name"])
	privs := body["privileges"].([]any)
	require.Len(t, privs, 1)
	p0 := privs[0].(map[string]any)
	assert.Equal(t, "read-write", p0["code"])
	assert.Equal(t, "test", p0["namespace"])
	_, hasSet := p0["set"]
	assert.False(t, hasSet, "empty set must be omitted from wire body")
	assert.Equal(t, []any{"10.0.0.0/8"}, body["whitelist"])
	assert.Equal(t, float64(100), body["readQuota"])
	assert.Equal(t, float64(50), body["writeQuota"])
	assert.Equal(t, "app-rw", role.Name)
}

func TestCreateAdminRoleOmitsUnsetQuotasAndWhitelist(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"r","privileges":[{"code":"read"}],"whitelist":[],"readQuota":0,"writeQuota":0}`))
	})
	_, err := c.CreateAdminRole(context.Background(), "conn-1", CreateRoleRequest{
		Name: "r", Privileges: []RolePrivilege{{Code: "read"}},
	})
	require.NoError(t, err)
	_, hasWl := body["whitelist"]
	_, hasRQ := body["readQuota"]
	_, hasWQ := body["writeQuota"]
	assert.False(t, hasWl, "empty whitelist must be omitted")
	assert.False(t, hasRQ, "unset readQuota must be omitted")
	assert.False(t, hasWQ, "unset writeQuota must be omitted")
}

func TestCreateAdminRoleValidates(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit on client-side validation")
	})
	_, err := c.CreateAdminRole(context.Background(), "conn-1", CreateRoleRequest{Privileges: []RolePrivilege{{Code: "read"}}})
	require.Error(t, err)
	_, err = c.CreateAdminRole(context.Background(), "conn-1", CreateRoleRequest{Name: "r"})
	require.Error(t, err)
}

func TestDeleteAdminRoleSendsNameQuery(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/roles", r.URL.Path)
		assert.Equal(t, "app-read", r.URL.Query().Get("name"))
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, c.DeleteAdminRole(context.Background(), "conn-1", "app-read"))
}

func TestDeleteAdminRoleRequiresName(t *testing.T) {
	c, _ := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when name is empty")
	})
	require.Error(t, c.DeleteAdminRole(context.Background(), "conn-1", ""))
}

func TestAdminPathSegmentsAreEscaped(t *testing.T) {
	// Connection IDs may include URL-unsafe characters in workspace-prefixed
	// formats (e.g. "ws-1/conn-1"); make sure we percent-encode.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/admin/ws-1%2Fconn-1/users", r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	_, err := c.ListAdminUsers(context.Background(), "ws-1/conn-1")
	require.NoError(t, err)
}
