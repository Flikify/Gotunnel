# GoTunnel Build Script for Windows
# Usage: .\build.ps1 [command]
# Commands: all, current, web, server, client, android, clean, help

param(
    [Parameter(Position=0)]
    [string]$Command = "all",

    [string]$Version = "",

    [switch]$NoUPX
)

$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
$BuildDir = Join-Path $RootDir "build"
$env:GOCACHE = if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path $BuildDir ".gocache" }

$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd HH:mm:ss")
try {
    $GitCommit = (git -C $RootDir rev-parse --short HEAD 2>$null)
    if (-not $GitCommit) { $GitCommit = "unknown" }
} catch {
    $GitCommit = "unknown"
}

$DesktopPlatforms = @(
    @{ OS = "windows"; Arch = "amd64" },
    @{ OS = "windows"; Arch = "arm64" },
    @{ OS = "linux"; Arch = "amd64" },
    @{ OS = "linux"; Arch = "arm64" },
    @{ OS = "darwin"; Arch = "amd64" },
    @{ OS = "darwin"; Arch = "arm64" }
)

function Normalize-Version {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return "v0.0.0-dev"
    }

    if ($Value.StartsWith("v", [System.StringComparison]::OrdinalIgnoreCase)) {
        return $Value
    }

    if ($Value -match '^\d+(\.\d+){1,3}([-.+].*)?$') {
        return "v$Value"
    }

    return $Value
}

function Get-ResolvedVersion {
    if (-not [string]::IsNullOrWhiteSpace($Version)) {
        return Normalize-Version $Version
    }

    try {
        $ExactTag = (git -C $RootDir describe --tags --exact-match 2>$null)
        if ($ExactTag) {
            return Normalize-Version $ExactTag
        }
    } catch {}

    $LatestTag = ""
    try {
        $LatestTag = (git -C $RootDir describe --tags --abbrev=0 2>$null)
    } catch {}

    if ($LatestTag) {
        $NormalizedTag = Normalize-Version $LatestTag
        if ($GitCommit -and $GitCommit -ne "unknown") {
            return "$NormalizedTag-dev+$GitCommit"
        }
        return "$NormalizedTag-dev"
    }

    if ($GitCommit -and $GitCommit -ne "unknown") {
        return "v0.0.0-dev+$GitCommit"
    }

    return "v0.0.0-dev"
}

