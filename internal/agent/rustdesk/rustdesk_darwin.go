//go:build darwin

package rustdesk

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

func (cfg *RustDeskConfig) KillRustDeskProcess() error {
	return nil
}

func (cfg *RustDeskConfig) ConfigRollBack() error {
	return nil
}
