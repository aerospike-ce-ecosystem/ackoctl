package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "ackoctl/v1"
	Kind       = "Config"
)

type Context struct {
	Name            string `yaml:"name" json:"name"`
	Server          string `yaml:"server" json:"server"`
	Token           string `yaml:"token,omitempty" json:"token,omitempty"`
	WorkspaceID     string `yaml:"workspace-id,omitempty" json:"workspace-id,omitempty"`
	InsecureSkipTLS bool   `yaml:"insecure-skip-tls,omitempty" json:"insecure-skip-tls,omitempty"`
}

type Config struct {
	APIVersion     string    `yaml:"apiVersion" json:"apiVersion"`
	Kind           string    `yaml:"kind" json:"kind"`
	CurrentContext string    `yaml:"current-context,omitempty" json:"current-context,omitempty"`
	Contexts       []Context `yaml:"contexts,omitempty" json:"contexts,omitempty"`
}

var (
	ErrNoContext       = errors.New("no contexts defined")
	ErrContextNotFound = errors.New("context not found")
	ErrNoCurrent       = errors.New("current-context is not set")
)

func DefaultPath() (string, error) {
	if p := os.Getenv("ACKOCTL_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user home dir (set $HOME or pass --config / $ACKOCTL_CONFIG): %w", err)
	}
	return filepath.Join(home, ".ackoctl", "config.yaml"), nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIVersion: APIVersion, Kind: Kind}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = APIVersion
	}
	if cfg.Kind == "" {
		cfg.Kind = Kind
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if cfg.APIVersion == "" {
		cfg.APIVersion = APIVersion
	}
	if cfg.Kind == "" {
		cfg.Kind = Kind
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	// Atomic write: temp file in same dir + rename, with 0600 perms enforced
	// regardless of any pre-existing file mode. Prevents credential exposure
	// and avoids truncation on disk-full / interrupted writes.
	tmp, err := os.CreateTemp(dir, ".ackoctl-config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (c *Config) Find(name string) (*Context, int) {
	for i := range c.Contexts {
		if c.Contexts[i].Name == name {
			return &c.Contexts[i], i
		}
	}
	return nil, -1
}

func (c *Config) Upsert(ctx Context) {
	if _, idx := c.Find(ctx.Name); idx >= 0 {
		c.Contexts[idx] = ctx
		return
	}
	c.Contexts = append(c.Contexts, ctx)
}

func (c *Config) Delete(name string) error {
	_, idx := c.Find(name)
	if idx < 0 {
		return fmt.Errorf("%w: %s", ErrContextNotFound, name)
	}
	c.Contexts = append(c.Contexts[:idx], c.Contexts[idx+1:]...)
	if c.CurrentContext == name {
		c.CurrentContext = ""
	}
	return nil
}

func (c *Config) Use(name string) error {
	if _, idx := c.Find(name); idx < 0 {
		return fmt.Errorf("%w: %s", ErrContextNotFound, name)
	}
	c.CurrentContext = name
	return nil
}

func (c *Config) Current() (*Context, error) {
	if len(c.Contexts) == 0 {
		return nil, ErrNoContext
	}
	if c.CurrentContext == "" {
		return nil, ErrNoCurrent
	}
	ctx, _ := c.Find(c.CurrentContext)
	if ctx == nil {
		return nil, fmt.Errorf("%w: %s (current-context)", ErrContextNotFound, c.CurrentContext)
	}
	return ctx, nil
}