$Version = Get-ResolvedVersion

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Write-Err {
    param([string]$Message)
    Write-Host "[ERROR] " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Test-UPX {
    try {
        $null = Get-Command upx -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

function Compress-Binary {
    param(
        [string]$FilePath,
        [string]$OS
    )

    if ($NoUPX) { return }
    if (-not (Test-UPX)) {
        Write-Warn "UPX not found, skipping compression"
        return
    }
    if ($OS -eq "darwin") {
        Write-Warn "Skipping UPX for macOS binary: $FilePath"
        return
    }

    Write-Info "Compressing $FilePath with UPX..."
    try {
        & upx -9 -q $FilePath 2>$null
    } catch {
        Write-Warn "UPX compression failed for $FilePath"
    }
}

function Build-Web {
    Write-Info "Generating Swagger docs..."
    & go generate (Join-Path $RootDir "cmd\server")
    if ($LASTEXITCODE -ne 0) { throw "swagger generation failed" }

    Write-Info "Building web UI..."
    $LegacyDistDir = Join-Path $RootDir "web\dist"
    if (Test-Path $LegacyDistDir) {
        Remove-Item -Recurse -Force $LegacyDistDir
    }

    $WebDir = Join-Path $RootDir "web"
    Push-Location $WebDir

    try {
        if (-not (Test-Path "node_modules")) {
            Write-Info "Installing npm dependencies..."
            & npm install
            if ($LASTEXITCODE -ne 0) { throw "npm install failed" }
        }

        & npm run build
        if ($LASTEXITCODE -ne 0) { throw "npm build failed" }
    } finally {
        Pop-Location
    }

    Write-Info "Web UI built successfully"
}

function Get-OutputName {
    param(
        [string]$Component,
        [string]$OS
    )

    if ($OS -eq "windows") {
        return "$Component.exe"
    }

    return $Component
}

function Build-Binary {
    param(
        [string]$OS,
        [string]$Arch,
        [string]$Component
    )

    $OutputDir = Join-Path $BuildDir "${OS}_${Arch}"
    if (-not (Test-Path $OutputDir)) {
        New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
    }

    $OutputName = Get-OutputName -Component $Component -OS $OS
    $OutputPath = Join-Path $OutputDir $OutputName
    $SourcePath = Join-Path $RootDir "cmd\$Component"

    Write-Info "Building $Component for $OS/$Arch..."

    $env:GOOS = $OS
    $env:GOARCH = $Arch
    $env:CGO_ENABLED = "0"

    $LdFlags = "-s -w -X 'main.Version=$Version' -X 'main.BuildTime=$BuildTime' -X 'main.GitCommit=$GitCommit'"
    & go build -buildvcs=false -trimpath -ldflags $LdFlags -o $OutputPath $SourcePath

    if ($LASTEXITCODE -ne 0) {
        throw "Build failed for $Component $OS/$Arch"
    }

    Compress-Binary -FilePath $OutputPath -OS $OS
    $FileSize = (Get-Item $OutputPath).Length / 1MB
    Write-Info ("  -> {0} ({1:N2} MB)" -f $OutputPath, $FileSize)
}

function Build-All {
    foreach ($Platform in $DesktopPlatforms) {
        Build-Binary -OS $Platform.OS -Arch $Platform.Arch -Component "server"
        Build-Binary -OS $Platform.OS -Arch $Platform.Arch -Component "client"
    }

    Write-Info ""
    Write-Info "Build completed! Output directory: $BuildDir"
    Write-Info ""
    Write-Info "Built files:"
    Get-ChildItem -Recurse $BuildDir -File | ForEach-Object {
        $RelPath = $_.FullName.Replace($BuildDir, "").TrimStart("\")
        $Size = "{0:N2} MB" -f ($_.Length / 1MB)
        Write-Host "  $RelPath ($Size)"
    }
}

function Build-Current {
    $OS = go env GOOS
    $Arch = go env GOARCH

    Build-Binary -OS $OS -Arch $Arch -Component "server"
    Build-Binary -OS $OS -Arch $Arch -Component "client"

    Write-Info "Binaries built in $BuildDir\${OS}_${Arch}\"
}

function Build-Android {
    $OutputDir = Join-Path $BuildDir "android_arm64"
    $AndroidLibDir = Join-Path $RootDir "android\app\libs"
    if (-not (Test-Path $OutputDir)) {
        New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
    }

    Write-Info "Building client for android/arm64..."
    $env:GOOS = "android"
    $env:GOARCH = "arm64"
    $env:CGO_ENABLED = "0"

    $OutputPath = Join-Path $OutputDir "client"
    $LdFlags = "-s -w -X 'main.Version=$Version' -X 'main.BuildTime=$BuildTime' -X 'main.GitCommit=$GitCommit'"
    & go build -buildvcs=false -trimpath -ldflags $LdFlags -o $OutputPath (Join-Path $RootDir "cmd\client")
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed for client android/arm64"
    }

    if (Get-Command gomobile -ErrorAction SilentlyContinue) {
        Write-Info "Building gomobile Android binding..."
        & gomobile bind -target android/arm64 -androidapi 21 -javapkg com.gotunnel.mobilebind -o (Join-Path $OutputDir "gotunnelmobile.aar") "github.com/gotunnel/mobile/gotunnelmobile"
        if ($LASTEXITCODE -ne 0) {
            throw "gomobile bind failed"
        }
        if (-not (Test-Path $AndroidLibDir)) {
            New-Item -ItemType Directory -Path $AndroidLibDir -Force | Out-Null
        }
        Copy-Item (Join-Path $OutputDir "gotunnelmobile.aar") (Join-Path $AndroidLibDir "gotunnelmobile.aar") -Force
    } else {
        Write-Warn "gomobile not found, skipping Android AAR build"
    }

    $GradleWrapper = Join-Path $RootDir "android\gradlew.bat"
    if (Test-Path $GradleWrapper) {
        Write-Info "Building Android debug APK..."
        Push-Location (Join-Path $RootDir "android")
        try {
            & $GradleWrapper assembleDebug
            if ($LASTEXITCODE -ne 0) {
                throw "Android APK build failed"
            }
        } finally {
            Pop-Location
        }
    } else {
        Write-Warn "android\\gradlew.bat not found, skipping APK build"
    }
}

function Clean-Build {
    Write-Info "Cleaning build directory..."
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
    }
    Write-Info "Clean completed"
}

function Show-Help {
    Write-Host @"
GoTunnel Build Script for Windows

Usage: .\build.ps1 [command] [-Version <version>] [-NoUPX]

Commands:
  all       Build web UI + all desktop platforms (default)
  current   Build web UI + current platform only
  web       Build web UI only
  server    Build server for current platform
  client    Build client for current platform
  android   Build android/arm64 client and optional Android artifacts
  clean     Clean build directory
  help      Show this help message

Options:
  -Version  Set version string (default: auto-resolved from tag or latest tag + commit)
  -NoUPX    Disable UPX compression

Target platforms:
  - windows/amd64
  - windows/arm64
  - linux/amd64
  - linux/arm64
  - darwin/amd64
  - darwin/arm64

Examples:
  .\build.ps1                    # Build all desktop platforms
  .\build.ps1 all -Version 1.0.0 # Build with version
  .\build.ps1 current            # Build current platform only
  .\build.ps1 clean              # Clean build directory
"@
}

function Main {
    Push-Location $RootDir

    try {
        Write-Info "GoTunnel Build Script"
        Write-Info "Version: $Version | Commit: $GitCommit"
        Write-Info ""

        switch ($Command.ToLower()) {
            "all" {
                Build-Web
                Build-All
            }
            "current" {
                Build-Web
                Build-Current
            }
            "web" {
                Build-Web
            }
            "server" {
                $OS = go env GOOS
                $Arch = go env GOARCH
                Build-Binary -OS $OS -Arch $Arch -Component "server"
            }
            "client" {
                $OS = go env GOOS
                $Arch = go env GOARCH
                Build-Binary -OS $OS -Arch $Arch -Component "client"
            }
            "android" {
                Build-Android
            }
            "clean" {
                Clean-Build
            }
            { $_ -in @("help", "--help", "-h", "/?") } {
                Show-Help
                return
            }
            default {
                Write-Err "Unknown command: $Command"
                Show-Help
                exit 1
            }
        }

        Write-Info ""
        Write-Info "Done!"
    } finally {
        Pop-Location
    }
}

Main
