package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvOverridesInsecureSkipTLS(t *testing.T) {
	t.Setenv(EnvInsecureSkipTLS, "true")
	o, err := EnvOverrides()
	require.NoError(t, err)
	require.NotNil(t, o.InsecureSkipTLS)
	assert.True(t, *o.InsecureSkipTLS)
}

func TestEnvOverridesInsecureSkipTLSExplicitFalse(t *testing.T) {
	t.Setenv(EnvInsecureSkipTLS, "false")
	o, err := EnvOverrides()
	require.NoError(t, err)
	require.NotNil(t, o.InsecureSkipTLS)
	assert.False(t, *o.InsecureSkipTLS)
}

func TestEnvOverridesInsecureSkipTLSInvalid(t *testing.T) {
	t.Setenv(EnvInsecureSkipTLS, "yesplease")
	_, err := EnvOverrides()
	assert.Error(t, err)
}

func TestEnvOverridesContextExplicit(t *testing.T) {
	t.Setenv(EnvContext, "kind")
	o, err := EnvOverrides()
	require.NoError(t, err)
	assert.Equal(t, "kind", o.Context)
	assert.True(t, o.ContextExplicit)
}

func TestResolveExplicitContextNotFoundIsError(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Context{Name: "kind", Server: "http://1"})
	_, err := Resolve(
		cfg,
		Overrides{},
		Overrides{Context: "missing", ContextExplicit: true, Server: "http://envonly"},
	)
	assert.ErrorIs(t, err, ErrContextNotFound,
		"explicit --context must error when the named context is absent even if a server override exists")
}

func TestResolveFlagFalseOverridesContextTrue(t *testing.T) {
	cfg := &Config{CurrentContext: "kind"}
	cfg.Upsert(Context{Name: "kind", Server: "http://1", InsecureSkipTLS: true})
	f := false
	ctx, err := Resolve(cfg, Overrides{}, Overrides{InsecureSkipTLS: &f})
	require.NoError(t, err)
	assert.False(t, ctx.InsecureSkipTLS,
		"--insecure-skip-tls=false must be able to disable a context-level true")
}

func TestResolveEnvOverridesFileButFlagBeatsEnv(t *testing.T) {
	cfg := &Config{CurrentContext: "kind"}
	cfg.Upsert(Context{Name: "kind", Server: "http://1", InsecureSkipTLS: false})
	envT := true
	flagF := false
	ctx, err := Resolve(cfg, Overrides{InsecureSkipTLS: &envT}, Overrides{InsecureSkipTLS: &flagF})
	require.NoError(t, err)
	assert.False(t, ctx.InsecureSkipTLS, "flag beats env beats file")
}
