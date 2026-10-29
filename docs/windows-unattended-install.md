# Unattended Windows installation

Use this checklist when delivering the Windows agent through a deployment tool.
The installer options are described in the
[Windows installation reference](https://openuem.eu/docs/Installation/Agent/windows/).
This example is for a fresh installation; handle upgrades separately.
Windows Server support is limited; some inventory fields may be absent from reports.

## Prepare the installation

1. Use an elevated 64-bit PowerShell session or a deployment job running as SYSTEM.
2. Select an installer version and architecture from
   [Windows downloads](https://openuem.eu/docs/Downloads/windows/). Record its
   version, download URL and published SHA256 in the deployment definition.
3. Prepare a staging directory accessible only to SYSTEM and administrators.
   Restrict access **before** copying credentials into it. Distribute the installer
   and enrollment files through an authenticated private deployment channel.
4. Obtain `ca.cer`, `agent.cer`, `agent.key` and `sftp.cer` from the administrator
   of the intended OpenUEM environment. `agent.key` is a private credential.
   Never distribute the CA signing key or publish enrollment files in a repository.
5. Confirm that the endpoint can resolve and reach its configured NATS server.

Importing a CA into a browser trust store only addresses that browser's trust.
The agent still needs the certificate and key files specified to its installer.

## Run the installer

Set the paths, server and checksum below for your environment. Use the checksum
for the **selected installer version**, rather than a checksum from an older example.
The sample paths contain spaces intentionally.

```powershell
$ErrorActionPreference = 'Stop'
$installer = 'C:\OpenUEM Bootstrap\openuem-agent-setup.exe'
$credentials = 'C:\OpenUEM Bootstrap\enrollment'
$natsServer = 'uem.example.com:4433'
$expectedSha256 = 'REPLACE_WITH_THE_PUBLISHED_SHA256'
# Set both IDs for explicit placement. Leave both null only for default placement.
$tenantId = $null
$siteId = $null

if (-not [Environment]::Is64BitProcess) {
    throw 'Use a 64-bit PowerShell process so both registry views can be inspected.'
}
if (Get-Service -Name 'openuem-agent' -ErrorAction SilentlyContinue) {
    throw 'An agent service already exists. Review the existing installation first.'
}
$installDirectories = @(
    (Join-Path $env:ProgramFiles 'OpenUEM Agent')
)
if (${env:ProgramFiles(x86)}) {
    $installDirectories += Join-Path ${env:ProgramFiles(x86)} 'OpenUEM Agent'
}
foreach ($directory in $installDirectories) {
    if (Test-Path -LiteralPath $directory) {
        throw "An installation directory already exists: $directory. Recover or uninstall it first."
    }
}
foreach ($uninstallRoot in @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall'
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
)) {
    if (Test-Path -LiteralPath $uninstallRoot) {
        foreach ($entry in Get-ChildItem -LiteralPath $uninstallRoot) {
            $registration = Get-ItemProperty -LiteralPath $entry.PSPath
            if ($registration.DisplayName -like 'OpenUEM*Agent*') {
                throw 'An agent uninstall registration exists. Recover or uninstall it first.'
            }
        }
    }
}
$hasTenant = -not [string]::IsNullOrWhiteSpace([string]$tenantId)
$hasSite = -not [string]::IsNullOrWhiteSpace([string]$siteId)
if ($hasTenant -ne $hasSite) { throw 'Set both tenantId and siteId, or leave both null.' }
if ($hasTenant -and (([string]$tenantId -notmatch '\A[1-9][0-9]*\z') -or
                    ([string]$siteId -notmatch '\A[1-9][0-9]*\z'))) {
    throw 'Tenant and site IDs must be positive integers from the console.'
}
if ($natsServer -match '\s') { throw 'Whitespace is not permitted in the NATS server list.' }
$dnsLabel = '[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?'
foreach ($endpoint in $natsServer.Split(',')) {
    if ($endpoint -notmatch "\A(?<dnsHost>$dnsLabel(?:\.$dnsLabel)*):(?<port>[0-9]{1,5})\z") {
        throw 'Use DNS-name:port or IPv4:port, separated by commas for a cluster.'
    }
    $dnsHost = $Matches.dnsHost
    $port = [int]$Matches.port
    if ($dnsHost.Length -gt 253) { throw 'NATS hostname is too long.' }
    if ($dnsHost -match '\A[0-9.]+\z') {
        $octets = $dnsHost.Split('.')
        if ($octets.Count -ne 4) { throw 'Use a four-octet IPv4 address.' }
        foreach ($octet in $octets) {
            if ($octet -notmatch '\A(?:0|[1-9][0-9]{0,2})\z' -or [int]$octet -gt 255) {
                throw 'Invalid IPv4 address.'
            }
        }
    }
    if ($port -lt 1 -or $port -gt 65535) { throw 'Invalid NATS port.' }
}
if ($expectedSha256 -notmatch '\A[0-9a-fA-F]{64}\z') {
    throw 'Set the published SHA256 for the selected installer.'
}
if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash -ne $expectedSha256) {
    throw 'Installer checksum mismatch.'
}
if ((Get-AuthenticodeSignature -LiteralPath $installer).Status -ne 'Valid') {
    throw 'Installer signature validation failed. Investigate before continuing.'
}
foreach ($file in @('ca.cer', 'agent.cer', 'agent.key', 'sftp.cer')) {
    if (-not (Test-Path -LiteralPath (Join-Path $credentials $file) -PathType Leaf)) {
        throw "Missing enrollment file: $file"
    }
}

$installerArguments = @(
    '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
    "/server=$natsServer"
    "/cacert=`"$credentials\ca.cer`""
    "/cert=`"$credentials\agent.cer`""
    "/key=`"$credentials\agent.key`""
    "/sftpcert=`"$credentials\sftp.cer`""
)
if ($hasTenant) {
    $installerArguments += "/tenantid=$tenantId", "/siteid=$siteId"
}
$process = Start-Process -FilePath $installer -ArgumentList ($installerArguments -join ' ') -Wait -PassThru
if ($process.ExitCode -ne 0) {
    throw "Installer exited with code $($process.ExitCode). Inspect the installation before retrying."
}
$service = Get-Service -Name 'openuem-agent'
$service.WaitForStatus(
    [System.ServiceProcess.ServiceControllerStatus]::Running,
    [TimeSpan]::FromSeconds(60)
)
$service.Refresh()
if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::Running) {
    throw 'Agent service is not running. Inspect its logs before retrying.'
}
$service
```

The `/server` value is the NATS endpoint, which may differ from the console's
HTTPS address. This example accepts DNS names and IPv4 addresses, including a
comma-separated cluster list. Set both placement variables to the numeric
organization and site IDs from the console; the example appends both installer
switches together. Leaving both null deliberately selects default placement.
Any detected existing installation requires recovery or uninstallation before
using this fresh-install example; do not delete existing credentials to bypass it.

The silent switches suppress installer interaction and automatic reboot. A
nonzero result requires investigation; this example does not automatically
retry or treat a reboot-required result as success.
The service check fails if the agent does not reach Running within 60 seconds.

## Check admission, reporting and credentials

A successful installer exit and a running service are only local checks.

1. Inspect the installed credentials, including their effective file permissions.
   For a default installation, start with:

   ```powershell
   icacls 'C:\Program Files\OpenUEM Agent\certificates\agent.key'
   ```

   Confirm that ordinary users cannot read the private key. For a SYSTEM service,
   SYSTEM and administrators should retain access. If installation fails, inspect
   any credentials left in the destination as well as the staging directory.
2. In the console, find the expected endpoint, verify its organization/site and
   admit it. Confirm a recent inventory report from that endpoint.
3. Remove the source enrollment files and deployment archive when they are no
   longer required. Verify removal, and report cleanup errors without discarding
   the original installation error. Preserve the credentials used by the
   installed service.

For troubleshooting, inspect the installed agent's logs. Restrict access to logs
and review their contents before sharing them. Keep certificate validation enabled;
investigate trust, server name, time and network failures rather than bypassing TLS.

## Validation scope

This document is an installation checklist, not an automated enrollment service.
The example requires your Windows acceptance checks, including process argument
quoting, Authenticode, permissions, service startup, admission and reporting.
It does not establish per-device enrollment credentials, customer isolation or
production readiness. Validate those separately before fleet deployment.

For documentation changes, run `./scripts/Test-WindowsInstallDocs.ps1` from the
repository root. The documentation workflow runs this check in Windows PowerShell
5.1 and PowerShell 7. It parses both examples and tests the guide's placement and
server validation with valid and invalid inputs. It does not run the installer
or any registry, service, certificate or permissions commands.
