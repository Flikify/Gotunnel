[CmdletBinding()]
param(
  [string]$Server,
  [string]$Token,
  [string]$Version = 'latest',
  [string]$Repository = 'Flikify/Gotunnel',
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

function ConvertTo-GoTunnelYamlScalar {
  param(
    [Parameter(Mandatory = $true)][string]$Value
  )

  return '"' + ($Value.Replace('\', '\\').Replace('"', '\"')) + '"'
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

function Invoke-GoTunnelSc {
  param(
    [Parameter(Mandatory = $true)][string[]]$Arguments
  )

  $output = & sc.exe @Arguments 2>&1
  if ($LASTEXITCODE -ne 0) {
    $text = ($output | Out-String).Trim()
    throw "sc.exe $($Arguments -join ' ') failed: $text"
  }
  return $output
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
    [Parameter(Mandatory = $true)][string]$Arch
  )

  $release = Get-GoTunnelRelease -Repository $Repository -Version $Version
  $asset = $release.assets |
    Where-Object { $_.name -match "^gotunnel-client-.*-windows-$Arch\.zip$" } |
    Select-Object -First 1

  if (-not $asset) {
    throw "Failed to resolve GoTunnel client package for windows/$Arch from GitHub release $Version."
  }

  return @{
    DownloadUrl = $asset.browser_download_url
    Tag         = $release.tag_name
  }
}

function Write-GoTunnelConfigFile {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$Server,
    [Parameter(Mandatory = $true)][string]$Token,
    [Parameter(Mandatory = $true)][string]$DataDir
  )

  $yaml = @(
    "server: $(ConvertTo-GoTunnelYamlScalar -Value $Server)"
    "token: $(ConvertTo-GoTunnelYamlScalar -Value $Token)"
    "data_dir: $(ConvertTo-GoTunnelYamlScalar -Value $DataDir)"
    'reconnect_min_sec: 5'
    'reconnect_max_sec: 30'
  ) -join [Environment]::NewLine

  Set-Content -Path $Path -Value ($yaml + [Environment]::NewLine) -Encoding UTF8
}

function Install-GoTunnel {
  [CmdletBinding()]
  param(
    [Parameter(Mandatory = $true)][string]$Server,
    [Parameter(Mandatory = $true)][string]$Token,
    [string]$Version = 'latest',
    [string]$Repository = 'Flikify/Gotunnel',
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
  $configPath = Join-Path $resolvedDataDir 'client.yaml'
  $serviceLogPath = Join-Path $resolvedDataDir 'service.log'
  $targetPath = Join-Path $resolvedInstallDir 'gotunnel-client.exe'

  New-Item -ItemType Directory -Force -Path $resolvedInstallDir | Out-Null
  New-Item -ItemType Directory -Force -Path $resolvedDataDir | Out-Null
  New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

  $arch = Get-GoTunnelArch
  $asset = Get-GoTunnelDownloadUrl -Repository $Repository -Version $Version -Arch $arch

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

  $existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  if ($existingService) {
    Write-Host "Stopping existing service $ServiceName"
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    $existingService.WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped, (New-TimeSpan -Seconds 20))
  }

  Copy-Item -Path $binary.FullName -Destination $targetPath -Force
  Write-GoTunnelConfigFile -Path $configPath -Server $Server -Token $Token -DataDir $resolvedDataDir

  $serviceCommand = ('"{0}" -c "{1}" -service -service-name "{2}" -service-log-file "{3}"' -f $targetPath, $configPath, $ServiceName, $serviceLogPath)

  if (-not $existingService) {
    Write-Host "Creating Windows service $ServiceName"
    Invoke-GoTunnelSc -Arguments @('create', $ServiceName, ('binPath= {0}' -f $serviceCommand), 'start= auto', 'obj= LocalSystem', ('DisplayName= {0}' -f $DisplayName)) | Out-Null
  } else {
    Write-Host "Updating Windows service $ServiceName"
    Invoke-GoTunnelSc -Arguments @('config', $ServiceName, ('binPath= {0}' -f $serviceCommand), 'start= auto', 'obj= LocalSystem', ('DisplayName= {0}' -f $DisplayName)) | Out-Null
  }

  Invoke-GoTunnelSc -Arguments @('description', $ServiceName, 'GoTunnel client tunnel service managed by the installer.') | Out-Null
  Invoke-GoTunnelSc -Arguments @('failure', $ServiceName, 'reset=', '86400', 'actions=', 'restart/5000/restart/5000/restart/5000') | Out-Null
  Invoke-GoTunnelSc -Arguments @('failureflag', $ServiceName, '1') | Out-Null

  Start-Service -Name $ServiceName
  $startedService = Get-Service -Name $ServiceName

  Write-Host "GoTunnel client installed to $targetPath"
  Write-Host "Config written to $configPath"
  Write-Host "Service name: $ServiceName"
  Write-Host "Service account: LocalSystem"
  Write-Host "Startup type: Automatic"
  Write-Host "Service status: $($startedService.Status)"
}

if ($PSBoundParameters.ContainsKey('Server') -or $PSBoundParameters.ContainsKey('Token')) {
  if (-not $Server -or -not $Token) {
    throw 'Both -Server and -Token are required.'
  }

  Install-GoTunnel `
    -Server $Server `
    -Token $Token `
    -Version $Version `
    -Repository $Repository `
    -ServiceName $ServiceName `
    -DisplayName $DisplayName `
    -InstallDir $InstallDir `
    -DataDir $DataDir
}
