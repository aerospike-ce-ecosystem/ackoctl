package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, APIVersion, cfg.APIVersion)
	assert.Equal(t, Kind, cfg.Kind)
	assert.Empty(t, cfg.Contexts)
}

func TestUpsertAndSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	cfg := &Config{}
	cfg.Upsert(Context{Name: "kind-local", Server: "http://localhost:8000/api"})
	cfg.Upsert(Context{Name: "prod", Server: "https://acm.example.com/api", Token: "t"})
	require.NoError(t, cfg.Use("kind-local"))
	require.NoError(t, Save(path, cfg))

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "kind-local", loaded.CurrentContext)
	assert.Len(t, loaded.Contexts, 2)
	prod, _ := loaded.Find("prod")
	require.NotNil(t, prod)
	assert.Equal(t, "t", prod.Token)
}

func TestUpsertReplacesExisting(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Context{Name: "a", Server: "http://1"})
	cfg.Upsert(Context{Name: "a", Server: "http://2", Token: "new"})
	require.Len(t, cfg.Contexts, 1)
	ctx, _ := cfg.Find("a")
	assert.Equal(t, "http://2", ctx.Server)
	assert.Equal(t, "new", ctx.Token)
}

func TestDeleteContextClearsCurrent(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Context{Name: "a", Server: "http://1"})
	require.NoError(t, cfg.Use("a"))
	require.NoError(t, cfg.Delete("a"))
	assert.Empty(t, cfg.CurrentContext)

	err := cfg.Delete("missing")
	assert.ErrorIs(t, err, ErrContextNotFound)
}

func TestCurrentErrors(t *testing.T) {
	cfg := &Config{}
	_, err := cfg.Current()
	assert.ErrorIs(t, err, ErrNoContext)

	cfg.Upsert(Context{Name: "a", Server: "http://1"})
	_, err = cfg.Current()
	assert.ErrorIs(t, err, ErrNoCurrent)
}

func TestResolveFlagBeatsEnvBeatsFile(t *testing.T) {
	cfg := &Config{CurrentContext: "a"}
	cfg.Upsert(Context{Name: "a", Server: "http://file", Token: "file-tok", WorkspaceID: "w-file"})

	ctx, err := Resolve(cfg, Overrides{Server: "http://env", Token: "env-tok"}, Overrides{Token: "flag-tok"})
	require.NoError(t, err)
	assert.Equal(t, "http://env", ctx.Server, "env overrides file")
	assert.Equal(t, "flag-tok", ctx.Token, "flag overrides env")
	assert.Equal(t, "w-file", ctx.WorkspaceID, "file wins when neither flag nor env set")
}

func TestResolveEphemeralContextWhenServerOnly(t *testing.T) {
	cfg := &Config{}
	ctx, err := Resolve(cfg, Overrides{Server: "http://envonly"}, Overrides{})
	require.NoError(t, err)
	assert.Equal(t, "http://envonly", ctx.Server)
	assert.Empty(t, ctx.Name)
}

func TestResolveErrsWithoutServer(t *testing.T) {
	cfg := &Config{}
	_, err := Resolve(cfg, Overrides{}, Overrides{})
	assert.ErrorIs(t, err, ErrNoCurrent)
}
