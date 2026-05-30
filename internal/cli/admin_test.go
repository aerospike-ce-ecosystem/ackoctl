package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
)

// runAdminCmd mirrors runNoteCmd: builds a root command, points it at the
// httptest server via env, and forces JSON output unless the test overrides
// it. “stdinBody“ is fed into “cmd.InOrStdin()“ so --password-stdin paths
// can be exercised without touching the real process stdin.
func runAdminCmd(t *testing.T, srvURL, stdinBody string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	if stdinBody != "" {
		root.SetIn(strings.NewReader(stdinBody))
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srvURL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs(append([]string{"--output", "json"}, args...))
	root.SetContext(context.Background())
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// ---------------------------------------------------------------------------
// privilege parsing
// ---------------------------------------------------------------------------

func TestParsePrivilegesCodeOnly(t *testing.T) {
	got, err := parsePrivileges([]string{"read"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, client.RolePrivilege{Code: "read"}, got[0])
}

func TestParsePrivilegesCodeNamespace(t *testing.T) {
	got, err := parsePrivileges([]string{"read:test"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, client.RolePrivilege{Code: "read", Namespace: "test"}, got[0])
}

func TestParsePrivilegesCodeNamespaceSet(t *testing.T) {
	got, err := parsePrivileges([]string{"read:test/users"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, client.RolePrivilege{Code: "read", Namespace: "test", Set: "users"}, got[0])
}

func TestParsePrivilegesMultiple(t *testing.T) {
	got, err := parsePrivileges([]string{"read", "write:test", "read-write:test/users"})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "read", got[0].Code)
	assert.Equal(t, "write", got[1].Code)
	assert.Equal(t, "test", got[1].Namespace)
	assert.Equal(t, "read-write", got[2].Code)
	assert.Equal(t, "users", got[2].Set)
}

func TestParsePrivilegesRejectsEmptySpec(t *testing.T) {
	_, err := parsePrivileges([]string{""})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsEmptyCode(t *testing.T) {
	_, err := parsePrivileges([]string{":test"})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsEmptyNamespace(t *testing.T) {
	_, err := parsePrivileges([]string{"read:"})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsEmptySet(t *testing.T) {
	_, err := parsePrivileges([]string{"read:test/"})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsDoubleColon(t *testing.T) {
	// "read::" must not silently produce namespace = ":" — the second colon
	// is malformed and should surface a clear CLI error.
	_, err := parsePrivileges([]string{"read::"})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsColonInNamespace(t *testing.T) {
	// "read:a:b" likewise must not produce namespace = "a:b".
	_, err := parsePrivileges([]string{"read:a:b"})
	require.Error(t, err)
}

func TestParsePrivilegesRejectsSlashInSet(t *testing.T) {
	// An extra '/' in the set section must not silently fold into the set
	// name. strings.Cut(rest, "/") keeps everything after the first '/' as
	// the set, so these previously produced a malformed set such as
	// "set/extra" that Aerospike can never have. Reject them client-side,
	// mirroring the namespace ':'/'/' guard.
	for _, spec := range []string{
		"read:ns/set/extra",
		"read:ns/set/",
		"read:ns/a/b/c",
	} {
		_, err := parsePrivileges([]string{spec})
		require.Error(t, err, "spec %q should be rejected", spec)
	}
}

// ---------------------------------------------------------------------------
// resolvePassword
// ---------------------------------------------------------------------------

func TestResolvePasswordExplicit(t *testing.T) {
	pw, err := resolvePassword(strings.NewReader(""), "secret", false)
	require.NoError(t, err)
	assert.Equal(t, "secret", pw)
}

func TestResolvePasswordStdin(t *testing.T) {
	pw, err := resolvePassword(strings.NewReader("from-stdin\n"), "", true)
	require.NoError(t, err)
	assert.Equal(t, "from-stdin", pw)
}

func TestResolvePasswordStdinStripsCRLF(t *testing.T) {
	pw, err := resolvePassword(strings.NewReader("from-stdin\r\n"), "", true)
	require.NoError(t, err)
	assert.Equal(t, "from-stdin", pw)
}

func TestResolvePasswordRejectsBothModes(t *testing.T) {
	_, err := resolvePassword(strings.NewReader("x"), "explicit", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestResolvePasswordRequiresOneMode(t *testing.T) {
	_, err := resolvePassword(strings.NewReader(""), "", false)
	require.Error(t, err)
}

func TestResolvePasswordStdinEmptyRejected(t *testing.T) {
	_, err := resolvePassword(strings.NewReader("\n"), "", true)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// admin user CLI
// ---------------------------------------------------------------------------

func TestAdminUserListRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"username":"alice","roles":["read"],"readQuota":0,"writeQuota":0,"connections":1}]`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runAdminCmd(t, srv.URL, "", "admin", "user", "list", "conn-1")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"username": "alice"`)
}

func TestAdminUserCreateUsesExplicitPassword(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"username":"alice","roles":["read"],"readQuota":0,"writeQuota":0,"connections":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "user", "create", "conn-1",
		"--username", "alice", "--password", "s3cret", "--roles", "read",
	)
	require.NoError(t, err)
	assert.Equal(t, "alice", body["username"])
	assert.Equal(t, "s3cret", body["password"])
	assert.Equal(t, []any{"read"}, body["roles"])
}

func TestAdminUserCreateStripsBlankRoles(t *testing.T) {
	// A trailing comma in --roles (read,) splits into ["read", ""] via cobra's
	// StringSliceVar. The empty entry must be dropped before the request lands
	// so the server never sees a meaningless empty role name.
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"username":"alice","roles":["read","write"],"readQuota":0,"writeQuota":0,"connections":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "user", "create", "conn-1",
		"--username", "alice", "--password", "s3cret",
		"--roles", "read, ,write,",
	)
	require.NoError(t, err)
	assert.Equal(t, []any{"read", "write"}, body["roles"])
}

func TestAdminUserCreateReadsPasswordFromStdin(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"username":"alice","roles":[],"readQuota":0,"writeQuota":0,"connections":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "piped-secret\n",
		"admin", "user", "create", "conn-1",
		"--username", "alice", "--password-stdin",
	)
	require.NoError(t, err)
	assert.Equal(t, "piped-secret", body["password"])
}

func TestAdminUserCreateRejectsBothPasswordModes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit when password flags conflict")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "x\n",
		"admin", "user", "create", "conn-1",
		"--username", "alice", "--password", "p", "--password-stdin",
	)
	require.Error(t, err)
	// Cobra's MarkFlagsMutuallyExclusive emits "none of the others can be"
	// when both flags are set; resolvePassword's fallback says "mutually
	// exclusive". Accept either so the assertion survives the cobra-level
	// guard added on top of the runtime check.
	assert.Regexp(t, "mutually exclusive|none of the others can be", err.Error())
}

func TestAdminUserCreateRequiresUsername(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit on missing --username")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "", "admin", "user", "create", "conn-1", "--password", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestAdminUserPasswdRoundTrip(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Password updated"}`))
	}))
	t.Cleanup(srv.Close)
	stdout, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "user", "passwd", "conn-1",
		"--username", "alice", "--password", "newpass",
	)
	require.NoError(t, err)
	assert.Equal(t, "alice", body["username"])
	assert.Equal(t, "newpass", body["password"])
	assert.Contains(t, stdout, "Password updated")
}

func TestAdminUserDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "user", "delete", "conn-1", "--username", "alice",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestAdminUserDeleteWithYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/admin/conn-1/users", r.URL.Path)
		assert.Equal(t, "alice", r.URL.Query().Get("username"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runAdminCmd(t, srv.URL, "",
		"admin", "user", "delete", "conn-1", "--username", "alice", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted user alice")
}

// ---------------------------------------------------------------------------
// admin role CLI
// ---------------------------------------------------------------------------

func TestAdminRoleListRendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/admin/conn-1/roles", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"app-read","privileges":[{"code":"read","namespace":"test","set":"users"}],"whitelist":["10.0.0.0/8"],"readQuota":0,"writeQuota":0}]`))
	}))
	t.Cleanup(srv.Close)

	// Default output is table — leave --output off to exercise the table
	// renderer including the privilege-formatting helper.
	root := NewRootCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ACKOCTL_SERVER", srv.URL)
	t.Setenv("ACKOCTL_TOKEN", "test-token")
	root.SetArgs([]string{"admin", "role", "list", "conn-1"})
	root.SetContext(context.Background())
	require.NoError(t, root.Execute())
	out := stdout.String()
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "app-read")
	assert.Contains(t, out, "read:test/users")
	assert.Contains(t, out, "10.0.0.0/8")
}

