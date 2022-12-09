package config

type Kairos struct {
	NetworkToken string `yaml:"network_token,omitempty"`
	NetworkID    string `yaml:"network_id,omitempty"`
	Role         string `yaml:"role,omitempty"`
	DNS          bool   `yaml:"dns,omitempty"`
	LogLevel     string `yaml:"loglevel,omitempty"`
	Hybrid       bool   `yaml:"hybrid,omitempty"`
	HybridVPN    bool   `yaml:"hybrid_vpn,omitempty"`
	MinimumNodes int    `yaml:"minimum_nodes,omitempty"`
	SkipAuto     bool   `yaml:"skip_auto"`
	DisableDHT   bool   `yaml:"disable_dht,omitempty"`
	HA           HA     `yaml:"ha,omitempty"`
}

type Config struct {
	Kairos   *Kairos           `yaml:"kairos,omitempty"`
	K3sAgent K3s               `yaml:"k3s-agent,omitempty"`
	K3s      K3s               `yaml:"k3s,omitempty"`
	VPN      map[string]string `yaml:"vpn,omitempty"`
	KubeVIP  KubeVIP           `yaml:"kubevip,omitempty"`
}

type KubeVIP struct {
	Args        []string `yaml:"args,omitempty"`
	EIP         string   `yaml:"eip,omitempty"`
	ManifestURL string   `yaml:"manifest_url,omitempty"`
	Interface   string   `yaml:"interface,omitempty"`
}

type HA struct {
	Enable      bool   `yaml:"enable,omitempty"`
	ExternalDB  string `yaml:"external_db,omitempty"`
	MasterNodes int    `yaml:"master_nodes,omitempty"`
}

type K3s struct {
	Env         map[string]string `yaml:"env,omitempty"`
	ReplaceEnv  bool              `yaml:"replace_env,omitempty"`
	ReplaceArgs bool              `yaml:"replace_args,omitempty"`
	Args        []string          `yaml:"args,omitempty"`
	Enabled     bool              `yaml:"enabled,omitempty"`
}
