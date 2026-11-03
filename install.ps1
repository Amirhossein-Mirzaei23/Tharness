# install.ps1 — install the tharness release binary on Windows (PowerShell).
#
# Usage:
#   .\install.ps1                              # installs to $env:LOCALAPPDATA\Programs\tharness
#   $env:PREFIX = "D:\bin"; .\install.ps1
#   $env:THARNESS_VERSION = "0.0.1"; .\install.ps1
#
# Downloads the release asset and checksums.txt from GitHub Releases,
# verifies the sha256, and installs the binary as tharness.exe on the user PATH.
$ErrorActionPreference = "Stop"

$Repo = "Amirhossein-Mirzaei23/Tharness"
$Version = if ($env:THARNESS_VERSION) { $env:THARNESS_VERSION } else { "0.0.1" }
$Prefix = if ($env:PREFIX) { $env:PREFIX } else { Join-Path $env:LOCALAPPDATA "Programs\tharness" }

$arch = $env:PROCESS_ARCHITECTURE
if (-not $arch) {
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { Write-Error "install: unsupported arch: $env:PROCESSOR_ARCHITECTURE" }
    }
}

$Asset = "tharness_${Version}_windows_${arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/v$Version"
$Tmp = Join-Path $env:TEMP ("tharness-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null

Write-Host "install: downloading $Asset"
try {
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile (Join-Path $Tmp $Asset)
    Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile (Join-Path $Tmp "checksums.txt")
} catch {
    Write-Error "install: download failed - is release v$Version published? ($_)"
}

Write-Host "install: verifying checksum"
$Want = (Select-String -Path (Join-Path $Tmp "checksums.txt") -Pattern ([regex]::Escape($Asset)) |
    Select-Object -First 1).Line.Split(" ")[0]
if (-not $Want) { Write-Error "install: $Asset missing from checksums.txt" }
$Hash = (Get-FileHash -Algorithm SHA256 (Join-Path $Tmp $Asset)).Hash.ToLower()
if ($Hash -ne $Want.ToLower()) { Write-Error "install: checksum mismatch ($Hash != $Want)" }

Expand-Archive -Path (Join-Path $Tmp $Asset) -DestinationPath $Tmp -Force
if (-not (Test-Path (Join-Path $Tmp "tharness.exe"))) { Write-Error "install: archive missing tharness.exe" }

New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
Copy-Item (Join-Path $Tmp "tharness.exe") (Join-Path $Prefix "tharness.exe") -Force

# Add to the user PATH if not already present.
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($UserPath -split ";") -notcontains $Prefix) {
    [Environment]::SetEnvironmentVariable("Path", ($UserPath.TrimEnd(";") + ";" + $Prefix), "User")
    Write-Host "install: added $Prefix to user PATH (open a new terminal for it to apply)"
}

Write-Host "install: tharness $Version installed to $Prefix\tharness.exe"
& (Join-Path $Prefix "tharness.exe") --version
Write-Host "next: run 'tharness setup' then 'tharness' (in a NEW terminal)"

Remove-Item -Recurse -Force $Tmp
