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

func KillRustDeskProcess() error {
	return nil
}

func ConfigRollBack(isFlatpak bool) error {
	return nil
}
