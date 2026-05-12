package cli

import (
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/config"
)

// resolveContext builds the effective Context applying file < env < flag.
// --insecure-skip-tls is treated as "true forces override"; false means
// "fall back to context value".
func resolveContext(global *GlobalFlags) (config.Context, error) {
	cfg, err := config.Load(resolveConfigPath(global))
	if err != nil {
		return config.Context{}, err
	}
	flags := config.Overrides{
		Context:     global.Context,
		Server:      global.Server,
		Token:       global.Token,
		WorkspaceID: global.WorkspaceID,
	}
	if global.InsecureSkipTLS {
		t := true
		flags.InsecureSkipTLS = &t
	}
	return config.Resolve(cfg, config.EnvOverrides(), flags)
}

func newClient(global *GlobalFlags) (*client.BaseClient, error) {
	ctx, err := resolveContext(global)
	if err != nil {
		return nil, err
	}
	return client.New(ctx), nil
}
