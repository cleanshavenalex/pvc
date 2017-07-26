package pvc

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// SecretsClient is the client that retrieves secret values
type SecretsClient struct {
	backend secretBackend
}

// Get returns the value of a secret from the configured backend
func (sc *SecretsClient) Get(id string) ([]byte, error) {
	return sc.backend.Get(id)
}

type secretBackend interface {
	Get(id string) ([]byte, error)
}

// SecretDefinition defines a secret and how it can be accessed via the various backends
type SecretDefinition struct {
	ID         string // arbitrary identifier for this secret
	VaultPath  string // path in Vault (no leading slash, eg "secret/foo/bar")
	EnvVarName string // environment variable name
	JSONKey    string // key in JSON object
}

type vaultBackend struct {
	host               string
	authentication     VaultAuthentication
	authRetries        uint
	authRetryDelaySecs uint
	token              string
	appid              string
	userid             string
	useridpath         string
	roleid             string
}

type envVarBackend struct {
}

type jsonFileBackend struct {
	fileLocation string
}

type secretsClientConfig struct {
	mapping         string
	backendCount    int
	vaultBackend    *vaultBackend
	envVarBackend   *envVarBackend
	jsonFileBackend *jsonFileBackend
}

// SecretsClientOption defines options when creating a SecretsClient
type SecretsClientOption func(*secretsClientConfig)

// WithMapping sets the template string mapping to determine the location for each secret in the backend. The secret ID will be interpolated as ".ID".
// Example (Vault Backend): "secret/foo/bar/{{ .ID }}".
// Example (Env Var Backend): "MYAPP_SECRET_{{ .ID }}"
// Example (JSON Backend): "{{ .ID }}"
func WithMapping(mapping string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		s.mapping = mapping
	}
}

// WithVaultBackend enables the Vault backend.
func WithVaultBackend() SecretsClientOption {
	return func(s *secretsClientConfig) {
		s.vaultBackend = &vaultBackend{}
		s.backendCount++
	}
}

// WithVaultHost sets the Vault server host (eg, https//my.vault.com:8300)
func WithVaultHost(host string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.host = host
	}
}

// WithVaultAuthentication sets the Vault authentication method
func WithVaultAuthentication(auth VaultAuthentication) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.authentication = auth
	}
}

// WithVaultAuthRetries sets the number of retries if authentication fails (default: 0)
func WithVaultAuthRetries(retries uint) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.authRetries = retries
	}
}

// WithVaultAuthRetryDelay sets the delay in seconds between authentication attempts (default: 0)
func WithVaultAuthRetryDelay(secs uint) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.authRetryDelaySecs = secs
	}
}

// WithVaultToken sets the token to use when using token auth
func WithVaultToken(token string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.token = token
	}
}

// WithVaultAppID sets the AppID to use when using AppID auth
func WithVaultAppID(appid string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.appid = appid
	}
}

// WithVaultUserID sets the UserID to use when using AppID auth
func WithVaultUserID(userid string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.userid = userid
	}
}

// WithVaultUserIDPath sets the path to the file containing UserID when using AppID auth
func WithVaultUserIDPath(useridpath string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.useridpath = useridpath
	}
}

// WithVaultRoleID sets the RoleID when using AppRole authentication
func WithVaultRoleID(roleid string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.vaultBackend == nil {
			s.vaultBackend = &vaultBackend{}
		}
		s.vaultBackend.roleid = roleid
	}
}

// WithEnvVarBackend enables the environment variable backend.
func WithEnvVarBackend() SecretsClientOption {
	return func(s *secretsClientConfig) {
		s.envVarBackend = &envVarBackend{}
		s.backendCount++
	}
}

// WithJSONFileBackend enables the JSON file backend. The file should contain a single JSON object associating a name with a value: { "mysecret": "pa55w0rd"}.
func WithJSONFileBackend() SecretsClientOption {
	return func(s *secretsClientConfig) {
		s.jsonFileBackend = &jsonFileBackend{}
		s.backendCount++
	}
}

// WithJSONFileLocation sets the location to the JSON file
func WithJSONFileLocation(loc string) SecretsClientOption {
	return func(s *secretsClientConfig) {
		if s.jsonFileBackend == nil {
			s.jsonFileBackend = &jsonFileBackend{}
		}
		s.jsonFileBackend.fileLocation = loc
	}
}

// NewSecretsClient returns a SecretsClient configured according to the SecretsClientOptions supplied. Exactly one backend must be enabled.
// Weird things will happen if you mix options with incompatible backends.
func NewSecretsClient(ops ...SecretsClientOption) (*SecretsClient, error) {
	config := &secretsClientConfig{}
	for _, op := range ops {
		op(config)
	}
	if config.backendCount != 1 {
		return nil, fmt.Errorf("exactly one backend must be enabled")
	}
	sc := SecretsClient{}
	switch {
	case config.vaultBackend != nil:
		vbe, err := newVaultBackendGetter(config.vaultBackend, config.mapping)
		if err != nil {
			return nil, fmt.Errorf("error getting vault backend: %v", err)
		}
		sc.backend = vbe
	case config.envVarBackend != nil:
		return nil, fmt.Errorf("env var backend not implemented")
	case config.jsonFileBackend != nil:
		return nil, fmt.Errorf("json file backend not implemented")
	}
	return &sc, nil
}

// secretMapper manages turning secret IDs into a location suitable for a backend to use
type secretMapper struct {
	mappingTmpl *template.Template
}

// newSecretMapper returns a secret mapper using the supplied mapping string
func newSecretMapper(mapping string) (*secretMapper, error) {
	if !strings.Contains(mapping, "{{ .ID") && !strings.Contains(mapping, "{{.ID") {
		return nil, fmt.Errorf("mapping must contain {{ .ID }}")
	}
	tmpl, err := template.New("secret-mapper").Parse(mapping)
	if err != nil {
		return nil, fmt.Errorf("error parsing mapping: %v", err)
	}
	return &secretMapper{
		mappingTmpl: tmpl,
	}, nil
}

// mapSecret maps a secret ID to a location via the mapping string
func (sm *secretMapper) mapSecret(id string) (string, error) {
	d := struct{ ID string }{ID: id}
	b := bytes.Buffer{}
	err := sm.mappingTmpl.Execute(&b, d)
	if err != nil {
		return "", fmt.Errorf("error executing mapping template: %v", err)
	}
	return string(b.Bytes()), nil
}