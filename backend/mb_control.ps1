# Moon Bridge control server (zero-build).
# Serves config_tool.html, owns the moonbridge child process, and exposes a
# small JSON API used by the panel to switch models / restart / regen Codex.
# Launched by 2_start_bridge.bat. Keep this window open; Ctrl+C stops everything.

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

# ---- constants -------------------------------------------------------------
$ControlPort = 38450
$Prefix      = "http://127.0.0.1:$ControlPort/"
$ConfigYml   = Join-Path $Root 'config.yml'
$ConfigJson  = Join-Path $Root 'mb_config.json'
$HtmlFile    = Join-Path $Root 'config_tool.html'
$Utf8        = New-Object System.Text.UTF8Encoding($false)   # no BOM

# Bridge invocation: prefer compiled exe, else fall back to `go run`.
# NOTE: NoDefaultCurrentDirectoryInExePath requires the .\ prefix.
if (Test-Path (Join-Path $Root 'moonbridge.exe')) {
  $MbCmd = Join-Path $Root 'moonbridge.exe'; $MbPre = @()
} else {
  $MbCmd = 'go'; $MbPre = @('run', './cmd/moonbridge')
}

# Codex home (read from mb_config.json, fallback to repo-local .codex)
$CodexHome = 'D:\moon_bridge_main\.codex'
# Global Codex home: the user's default ~/.codex (has ChatGPT auth.json). The
# native-GPT toggle edits THIS file, not the repo-local bridge home above.
$GlobalCodexHome = Join-Path $env:USERPROFILE '.codex'
$NativeCodexModel = 'gpt-5.5'
# Codex launcher (used by /api/open-codex). codexAppPath = the codex CLI to run
# `codex app` with (PATH name or absolute path); codexWorkdir = folder it opens.
# These are only startup defaults; the handler re-reads mb_config.json each call.
$CodexAppPath = 'codex'
$CodexWorkdir = ''
try {
  $cfg0 = [System.IO.File]::ReadAllText($ConfigJson, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
  if ($cfg0.codexHome)        { $CodexHome        = [string]$cfg0.codexHome }
  if ($cfg0.globalCodexHome)  { $GlobalCodexHome  = [string]$cfg0.globalCodexHome }
  if ($cfg0.nativeCodexModel) { $NativeCodexModel = [string]$cfg0.nativeCodexModel }
  if ($cfg0.codexAppPath)     { $CodexAppPath     = [string]$cfg0.codexAppPath }
  if ($cfg0.codexWorkdir)     { $CodexWorkdir     = [string]$cfg0.codexWorkdir }
} catch {}

$script:Bridge = $null
# Cached DeepSeek balance from the previous /api/deepseek-balance poll, so we can
# report "spend since last refresh" (= prevBalance - currentBalance).
$script:PrevBalance = $null

# ---- bridge process management --------------------------------------------
function Start-Bridge {
  $bargs = $MbPre + @('-config', 'config.yml')
  Write-Host "[control] starting bridge: $MbCmd $($bargs -join ' ')" -ForegroundColor Cyan
  # When hosted by MoonBridgeSwitcher.exe (which sets MB_NO_BROWSER), hide the
  # bridge's console window and tee its output to bridge.log so no command box
  # pops up. Standalone (2_start_bridge.bat) keeps the visible log window.
  $sp = @{ FilePath = $MbCmd; ArgumentList = $bargs; WorkingDirectory = $Root; PassThru = $true }
  if ($env:MB_NO_BROWSER) {
    $log = Join-Path $Root 'bridge.log'
    $sp['WindowStyle']            = 'Hidden'
    $sp['RedirectStandardOutput'] = $log
    $sp['RedirectStandardError']  = (Join-Path $Root 'bridge.err.log')
  }
  $p = Start-Process @sp
  $script:Bridge = $p
  return $p
}

function Stop-Bridge {
  if ($script:Bridge -and -not $script:Bridge.HasExited) {
    Write-Host "[control] stopping bridge pid $($script:Bridge.Id)" -ForegroundColor DarkYellow
    # /T kills the whole tree (covers `go run`'s temp child exe).
    cmd /c "taskkill /PID $($script:Bridge.Id) /T /F" 2>$null | Out-Null
  }
  $script:Bridge = $null
}

function Test-Listening {
  param([string]$HostName = '127.0.0.1', [int]$Port = 38440)
  $c = New-Object System.Net.Sockets.TcpClient
  try {
    $iar = $c.BeginConnect($HostName, $Port, $null, $null)
    if ($iar.AsyncWaitHandle.WaitOne(300)) { $c.EndConnect($iar); return $true }
    return $false
  } catch { return $false } finally { $c.Close() }
}

# ---- Codex config generation (mirrors 3_gen_codex_config.bat) --------------
function Invoke-RegenCodex {
  if (-not (Test-Path $CodexHome)) { New-Item -ItemType Directory -Path $CodexHome -Force | Out-Null }
  $model = (& $MbCmd @($MbPre + @('-print-codex-model', '-config', 'config.yml')) | Select-Object -First 1)
  if (-not $model) { throw 'could not read codex model alias from config.yml' }
  $model = $model.Trim()
  $toml = & $MbCmd @($MbPre + @('-print-codex-config', $model,
            '-codex-base-url', 'http://127.0.0.1:38440/v1',
            '-codex-home', $CodexHome, '-config', 'config.yml')) | Out-String
  if ($LASTEXITCODE -ne 0) { throw "moonbridge -print-codex-config failed (exit $LASTEXITCODE)" }
  [System.IO.File]::WriteAllText((Join-Path $CodexHome 'config.toml'), $toml, $Utf8)  # no BOM
  return @{ model = $model; codexHome = $CodexHome }
}

# ---- global Codex model toggle (native GPT <-> Moon Bridge) -----------------
# Edits the user's GLOBAL ~/.codex/config.toml so a plain `codex` run uses either
# the native GPT model (ChatGPT login) or routes through Moon Bridge. Only the
# model-routing keys + [model_providers.moonbridge] table are touched; [projects.*],
# [mcp_servers.*], [notice.*], personality, reasoning effort, etc. are preserved.
$GlobalCodexToml = Join-Path $GlobalCodexHome 'config.toml'

function Get-CodexMode {
  if (-not (Test-Path $GlobalCodexToml)) { return 'unknown' }
  $t = [System.IO.File]::ReadAllText($GlobalCodexToml, [System.Text.Encoding]::UTF8)
  if ($t -match '(?m)^\s*model_provider\s*=\s*"moonbridge"') { return 'bridge' }
  return 'native'
}

function Backup-GlobalCodexToml {
  if (Test-Path $GlobalCodexToml) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    Copy-Item $GlobalCodexToml "$GlobalCodexToml.mb-bak-$stamp" -Force
  }
}

