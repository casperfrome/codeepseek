#Requires -Version 5.1
<#
.SYNOPSIS
    One-click build for Moon Bridge Switcher.

.DESCRIPTION
    Builds both halves of the project into a single output folder:
      1. control/  -> mbcontrol.exe   (Go control backend, stdlib only)
      2. launcher/ -> MoonBridgeSwitcher.exe (C# WinForms + WebView2, self-contained single-file)

    Both end up side by side in .\publish, which is exactly where the launcher
    expects to find mbcontrol.exe.

.PARAMETER Output
    Output directory for the built binaries. Default: .\publish

.PARAMETER Clean
    Remove the output directory before building.

.PARAMETER SkipControl
    Skip building the Go control backend (mbcontrol.exe).

.PARAMETER SkipLauncher
    Skip publishing the C# launcher (MoonBridgeSwitcher.exe).

.EXAMPLE
    .\build.ps1
    Build everything into .\publish

.EXAMPLE
    .\build.ps1 -Clean -Output dist
    Wipe .\dist, then build everything into it.
#>
[CmdletBinding()]
param(
    [string]$Output = 'publish',
    [switch]$Clean,
    [switch]$SkipControl,
    [switch]$SkipLauncher
)

# --- Make console + child-process output UTF-8 so non-ASCII text renders correctly ---
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
try { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false) } catch {}

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Always operate relative to this script's location, regardless of caller's CWD.
$Root = $PSScriptRoot
if (-not $Root) { $Root = (Get-Location).Path }

function Write-Step  { param([string]$m) Write-Host "==> $m" -ForegroundColor Cyan }
function Write-Ok    { param([string]$m) Write-Host "    $m" -ForegroundColor Green }
function Write-Warn2 { param([string]$m) Write-Host "    $m" -ForegroundColor Yellow }

function Test-Tool {
    param([string]$Name, [string]$Hint)
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $cmd) {
        throw "Required tool '$Name' was not found on PATH. $Hint"
    }
    return $cmd.Source
}

$sw = [System.Diagnostics.Stopwatch]::StartNew()

# --- Resolve / prepare output directory ---
$OutDir = if ([System.IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path $Root $Output }

if ($Clean -and (Test-Path $OutDir)) {
    Write-Step "Cleaning $OutDir"
    Remove-Item -Recurse -Force $OutDir
}
if (-not (Test-Path $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}
$OutDir = (Resolve-Path $OutDir).Path

Write-Host ''
Write-Host 'Moon Bridge Switcher - one-click build' -ForegroundColor White
Write-Host "Output: $OutDir" -ForegroundColor DarkGray
Write-Host ''

# --- 1. Go control backend -> mbcontrol.exe ---
if (-not $SkipControl) {
    Write-Step 'Building Go control backend (mbcontrol.exe)'
    $go = Test-Tool 'go' 'Install Go 1.25+ from https://go.dev/dl/ and reopen the terminal.'
    Write-Ok "using $go"

    $controlDir = Join-Path $Root 'control'
    $mbExe = Join-Path $OutDir 'mbcontrol.exe'

    Push-Location $controlDir
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "go vet failed (exit $LASTEXITCODE)." }
        & go build -o $mbExe .
        if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)." }
    }
    finally {
        Pop-Location
    }
    Write-Ok "-> $mbExe"
}
else {
    Write-Warn2 'Skipping Go control backend (-SkipControl).'
}

# --- 2. C# launcher -> MoonBridgeSwitcher.exe ---
if (-not $SkipLauncher) {
    Write-Step 'Publishing C# launcher (MoonBridgeSwitcher.exe)'
    $dotnet = Test-Tool 'dotnet' 'Install the .NET 8 SDK from https://dotnet.microsoft.com/download/dotnet/8.0 and reopen the terminal.'
    Write-Ok "using $dotnet"

    $csproj = Join-Path $Root 'launcher\MoonBridgeSwitcher.csproj'
    & dotnet publish $csproj -c Release -r win-x64 -o $OutDir
    if ($LASTEXITCODE -ne 0) { throw "dotnet publish failed (exit $LASTEXITCODE)." }
    Write-Ok "-> $(Join-Path $OutDir 'MoonBridgeSwitcher.exe')"
}
else {
    Write-Warn2 'Skipping C# launcher (-SkipLauncher).'
}

$sw.Stop()

# --- Summary ---
Write-Host ''
Write-Step 'Build complete'
foreach ($name in 'MoonBridgeSwitcher.exe', 'mbcontrol.exe') {
    $p = Join-Path $OutDir $name
    if (Test-Path $p) {
        $sizeMB = [math]::Round((Get-Item $p).Length / 1MB, 1)
        Write-Ok ("{0,-26} {1,6} MB" -f $name, $sizeMB)
    }
}
Write-Host ''
Write-Host ("Done in {0:N1}s. Run {1}" -f $sw.Elapsed.TotalSeconds, (Join-Path $OutDir 'MoonBridgeSwitcher.exe')) -ForegroundColor Green
