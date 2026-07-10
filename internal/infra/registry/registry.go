package registry

import (
	"fmt"
	"strings"
)

type Registry interface {
	DockerConfig() string
	Host() string
	Token() string
	BuildOutput(destination string) string
	Insecure() bool
}

type Provider string

const (
	ProviderGHCR          Provider = "ghcr"
	ProviderLocalRegistry Provider = "local_registry"
)

type Config struct {
	Provider Provider
	Host     string
	Username string
	Password string
}

func NewFromConfig(cfg Config) (Registry, error) {
	switch cfg.Provider {
	case ProviderGHCR:
		return NewGHCR(cfg.Host, cfg.Password), nil
	case ProviderLocalRegistry:
		return NewLocalRegistry(cfg.Host, cfg.Username, cfg.Password), nil
	default:
		return nil, fmt.Errorf("unsupported registry provider: %s", cfg.Provider)
	}
}

type ghcr struct {
	host  string
	token string
}

func NewGHCR(host, token string) Registry {
	return &ghcr{
		host:  host,
		token: token,
	}
}

func (r *ghcr) DockerConfig() string {
	authHost := strings.Split(strings.TrimSpace(r.host), "/")[0]
	return fmt.Sprintf(`{"auths":{"%s":{"username":"x-access-token","password":%q}}}`, authHost, r.token)
}

func (r *ghcr) Host() string {
	return r.host
}

func (r *ghcr) Token() string {
	return r.token
}

func (r *ghcr) BuildOutput(destination string) string {
	return fmt.Sprintf("type=image,name=%s,push=true", destination)
}

func (r *ghcr) Insecure() bool {
	return false
}

type localRegistry struct {
	host     string
	username string
	password string
}

func NewLocalRegistry(host, username, password string) Registry {
	return &localRegistry{
		host:     host,
		username: username,
		password: password,
	}
}

func (r *localRegistry) DockerConfig() string {
	authHost := strings.Split(strings.TrimSpace(r.host), "/")[0]
	return fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":%q}}}`, authHost, r.username, r.password)
}

func (r *localRegistry) Host() string {
	return r.host
}

func (r *localRegistry) Token() string {
	return r.password
}

// Localではhttpで通信したいので、insecure=trueにする
func (r *localRegistry) BuildOutput(destination string) string {
	return fmt.Sprintf("type=image,name=%s,push=true,registry.insecure=true", destination)
}

func (r *localRegistry) Insecure() bool {
	return true
}