# Drop the whole [model_providers.moonbridge] table (header until next [table] or EOF).
function Remove-MoonbridgeProviderTable([string[]]$lines) {
  $out = New-Object System.Collections.Generic.List[string]
  $skip = $false
  foreach ($ln in $lines) {
    if ($ln -match '^\s*\[model_providers\.moonbridge\]\s*$') { $skip = $true; continue }
    if ($skip) {
      if ($ln -match '^\s*\[') { $skip = $false } else { continue }  # next table ends the skip
    }
    $out.Add($ln)
  }
  return $out
}

function Set-CodexNative {
  if (-not (Test-Path $GlobalCodexToml)) { throw "global codex config not found: $GlobalCodexToml" }
  Backup-GlobalCodexToml
  $lines = [System.IO.File]::ReadAllText($GlobalCodexToml, [System.Text.Encoding]::UTF8) -split "`r?`n"
  $lines = Remove-MoonbridgeProviderTable $lines
  $out = New-Object System.Collections.Generic.List[string]
  foreach ($ln in $lines) {
    if ($ln -match '^\s*model\s*=')                   { $out.Add('model = "' + $NativeCodexModel + '"'); continue }
    if ($ln -match '^\s*model_provider\s*=')           { continue }
    if ($ln -match '^\s*model_context_window\s*=')     { continue }
    if ($ln -match '^\s*model_max_output_tokens\s*=')  { continue }
    if ($ln -match '^\s*model_catalog_json\s*=')       { continue }
    $out.Add($ln)
  }
  [System.IO.File]::WriteAllText($GlobalCodexToml, ($out -join "`r`n"), $Utf8)
  return @{ mode = 'native'; model = $NativeCodexModel; home = $GlobalCodexHome }
}

function Set-CodexBridge {
  if (-not (Test-Path $GlobalCodexToml)) { throw "global codex config not found: $GlobalCodexToml" }
  Backup-GlobalCodexToml
  $catalog = (Join-Path $CodexHome 'models_catalog.json')
  $lines = [System.IO.File]::ReadAllText($GlobalCodexToml, [System.Text.Encoding]::UTF8) -split "`r?`n"
  $lines = Remove-MoonbridgeProviderTable $lines   # rebuild it cleanly below
  # Drop existing routing keys so we re-emit them in a known-good block.
  $rest = New-Object System.Collections.Generic.List[string]
  foreach ($ln in $lines) {
    if ($ln -match '^\s*model\s*=')                   { continue }
    if ($ln -match '^\s*model_provider\s*=')           { continue }
    if ($ln -match '^\s*model_context_window\s*=')     { continue }
    if ($ln -match '^\s*model_max_output_tokens\s*=')  { continue }
    if ($ln -match '^\s*model_catalog_json\s*=')       { continue }
    $rest.Add($ln)
  }
  # Routing keys must sit at the top, before any [table] header.
  $head = @(
    'model = "moonbridge"'
    'model_provider = "moonbridge"'
    'model_context_window = 1000000'
    'model_max_output_tokens = 384000'
    'model_catalog_json = "' + ($catalog -replace '\\','\\') + '"'
  )
  $body = ($head + $rest) -join "`r`n"
  $providerBlock = @(
    ''
    '[model_providers.moonbridge]'
    'name = "Moon Bridge"'
    'base_url = "http://127.0.0.1:38440/v1"'
    'wire_api = "responses"'
  ) -join "`r`n"
  [System.IO.File]::WriteAllText($GlobalCodexToml, ($body.TrimEnd("`r","`n") + "`r`n" + $providerBlock + "`r`n"), $Utf8)
  return @{ mode = 'bridge'; home = $GlobalCodexHome }
}

