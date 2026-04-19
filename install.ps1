param(
    [string]$Version = "",
    [string]$Repo = "sajidmehmoodtariq-dev/ShellAI",
    [string]$InstallDir = "",
    [string]$BaseUrl = "",
    [string]$ApiUrl = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $env:SHELLAI_VERSION
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "latest"
}

if ([string]::IsNullOrWhiteSpace($Repo)) {
    if (-not [string]::IsNullOrWhiteSpace($env:SHELLAI_REPO)) {
        $Repo = $env:SHELLAI_REPO
    } else {
        $Repo = "sajidmehmoodtariq-dev/ShellAI"
    }
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    if (-not [string]::IsNullOrWhiteSpace($env:SHELLAI_INSTALL_DIR)) {
        $InstallDir = $env:SHELLAI_INSTALL_DIR
    } else {
        $InstallDir = Join-Path $env:USERPROFILE "bin"
    }
}

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    if (-not [string]::IsNullOrWhiteSpace($env:SHELLAI_BASE_URL)) {
        $BaseUrl = $env:SHELLAI_BASE_URL
    } else {
        $BaseUrl = "https://github.com/$Repo/releases/download"
    }
}

if ([string]::IsNullOrWhiteSpace($ApiUrl)) {
    if (-not [string]::IsNullOrWhiteSpace($env:SHELLAI_API_URL)) {
        $ApiUrl = $env:SHELLAI_API_URL
    } else {
        $ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
    }
}

function Resolve-Arch {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64") {
        return "arm64"
    }
    return "amd64"
}

function Get-LatestVersion {
    param([string]$Url)
    $resp = Invoke-RestMethod -Uri $Url
    if (-not $resp.tag_name) {
        throw "Could not determine latest release tag from $Url"
    }
    return [string]$resp.tag_name
}

$arch = Resolve-Arch

if ($Version -eq "latest") {
    Write-Host "Resolving latest release..."
    $Version = Get-LatestVersion -Url $ApiUrl
}

$assetName = "shellai-$Version-windows-$arch.exe"
$checksumName = "SHA256SUMS"
$assetUrl = "$BaseUrl/$Version/$assetName"
$checksumUrl = "$BaseUrl/$Version/$checksumName"

Write-Host "ShellAI Windows installer"
Write-Host "Version: $Version"
Write-Host "Asset:   $assetUrl"
Write-Host "Install: $InstallDir\shellai.exe"

if ($DryRun) {
    Write-Host "Dry-run mode enabled. Exiting before download."
    exit 0
}

$tmpRoot = Join-Path $env:TEMP ("shellai-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpRoot -Force | Out-Null

try {
    $assetPath = Join-Path $tmpRoot $assetName
    $checksumPath = Join-Path $tmpRoot $checksumName

    Invoke-WebRequest -Uri $assetUrl -OutFile $assetPath
    Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath

    $line = Select-String -Path $checksumPath -Pattern ([regex]::Escape($assetName)) | Select-Object -First 1
    if (-not $line) {
        throw "Checksum entry for $assetName not found in $checksumName"
    }

    $parts = ($line.Line -split "\s+") | Where-Object { $_ -ne "" }
    if ($parts.Count -lt 2) {
        throw "Malformed checksum line: $($line.Line)"
    }

    $expectedHash = $parts[0].ToLowerInvariant()
    $actualHash = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLowerInvariant()

    if ($expectedHash -ne $actualHash) {
        throw "Checksum mismatch for $assetName. Expected $expectedHash, got $actualHash"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destExe = Join-Path $InstallDir "shellai.exe"
    Copy-Item -Path $assetPath -Destination $destExe -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ([string]::IsNullOrWhiteSpace($userPath)) {
        $userPath = ""
    }

    if ($userPath -notlike "*${InstallDir}*") {
        $updatedPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable("Path", $updatedPath, "User")
        $env:Path = "$InstallDir;$env:Path"
        Write-Host "Added $InstallDir to your user PATH."
    }

    Write-Host "Installed ShellAI to $destExe"
    & $destExe --version

    Write-Host ""
    Write-Host "If 'shellai' is not recognized in your current terminal, open a new PowerShell window."
    Write-Host "You can always run it directly with: $destExe"
} finally {
    Remove-Item -Path $tmpRoot -Recurse -Force -ErrorAction SilentlyContinue
}
