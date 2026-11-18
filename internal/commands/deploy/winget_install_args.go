package deploy

import "regexp"

var packageVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// IsValidPackageVersion reports whether a requested package version can be passed to a
// package manager. An empty version means the latest one.
func IsValidPackageVersion(version string) bool {
	return version == "" || (len(version) <= 128 && packageVersionPattern.MatchString(version))
}

// wingetInstallArgs are the arguments of a machine-wide install. winget runs directly, not
// through PowerShell, so its exit code reports failures and the package id and version
// are never parsed by a shell.
func wingetInstallArgs(packageID, version string) []string {
	args := []string{"install", "--id", packageID, "--exact"}
	if version != "" {
		args = append(args, "--version", version)
	}
	return append(args, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
}

// wingetUninstallArgs are the arguments that remove every installed version of a package
func wingetUninstallArgs(packageID string) []string {
	return []string{"uninstall", "--id", packageID, "--exact", "--all-versions", "--silent", "--accept-source-agreements", "--disable-interactivity"}
}