# ---- DeepSeek balance ------------------------------------------------------
# Reads the active preset's api_key + base_url from mb_config.json, calls
# DeepSeek's GET /user/balance, and reports the balance plus spend-since-last-poll.
function Get-DeepseekBalance {
  $cfg = [System.IO.File]::ReadAllText($ConfigJson, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
  $preset = $cfg.presets.($cfg.active.preset)
  if (-not $preset) { throw 'no active preset in mb_config.json' }
  $key = [string]$preset.apiKey
  if (-not $key) { throw '当前模型未填写 API Key' }
  # Derive the API origin from base_url (e.g. https://api.deepseek.com/anthropic
  # -> https://api.deepseek.com), then append /user/balance.
  $base = [string]$preset.baseUrl
  if (-not $base) { $base = 'https://api.deepseek.com' }
  $uri = [Uri]$base
  $origin = $uri.Scheme + '://' + $uri.Authority
  $balUrl = $origin.TrimEnd('/') + '/user/balance'

  $resp = Invoke-RestMethod -Uri $balUrl -Method Get -TimeoutSec 15 -Headers @{
    'Authorization' = "Bearer $key"
    'Accept'        = 'application/json'
  }
  $info = $resp.balance_infos | Select-Object -First 1
  $balance  = [double]$info.total_balance
  $currency = [string]$info.currency

  $hasPrev = ($null -ne $script:PrevBalance)
  $lastSpend = 0.0
  if ($hasPrev) { $lastSpend = [math]::Round(($script:PrevBalance - $balance), 4) }
  $script:PrevBalance = $balance

  return @{
    ok        = $true
    balance   = $balance
    currency  = $currency
    available = [bool]$resp.is_available
    hasPrev   = $hasPrev
    lastSpend = $lastSpend
  }
}

# ---- open Codex desktop app ------------------------------------------------
# Launches `codex app <workdir>` with CODEX_HOME pointed at the repo-local .codex
# so it always routes through Moon Bridge -> DeepSeek (independent of the global
# native/bridge toggle). Path + workdir are read fresh from mb_config.json.
function Invoke-OpenCodex {
  $appPath = $CodexAppPath; $workdir = $CodexWorkdir
  try {
    $cfg = [System.IO.File]::ReadAllText($ConfigJson, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
    if ($cfg.codexAppPath) { $appPath = [string]$cfg.codexAppPath }
    if ($cfg.codexWorkdir) { $workdir = [string]$cfg.codexWorkdir }
  } catch {}
  if (-not $appPath) { $appPath = 'codex' }
  if (-not $workdir) { $workdir = $env:USERPROFILE }
  if (-not (Test-Path $workdir)) { throw "工作目录不存在: $workdir" }

  $env:CODEX_HOME = $CodexHome
  $p = Start-Process -FilePath $appPath -ArgumentList @('app', $workdir) -PassThru
  return @{ ok = $true; path = $appPath; workdir = $workdir; codexHome = $CodexHome; pid = $p.Id }
}

# ---- HTTP helpers ----------------------------------------------------------
function Send-Bytes($ctx, [byte[]]$body, [string]$type, [int]$code = 200) {
  $ctx.Response.StatusCode = $code
  $ctx.Response.ContentType = $type
  $ctx.Response.ContentLength64 = $body.Length
  $ctx.Response.OutputStream.Write($body, 0, $body.Length)
  $ctx.Response.OutputStream.Close()
}
function Send-Json($ctx, $obj, [int]$code = 200) {
  $json = ($obj | ConvertTo-Json -Depth 20 -Compress)
  Send-Bytes $ctx $Utf8.GetBytes($json) 'application/json; charset=utf-8' $code
}
function Read-Body($ctx) {
  # The panel always POSTs UTF-8 JSON. fetch() omits charset, so HttpListener's
  # Request.ContentEncoding falls back to the OS ANSI codepage (e.g. GBK on
  # zh-CN Windows), which mangles Chinese and can even swallow the closing quote
  # -> ConvertFrom-Json fails. Decode as UTF-8 explicitly.
  $enc = $ctx.Request.ContentEncoding
  if (-not $enc -or $enc.CodePage -ne 65001) { $enc = New-Object System.Text.UTF8Encoding($false) }
  $sr = New-Object System.IO.StreamReader($ctx.Request.InputStream, $enc)
  try { return $sr.ReadToEnd() } finally { $sr.Close() }
}

# ---- start everything ------------------------------------------------------
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add($Prefix)
try { $listener.Start() }
catch {
  Write-Host "[control] FAILED to bind $Prefix - port $ControlPort in use?" -ForegroundColor Red
  Write-Host $_.Exception.Message -ForegroundColor Red
  Read-Host 'Press Enter to exit'; exit 1
}

Start-Bridge | Out-Null
Write-Host "============================================" -ForegroundColor Green
Write-Host "  Moon Bridge control panel: $Prefix" -ForegroundColor Green
Write-Host "  Bridge target: 127.0.0.1:38440" -ForegroundColor Green
Write-Host "  KEEP THIS WINDOW OPEN. Ctrl+C stops the bridge + panel." -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
if (-not $env:MB_NO_BROWSER) { Start-Process $Prefix | Out-Null }   # open default browser (skip when hosted by exe)

# ---- request loop ----------------------------------------------------------
try {
  while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $path = $ctx.Request.Url.AbsolutePath
    $verb = $ctx.Request.HttpMethod
    try {
      switch -regex ($path) {
        '^/$|^/index\.html$' {
          Send-Bytes $ctx ([System.IO.File]::ReadAllBytes($HtmlFile)) 'text/html; charset=utf-8'; break
        }
        '^/api/state$' {
          Send-Bytes $ctx ([System.IO.File]::ReadAllBytes($ConfigJson)) 'application/json; charset=utf-8'; break
        }
        '^/api/status$' {
          $alive = ($script:Bridge -and -not $script:Bridge.HasExited)
          Send-Json $ctx @{ ok = $true; processAlive = [bool]$alive; listening = (Test-Listening); addr = '127.0.0.1:38440' }; break
        }
        '^/api/save$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          $payload = Read-Body $ctx | ConvertFrom-Json
          if (-not $payload.yaml) { Send-Json $ctx @{ ok=$false; message='missing yaml' } 400; break }
          if (Test-Path $ConfigYml) { Copy-Item $ConfigYml "$ConfigYml.bak" -Force }
          [System.IO.File]::WriteAllText($ConfigYml, [string]$payload.yaml, $Utf8)
          $jsonOut = ($payload.json | ConvertTo-Json -Depth 20)
          [System.IO.File]::WriteAllText($ConfigJson, $jsonOut, $Utf8)
          Send-Json $ctx @{ ok=$true; message='saved' }; break
        }
        '^/api/restart$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          Stop-Bridge; Start-Sleep -Milliseconds 400; $p = Start-Bridge
          Send-Json $ctx @{ ok=$true; pid=$p.Id }; break
        }
        '^/api/regen-codex$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          $r = Invoke-RegenCodex
          Send-Json $ctx @{ ok=$true; model=$r.model; output="config.toml + models_catalog.json -> $($r.codexHome)" }; break
        }
        '^/api/codex-mode$' {
          Send-Json $ctx @{ ok=$true; mode=(Get-CodexMode); home=$GlobalCodexHome; nativeModel=$NativeCodexModel }; break
        }
        '^/api/codex-native$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          $r = Set-CodexNative
          Send-Json $ctx @{ ok=$true; mode=$r.mode; model=$r.model; home=$r.home }; break
        }
        '^/api/codex-bridge$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          $r = Set-CodexBridge
          Send-Json $ctx @{ ok=$true; mode=$r.mode; home=$r.home }; break
        }
        '^/api/deepseek-balance$' {
          $r = Get-DeepseekBalance
          Send-Json $ctx $r; break
        }
        '^/api/open-codex$' {
          if ($verb -ne 'POST') { Send-Json $ctx @{ ok=$false; message='POST only' } 405; break }
          $r = Invoke-OpenCodex
          Send-Json $ctx $r; break
        }
        default { Send-Json $ctx @{ ok=$false; message="not found: $path" } 404 }
      }
    } catch {
      Write-Host "[control] $verb $path -> ERROR: $($_.Exception.Message)" -ForegroundColor Red
      try { Send-Json $ctx @{ ok=$false; message=$_.Exception.Message } 500 } catch {}
    }
  }
} finally {
  Write-Host "[control] shutting down..." -ForegroundColor DarkYellow
  try { $listener.Stop() } catch {}
  Stop-Bridge
}
