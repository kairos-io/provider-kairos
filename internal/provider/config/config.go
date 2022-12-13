package config

type P2P struct {
	NetworkToken string `yaml:"network_token,omitempty"`
	NetworkID    string `yaml:"network_id,omitempty"`
	Role         string `yaml:"role,omitempty"`
	DNS          bool   `yaml:"dns,omitempty"`
	LogLevel     string `yaml:"loglevel,omitempty"`
	VPN          VPN    `yaml:"vpn,omitempty"`

	MinimumNodes int  `yaml:"minimum_nodes,omitempty"`
	DisableDHT   bool `yaml:"disable_dht,omitempty"`
	Auto         Auto `yaml:"auto,omitempty"`

	DynamicRoles bool `yaml:"dynamic_roles,omitempty"`
}

type VPN struct {
	Create *bool             `yaml:"create,omitempty"`
	Use    *bool             `yaml:"use,omitempty"`
	Env    map[string]string `yaml:"env,omitempty"`
}

// If no setting is provided by the user,
// we assume that we are going to create and use the VPN
// for the network layer of our cluster.
func (p P2P) UseVPNWithKubernetes() bool {
	return p.VPNNeedsCreation() && (p.VPN.Use == nil || *p.VPN.Use)
}

func (p P2P) VPNNeedsCreation() bool {
	return p.VPN.Create == nil || *p.VPN.Create
}

type Config struct {
	P2P      *P2P    `yaml:"p2p,omitempty"`
	K3sAgent K3s     `yaml:"k3s-agent,omitempty"`
	K3s      K3s     `yaml:"k3s,omitempty"`
	KubeVIP  KubeVIP `yaml:"kubevip,omitempty"`
}

type KubeVIP struct {
	Args        []string `yaml:"args,omitempty"`
	EIP         string   `yaml:"eip,omitempty"`
	ManifestURL string   `yaml:"manifest_url,omitempty"`
	Interface   string   `yaml:"interface,omitempty"`
	Enable      *bool    `yaml:"enable,omitempty"`
}

func (k KubeVIP) IsEnabled() bool {
	return (k.Enable == nil && k.EIP != "") || (k.Enable != nil && *k.Enable)
}

type Auto struct {
	Enable *bool `yaml:"enable,omitempty"`
	HA     HA    `yaml:"ha,omitempty"`
}

func (a Auto) IsEnabled() bool {
	return a.Enable == nil || (a.Enable != nil && *a.Enable)
}

func (ha HA) IsEnabled() bool {
	return (ha.Enable != nil && *ha.Enable) || (ha.Enable == nil && ha.MasterNodes != nil)
}

type HA struct {
	Enable      *bool  `yaml:"enable,omitempty"`
	ExternalDB  string `yaml:"external_db,omitempty"`
	MasterNodes *int   `yaml:"master_nodes,omitempty"`
}

type K3s struct {
	Env         map[string]string `yaml:"env,omitempty"`
	ReplaceEnv  bool              `yaml:"replace_env,omitempty"`
	ReplaceArgs bool              `yaml:"replace_args,omitempty"`
	Args        []string          `yaml:"args,omitempty"`
	Enabled     bool              `yaml:"enabled,omitempty"`
}
