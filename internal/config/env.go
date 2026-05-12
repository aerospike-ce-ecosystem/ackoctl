package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	EnvContext         = "ACKOCTL_CONTEXT"
	EnvServer          = "ACKOCTL_SERVER"
	EnvToken           = "ACKOCTL_TOKEN"
	EnvWorkspace       = "ACKOCTL_WORKSPACE"
	EnvInsecureSkipTLS = "ACKOCTL_INSECURE_SKIP_TLS"
)

type Overrides struct {
	Context         string
	Server          string
	Token           string
	WorkspaceID     string
	InsecureSkipTLS *bool
	// ContextExplicit records that Context was supplied by the user (flag or
	// env). When true, Resolve refuses to silently fall back to an ephemeral
	// context if the named one is missing.
	ContextExplicit bool
}

func EnvOverrides() (Overrides, error) {
	o := Overrides{
		Context:     os.Getenv(EnvContext),
		Server:      os.Getenv(EnvServer),
		Token:       os.Getenv(EnvToken),
		WorkspaceID: os.Getenv(EnvWorkspace),
	}
	if o.Context != "" {
		o.ContextExplicit = true
	}
	if raw := os.Getenv(EnvInsecureSkipTLS); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return Overrides{}, fmt.Errorf("invalid %s=%q: %w", EnvInsecureSkipTLS, raw, err)
		}
		o.InsecureSkipTLS = &b
	}
	return o, nil
}

// Resolve merges env and flag overrides into a Context derived from cfg.
// Priority: flags > env > config file. The caller passes flag-derived values
// in flagsOverride; empty strings mean "no override at this layer."
//
// If --context (flag) or ACKOCTL_CONTEXT (env) names a context that does not
// exist in cfg, Resolve returns ErrContextNotFound — never silently invents
// an ephemeral context with the wrong name.
//
// If no context is named at all but a server is provided via env/flags, a
// synthetic ephemeral context is returned so first-time users can still run
// commands without a config file.
func Resolve(cfg *Config, env, flags Overrides) (Context, error) {
	name := firstNonEmpty(flags.Context, env.Context, cfg.CurrentContext)
	explicit := flags.ContextExplicit || env.ContextExplicit

	var base Context
	if name != "" {
		ctx, _ := cfg.Find(name)
		switch {
		case ctx != nil:
			base = *ctx
		case explicit:
			return Context{}, fmt.Errorf("%w: %s", ErrContextNotFound, name)
		case flags.Server == "" && env.Server == "":
			return Context{}, ErrContextNotFound
		default:
			base.Name = name
		}
	}

	merged := Context{
		Name:            base.Name,
		Server:          firstNonEmpty(flags.Server, env.Server, base.Server),
		Token:           firstNonEmpty(flags.Token, env.Token, base.Token),
		WorkspaceID:     firstNonEmpty(flags.WorkspaceID, env.WorkspaceID, base.WorkspaceID),
		InsecureSkipTLS: pickBool(base.InsecureSkipTLS, env.InsecureSkipTLS, flags.InsecureSkipTLS),
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

// pickBool walks overrides from lowest to highest precedence; a non-nil
// later override replaces the earlier value. base is the file-level value
// (always present, never nil). This lets a user explicitly set
// --insecure-skip-tls=false to override a context that has it set to true.
func pickBool(base bool, overrides ...*bool) bool {
	v := base
	for _, o := range overrides {
		if o != nil {
			v = *o
		}
	}
	return v
}