func TestAdminRoleCreateSendsParsedPrivileges(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"app-rw","privileges":[{"code":"read","namespace":"test"},{"code":"write","namespace":"test","set":"users"}],"whitelist":["10.0.0.0/8"],"readQuota":100,"writeQuota":50}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "create", "conn-1",
		"--name", "app-rw",
		"--privilege", "read:test",
		"--privilege", "write:test/users",
		"--whitelist", "10.0.0.0/8",
		"--read-quota", "100",
		"--write-quota", "50",
	)
	require.NoError(t, err)
	assert.Equal(t, "app-rw", body["name"])
	privs := body["privileges"].([]any)
	require.Len(t, privs, 2)
	p1 := privs[1].(map[string]any)
	assert.Equal(t, "write", p1["code"])
	assert.Equal(t, "test", p1["namespace"])
	assert.Equal(t, "users", p1["set"])
	assert.Equal(t, float64(100), body["readQuota"])
	assert.Equal(t, float64(50), body["writeQuota"])
}

func TestAdminRoleCreateStripsBlankWhitelist(t *testing.T) {
	// A trailing comma in --whitelist (10.0.0.0/8,) splits into
	// ["10.0.0.0/8", ""] via cobra's StringSliceVar. The empty CIDR must be
	// dropped before the request lands so the server never sees a blank entry.
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"r","privileges":[{"code":"read"}],"whitelist":["10.0.0.0/8"],"readQuota":0,"writeQuota":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "create", "conn-1",
		"--name", "r", "--privilege", "read",
		"--whitelist", "10.0.0.0/8, ,192.168.0.0/16,",
	)
	require.NoError(t, err)
	assert.Equal(t, []any{"10.0.0.0/8", "192.168.0.0/16"}, body["whitelist"])
}

