[CmdletBinding()]
param(
  [string]$Server,
  [string]$Token,
  [string]$Version = 'latest',
  [string]$Repository = 'Flikify/Gotunnel',
  [string]$Cdn = '',
  [string]$ServiceName = 'GoTunnelClient',
  [string]$DisplayName = 'GoTunnel Client',
  [string]$InstallDir = '',
  [string]$DataDir = ''
)

$ErrorActionPreference = 'Stop'

function Test-GoTunnelAdministrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($identity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-GoTunnelArch {
  switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()) {
    'x64' { return 'amd64' }
    'arm64' { return 'arm64' }
    'x86' { return '386' }
    default { throw "Unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
  }
}

function Get-GoTunnelDefaultInstallDir {
  param(
    [string]$Preferred = ''
  )

  if ($Preferred) {
    return $Preferred
  }
  return Join-Path ${env:ProgramFiles} 'GoTunnel'
}

function Get-GoTunnelDefaultDataDir {
  param(
    [string]$Preferred = ''
  )

  if ($Preferred) {
    return $Preferred
  }
  return Join-Path ${env:ProgramData} 'GoTunnel'
}

function Ensure-GoTunnelAdministrator {
  if (Test-GoTunnelAdministrator) {
    return
  }

  if (-not $PSCommandPath) {
    throw 'Installer must be executed from a file so it can self-elevate.'
  }

  $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"{0}"' -f $PSCommandPath))
  foreach ($entry in $MyInvocation.BoundParameters.GetEnumerator()) {
    $argList += ('-{0}' -f $entry.Key)
    $argList += ('"{0}"' -f ($entry.Value.ToString().Replace('"', '\"')))
  }

  Start-Process -FilePath 'powershell.exe' -Verb RunAs -ArgumentList $argList
  exit 0
}

function Join-GoTunnelCdnUrl {
  param(
    [Parameter(Mandatory = $true)][string]$Url,
    [string]$Prefix = ''
  )

  if (-not $Prefix) {
    return $Url
  }
  return ('{0}/{1}' -f $Prefix.TrimEnd('/'), $Url)
}

function Get-GoTunnelRelease {
  param(
    [Parameter(Mandatory = $true)][string]$Repository,
    [Parameter(Mandatory = $true)][string]$Version
  )

  $uri = if ($Version -eq 'latest') {
    "https://api.github.com/repos/$Repository/releases/latest"
  } else {
    "https://api.github.com/repos/$Repository/releases/tags/$Version"
  }

  return Invoke-RestMethod -Uri $uri -Headers @{
    'Accept' = 'application/vnd.github+json'
    'X-GitHub-Api-Version' = '2022-11-28'
    'User-Agent' = 'GoTunnel-Installer'
  }
}

function Get-GoTunnelDownloadUrl {
  param(
    [Parameter(Mandatory = $true)][string]$Repository,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Arch,
    [string]$Cdn = ''
  )

  $release = Get-GoTunnelRelease -Repository $Repository -Version $Version
  $asset = $release.assets |
    Where-Object { $_.name -match "^gotunnel-client-.*-windows-$Arch\.zip$" } |
    Select-Object -First 1

  if (-not $asset) {
    throw "Failed to resolve GoTunnel client package for windows/$Arch from GitHub release $Version."
  }

  return @{
    DownloadUrl = Join-GoTunnelCdnUrl -Url $asset.browser_download_url -Prefix $Cdn
    Tag         = $release.tag_name
  }
}

function Install-GoTunnel {
  [CmdletBinding()]
  param(
    [string]$Server,
    [string]$Token,
    [string]$Version = 'latest',
    [string]$Repository = 'Flikify/Gotunnel',
    [string]$Cdn = '',
    [string]$ServiceName = 'GoTunnelClient',
    [string]$DisplayName = 'GoTunnel Client',
    [string]$InstallDir = '',
    [string]$DataDir = ''
  )

  Ensure-GoTunnelAdministrator

  $resolvedInstallDir = Get-GoTunnelDefaultInstallDir -Preferred $InstallDir
  $resolvedDataDir = Get-GoTunnelDefaultDataDir -Preferred $DataDir
  $downloadDir = Join-Path $resolvedDataDir 'downloads'
  $extractDir = Join-Path $resolvedDataDir 'extract'
  $archivePath = Join-Path $downloadDir 'gotunnel-client.zip'
  $serviceLogPath = Join-Path $resolvedDataDir 'service.log'
  $targetPath = Join-Path $resolvedInstallDir 'gotunnel-client.exe'
  $configPath = Join-Path $resolvedDataDir 'client.yaml'

  New-Item -ItemType Directory -Force -Path $resolvedInstallDir | Out-Null
  New-Item -ItemType Directory -Force -Path $resolvedDataDir | Out-Null
  New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

  $arch = Get-GoTunnelArch
  $asset = Get-GoTunnelDownloadUrl -Repository $Repository -Version $Version -Arch $arch -Cdn $Cdn

  Write-Host "Downloading GoTunnel client $($asset.Tag) from $($asset.DownloadUrl)"
  Invoke-WebRequest -Uri $asset.DownloadUrl -OutFile $archivePath -MaximumRedirection 5

  if (Test-Path $extractDir) {
    Remove-Item -Path $extractDir -Recurse -Force
  }
  Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

  $binary = Get-ChildItem -Path $extractDir -Recurse -File |
    Where-Object { $_.Name -eq 'gotunnel-client.exe' } |
    Select-Object -First 1

  if (-not $binary) {
    throw 'Failed to find extracted client binary.'
  }

  Copy-Item -Path $binary.FullName -Destination $targetPath -Force

  Write-Host "Installing Windows service $ServiceName via client command"
  $serviceArgs = @(
    'service',
    'install',
    '-data-dir', $resolvedDataDir,
    '-service-name', $ServiceName,
    '-service-display-name', $DisplayName,
    '-service-log-file', $serviceLogPath
  )
  if ($Server) {
    $serviceArgs += @('-s', $Server)
  }
  if ($Token) {
    $serviceArgs += @('-t', $Token)
  }
  & $targetPath @serviceArgs
  if ($LASTEXITCODE -ne 0) {
    throw "gotunnel-client service install command failed with exit code $LASTEXITCODE"
  }
  $startedService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue

  Write-Host "GoTunnel client installed to $targetPath"
  Write-Host "Config written to $configPath"
  Write-Host "Service name: $ServiceName"
  Write-Host "Service account: LocalSystem"
  Write-Host "Startup type: Automatic"
  Write-Host "Service status: $($startedService.Status)"
}

if ($PSCommandPath -and $MyInvocation.InvocationName -ne '.') {
  Install-GoTunnel `
    -Server $Server `
    -Token $Token `
    -Version $Version `
    -Repository $Repository `
    -Cdn $Cdn `
    -ServiceName $ServiceName `
    -DisplayName $DisplayName `
    -InstallDir $InstallDir `
    -DataDir $DataDir
}
