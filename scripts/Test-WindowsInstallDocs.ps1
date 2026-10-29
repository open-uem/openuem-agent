# Parse the examples without running the installer, registry, service or ACL commands.
[CmdletBinding()]
param(
    [string]$DocumentPath = (Join-Path $PSScriptRoot '../docs/windows-unattended-install.md')
)

$ErrorActionPreference = 'Stop'
$document = [System.IO.File]::ReadAllText((Resolve-Path -LiteralPath $DocumentPath).Path)
$blocks = [regex]::Matches($document, '(?ms)^[ \t]*```powershell[ \t]*\r?\n(?<code>.*?)^[ \t]*```[ \t]*\r?$')
if ($blocks.Count -ne 2) { throw 'Expected exactly two PowerShell examples.' }

foreach ($block in $blocks) {
    $tokens = $null
    $parseErrors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseInput(
        $block.Groups['code'].Value, [ref]$tokens, [ref]$parseErrors
    )
    if ($parseErrors.Count -gt 0) { throw "Example has parser errors: $parseErrors" }
}

# Prove that the parser check can detect malformed PowerShell.
$tokens = $null
$parseErrors = $null
$null = [System.Management.Automation.Language.Parser]::ParseInput(
    'if (', [ref]$tokens, [ref]$parseErrors
)
if ($parseErrors.Count -eq 0) { throw 'Malformed PowerShell control was accepted.' }

# Exercise the current guide's pure validation, never a separate copy of its logic.
# Fail closed if an edit removes or duplicates either extraction boundary.
$example = $blocks[0].Groups['code'].Value
$startMarker = '$hasTenant = '
$endMarker = 'if ($expectedSha256 -notmatch '
$start = $example.IndexOf($startMarker, [StringComparison]::Ordinal)
$end = $example.IndexOf($endMarker, [StringComparison]::Ordinal)
if ($start -lt 0 -or $end -le $start -or
    $start -ne $example.LastIndexOf($startMarker, [StringComparison]::Ordinal) -or
    $end -ne $example.LastIndexOf($endMarker, [StringComparison]::Ordinal)) {
    throw 'Cannot uniquely locate the guide input-validation block.'
}
$validation = [scriptblock]::Create($example.Substring($start, $end - $start))
$cases = @(
    @{ name = 'default placement'; tenant = $null; site = $null; server = 'uem.example.com:4433'; pass = $true },
    @{ name = 'explicit placement and cluster'; tenant = '12'; site = '34'; server = 'uem.example.com:4433,node2:4433'; pass = $true },
    @{ name = 'IPv4'; tenant = '1'; site = '2'; server = '192.0.2.1:4433'; pass = $true },
    @{ name = 'missing site'; tenant = '1'; site = $null; server = 'uem.example.com:4433'; pass = $false },
    @{ name = 'missing tenant'; tenant = $null; site = '2'; server = 'uem.example.com:4433'; pass = $false },
    @{ name = 'placement switch injection'; tenant = '1 /key=bad'; site = '2'; server = 'uem.example.com:4433'; pass = $false },
    @{ name = 'zero tenant'; tenant = '0'; site = '2'; server = 'uem.example.com:4433'; pass = $false },
    @{ name = 'server switch injection'; tenant = '1'; site = '2'; server = 'uem.example.com:4433 /key=bad'; pass = $false },
    @{ name = 'server quote'; tenant = '1'; site = '2'; server = 'uem.example.com:4433"'; pass = $false },
    @{ name = 'server URL'; tenant = '1'; site = '2'; server = 'https://uem.example.com:4433'; pass = $false },
    @{ name = 'zero port'; tenant = '1'; site = '2'; server = 'uem.example.com:0'; pass = $false },
    @{ name = 'port overflow'; tenant = '1'; site = '2'; server = 'uem.example.com:65536'; pass = $false },
    @{ name = 'trailing comma'; tenant = '1'; site = '2'; server = 'uem.example.com:4433,'; pass = $false },
    @{ name = 'trailing LF'; tenant = '1'; site = '2'; server = "uem.example.com:4433`n"; pass = $false },
    @{ name = 'trailing CRLF'; tenant = '1'; site = '2'; server = "uem.example.com:4433`r`n"; pass = $false },
    @{ name = 'tenant LF'; tenant = "1`n"; site = '2'; server = 'uem.example.com:4433'; pass = $false },
    @{ name = 'leading hyphen'; tenant = '1'; site = '2'; server = '-bad.example.com:4433'; pass = $false },
    @{ name = 'empty DNS label'; tenant = '1'; site = '2'; server = 'bad..example.com:4433'; pass = $false },
    @{ name = 'trailing label hyphen'; tenant = '1'; site = '2'; server = 'example-.com:4433'; pass = $false },
    @{ name = 'IPv4 octet overflow'; tenant = '1'; site = '2'; server = '256.0.0.1:4433'; pass = $false },
    @{ name = 'IPv4 shorthand'; tenant = '1'; site = '2'; server = '127.1:4433'; pass = $false },
    @{ name = 'IPv4 leading zeros'; tenant = '1'; site = '2'; server = '192.000.2.1:4433'; pass = $false },
    @{ name = 'maximum port'; tenant = '1'; site = '2'; server = '0.0.0.0:65535'; pass = $true }
)
foreach ($case in $cases) {
    # A fresh child scope for each case prevents variables leaking between cases.
    $passed = & {
        $tenantId = $case.tenant
        $siteId = $case.site
        $natsServer = $case.server
        try {
            $null = & $validation
            $true
        } catch {
            $false
        }
    }
    if ($passed -ne $case.pass) { throw "Input validation case failed: $($case.name)" }
}
Write-Output "PASS: $($blocks.Count) parsed examples, malformed-syntax control, $($cases.Count) input-validation cases."