func TestAdminRoleCreateOmitsAllBlankWhitelist(t *testing.T) {
	// An all-blank --whitelist collapses to nil, so omitempty drops the field
	// entirely rather than forwarding [""] to the server.
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"r","privileges":[{"code":"read"}],"whitelist":[],"readQuota":0,"writeQuota":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "create", "conn-1",
		"--name", "r", "--privilege", "read",
		"--whitelist", " , ",
	)
	require.NoError(t, err)
	_, hasWL := body["whitelist"]
	assert.False(t, hasWL, "all-blank --whitelist must not serialise")
}

func TestAdminRoleCreateOmitsUnsetQuotas(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"r","privileges":[{"code":"read"}],"whitelist":[],"readQuota":0,"writeQuota":0}`))
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "create", "conn-1",
		"--name", "r", "--privilege", "read",
	)
	require.NoError(t, err)
	_, hasRQ := body["readQuota"]
	_, hasWQ := body["writeQuota"]
	assert.False(t, hasRQ, "unset --read-quota must not serialise")
	assert.False(t, hasWQ, "unset --write-quota must not serialise")
}

func TestAdminRoleCreateRejectsNonPositiveQuotas(t *testing.T) {
	// A zero or negative TPS quota is nonsensical; reject it client-side when
	// explicitly set, mirroring #70's numeric-flag guard.
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"read-quota=0", []string{"--read-quota", "0"}, "--read-quota must be a positive"},
		{"read-quota=-5", []string{"--read-quota", "-5"}, "--read-quota must be a positive"},
		{"write-quota=0", []string{"--write-quota", "0"}, "--write-quota must be a positive"},
		{"write-quota=-1", []string{"--write-quota", "-1"}, "--write-quota must be a positive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("server must not be called when a quota flag is not positive")
			}))
			t.Cleanup(srv.Close)
			args := append([]string{"admin", "role", "create", "conn-1", "--name", "r", "--privilege", "read"}, tc.args...)
			_, _, err := runAdminCmd(t, srv.URL, "", args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestAdminRoleCreateRejectsInvalidPrivilege(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit on client-side parse error")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "create", "conn-1",
		"--name", "r", "--privilege", "read:",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is empty")
}

func TestAdminRoleDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server should not be hit without --yes")
	}))
	t.Cleanup(srv.Close)
	_, _, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "delete", "conn-1", "--name", "app-read",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestAdminRoleDeleteWithYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "app-read", r.URL.Query().Get("name"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	_, stderr, err := runAdminCmd(t, srv.URL, "",
		"admin", "role", "delete", "conn-1", "--name", "app-read", "--yes",
	)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Deleted role app-read")
}
