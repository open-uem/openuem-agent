package deploy

// wingetSourceAddArgs are the arguments that register a WinGet REST source. The source is
// trusted: winget checks installers of untrusted sources with IAttachmentExecute, which
// fails without an interactive session (0x8A150001), so SYSTEM, which runs the
// deployments, couldn't install any of its packages. The source is the console's own
// repository, served over HTTPS, and winget still checks each installer's SHA-256.
func wingetSourceAddArgs(name, sourceURL string) []string {
	return []string{"source", "add", "--name", name, "--arg", sourceURL,
		"--type", "Microsoft.Rest", "--trust-level", "trusted", "--accept-source-agreements", "--disable-interactivity", "--verbose-logs"}
}
