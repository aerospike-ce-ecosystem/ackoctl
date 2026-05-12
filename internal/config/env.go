package config

import "os"

const (
	EnvContext   = "ACKOCTL_CONTEXT"
	EnvServer    = "ACKOCTL_SERVER"
	EnvToken     = "ACKOCTL_TOKEN"
	EnvWorkspace = "ACKOCTL_WORKSPACE"
)

type Overrides struct {
	Context         string
	Server          string
	Token           string
	WorkspaceID     string
	InsecureSkipTLS *bool
}

func EnvOverrides() Overrides {
	return Overrides{
		Context:     os.Getenv(EnvContext),
		Server:      os.Getenv(EnvServer),
		Token:       os.Getenv(EnvToken),
		WorkspaceID: os.Getenv(EnvWorkspace),
	}
}

// Resolve merges env and flag overrides into a Context derived from cfg.
// Priority: flags > env > config file. The caller passes flag-derived values
// in flagsOverride; empty strings mean "no override at this layer."
//
// If neither flags nor env specify a context name, cfg.CurrentContext is used.
// If no context exists at all but a server is provided via env/flags, a
// synthetic ephemeral context is returned so commands can still run.
func Resolve(cfg *Config, env, flags Overrides) (Context, error) {
	name := firstNonEmpty(flags.Context, env.Context, cfg.CurrentContext)

	var base Context
	if name != "" {
		if ctx, _ := cfg.Find(name); ctx != nil {
			base = *ctx
		} else if flags.Server == "" && env.Server == "" {
			return Context{}, ErrContextNotFound
		} else {
			base.Name = name
		}
	}

	merged := Context{
		Name:        base.Name,
		Server:      firstNonEmpty(flags.Server, env.Server, base.Server),
		Token:       firstNonEmpty(flags.Token, env.Token, base.Token),
		WorkspaceID: firstNonEmpty(flags.WorkspaceID, env.WorkspaceID, base.WorkspaceID),
		InsecureSkipTLS: pickBool(flags.InsecureSkipTLS, base.InsecureSkipTLS),
	}
	if merged.Server == "" {
		return Context{}, ErrNoCurrent
	}
	return merged, nil
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func pickBool(override *bool, fallback bool) bool {
	if override != nil {
		return *override
	}
	return fallback
}
