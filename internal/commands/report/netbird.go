package report

import "time"

type PeerStateDetailOutput struct {
	FQDN                   string           `json:"fqdn" yaml:"fqdn"`
	IP                     string           `json:"netbirdIp" yaml:"netbirdIp"`
	PubKey                 string           `json:"publicKey" yaml:"publicKey"`
	Status                 string           `json:"status" yaml:"status"`
	LastStatusUpdate       time.Time        `json:"lastStatusUpdate" yaml:"lastStatusUpdate"`
	ConnType               string           `json:"connectionType" yaml:"connectionType"`
	IceCandidateType       IceCandidateType `json:"iceCandidateType" yaml:"iceCandidateType"`
	IceCandidateEndpoint   IceCandidateType `json:"iceCandidateEndpoint" yaml:"iceCandidateEndpoint"`
	RelayAddress           string           `json:"relayAddress" yaml:"relayAddress"`
	LastWireguardHandshake time.Time        `json:"lastWireguardHandshake" yaml:"lastWireguardHandshake"`
	TransferReceived       int64            `json:"transferReceived" yaml:"transferReceived"`
	TransferSent           int64            `json:"transferSent" yaml:"transferSent"`
	Latency                time.Duration    `json:"latency" yaml:"latency"`
	RosenpassEnabled       bool             `json:"quantumResistance" yaml:"quantumResistance"`
	Networks               []string         `json:"networks" yaml:"networks"`
}

type PeersStateOutput struct {
	Total     int                     `json:"total" yaml:"total"`
	Connected int                     `json:"connected" yaml:"connected"`
	Details   []PeerStateDetailOutput `json:"details" yaml:"details"`
}

type SignalStateOutput struct {
	URL       string `json:"url" yaml:"url"`
	Connected bool   `json:"connected" yaml:"connected"`
	Error     string `json:"error" yaml:"error"`
}

type ManagementStateOutput struct {
	URL       string `json:"url" yaml:"url"`
	Connected bool   `json:"connected" yaml:"connected"`
	Error     string `json:"error" yaml:"error"`
}

type RelayStateOutputDetail struct {
	URI       string `json:"uri" yaml:"uri"`
	Available bool   `json:"available" yaml:"available"`
	Error     string `json:"error" yaml:"error"`
}

type RelayStateOutput struct {
	Total     int                      `json:"total" yaml:"total"`
	Available int                      `json:"available" yaml:"available"`
	Details   []RelayStateOutputDetail `json:"details" yaml:"details"`
}

type IceCandidateType struct {
	Local  string `json:"local" yaml:"local"`
	Remote string `json:"remote" yaml:"remote"`
}

type NsServerGroupStateOutput struct {
	Servers []string `json:"servers" yaml:"servers"`
	Domains []string `json:"domains" yaml:"domains"`
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Error   string   `json:"error" yaml:"error"`
}

type SSHSessionOutput struct {
	Username      string `json:"username" yaml:"username"`
	RemoteAddress string `json:"remoteAddress" yaml:"remoteAddress"`
	Command       string `json:"command" yaml:"command"`
	JWTUsername   string `json:"jwtUsername,omitempty" yaml:"jwtUsername,omitempty"`
}

type SSHServerStateOutput struct {
	Enabled  bool               `json:"enabled" yaml:"enabled"`
	Sessions []SSHSessionOutput `json:"sessions" yaml:"sessions"`
}

type SystemEventOutput struct {
	ID          string            `json:"id" yaml:"id"`
	Severity    string            `json:"severity" yaml:"severity"`
	Category    string            `json:"category" yaml:"category"`
	Message     string            `json:"message" yaml:"message"`
	UserMessage string            `json:"userMessage" yaml:"userMessage"`
	Timestamp   time.Time         `json:"timestamp" yaml:"timestamp"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
}

type NetBirdOverview struct {
	Peers                   PeersStateOutput           `json:"peers" yaml:"peers"`
	CliVersion              string                     `json:"cliVersion" yaml:"cliVersion"`
	DaemonVersion           string                     `json:"daemonVersion" yaml:"daemonVersion"`
	ManagementState         ManagementStateOutput      `json:"management" yaml:"management"`
	SignalState             SignalStateOutput          `json:"signal" yaml:"signal"`
	Relays                  RelayStateOutput           `json:"relays" yaml:"relays"`
	IP                      string                     `json:"netbirdIp" yaml:"netbirdIp"`
	PubKey                  string                     `json:"publicKey" yaml:"publicKey"`
	KernelInterface         bool                       `json:"usesKernelInterface" yaml:"usesKernelInterface"`
	FQDN                    string                     `json:"fqdn" yaml:"fqdn"`
	RosenpassEnabled        bool                       `json:"quantumResistance" yaml:"quantumResistance"`
	RosenpassPermissive     bool                       `json:"quantumResistancePermissive" yaml:"quantumResistancePermissive"`
	Networks                []string                   `json:"networks" yaml:"networks"`
	NumberOfForwardingRules int                        `json:"forwardingRules" yaml:"forwardingRules"`
	NSServerGroups          []NsServerGroupStateOutput `json:"dnsServers" yaml:"dnsServers"`
	Events                  []SystemEventOutput        `json:"events" yaml:"events"`
	LazyConnectionEnabled   bool                       `json:"lazyConnectionEnabled" yaml:"lazyConnectionEnabled"`
	ProfileName             string                     `json:"profileName" yaml:"profileName"`
	SSHServerState          SSHServerStateOutput       `json:"sshServer" yaml:"sshServer"`
}
