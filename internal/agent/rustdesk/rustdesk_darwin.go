//go:build darwin

package rustdesk

type RustDeskConfig struct {
	Binary     string
	LaunchArgs []string
	GetIDArgs  []string
	ConfigFile string
}

func New() *RustDeskConfig {
	return &RustDeskConfig{}
}

func (cfg *RustDeskConfig) Configure(config []byte) error {
	return nil
}

func (cfg *RustDeskConfig) GetInstallationInfo() error {
	return nil
}

func (cfg *RustDeskConfig) LaunchRustDesk() error {
	return nil
}

func (cfg *RustDeskConfig) GetRustDeskID() (string, error) {
	return "", nil
}
