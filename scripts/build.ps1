# GoTunnel Build Script for Windows
# Usage: .\build.ps1 [command]
# Commands: all, current, web, server, client, clean, help

param(
    [Parameter(Position=0)]
    [string]$Command = "all",

    [string]$Version = "dev",

    [switch]$NoUPX
)

$ErrorActionPreference = "Stop"

# 项目根目录
$RootDir = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $RootDir) {
    $RootDir = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
}
$BuildDir = Join-Path $RootDir "build"

# 版本信息
$BuildTime = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
try {
    $GitCommit = (git -C $RootDir rev-parse --short HEAD 2>$null)
    if (-not $GitCommit) { $GitCommit = "unknown" }
} catch {
    $GitCommit = "unknown"
}

# 目标平台
$Platforms = @(
    @{OS="windows"; Arch="amd64"},
    @{OS="linux"; Arch="amd64"},
    @{OS="linux"; Arch="arm64"},
    @{OS="darwin"; Arch="amd64"},
    @{OS="darwin"; Arch="arm64"}
)
)

# 颜色输出函数
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

# 检查 UPX 是否可用
function Test-UPX {
    try {
        $null = Get-Command upx -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

# UPX 压缩二进制
function Compress-Binary {
    param([string]$FilePath, [string]$OS)

    if ($NoUPX) { return }
    if (-not (Test-UPX)) {
        Write-Warn "UPX not found, skipping compression"
        return
    }
    # macOS 二进制不支持 UPX
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

# 构建 Web UI
function Build-Web {
    Write-Info "Building web UI..."

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

    # 复制到 embed 目录
    Write-Info "Copying dist to embed directory..."
    $DistSource = Join-Path $WebDir "dist"
    $DistDest = Join-Path $RootDir "internal\server\app\dist"

    if (Test-Path $DistDest) {
        Remove-Item -Recurse -Force $DistDest
    }
    Copy-Item -Recurse $DistSource $DistDest

    Write-Info "Web UI built successfully"
}

# 构建单个二进制
function Build-Binary {
    param(
        [string]$OS,
        [string]$Arch,
        [string]$Component  # server 或 client
    )

    $OutputName = $Component
    if ($OS -eq "windows") {
        $OutputName = "$Component.exe"
    }

    $OutputDir = Join-Path $BuildDir "${OS}_${Arch}"
    if (-not (Test-Path $OutputDir)) {
        New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
    }

    Write-Info "Building $Component for $OS/$Arch..."

    $env:GOOS = $OS
    $env:GOARCH = $Arch
    $env:CGO_ENABLED = "0"

    $LDFlags = "-s -w -X 'github.com/gotunnel/pkg/version.Version=$Version' -X 'github.com/gotunnel/pkg/version.BuildTime=$BuildTime' -X 'github.com/gotunnel/pkg/version.GitCommit=$GitCommit'"
    $OutputPath = Join-Path $OutputDir $OutputName
    $SourcePath = Join-Path $RootDir "cmd\$Component"

    & go build -ldflags $LDFlags -o $OutputPath $SourcePath

    if ($LASTEXITCODE -ne 0) {
        throw "Build failed for $Component $OS/$Arch"
    }

    # UPX 压缩
    Compress-Binary -FilePath $OutputPath -OS $OS

    # 显示文件大小
    $FileSize = (Get-Item $OutputPath).Length / 1MB
    Write-Info "  -> $OutputPath ({0:N2} MB)" -f $FileSize
}

# 构建所有平台
function Build-All {
    foreach ($Platform in $Platforms) {
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

# 仅构建当前平台
function Build-Current {
    $OS = go env GOOS
    $Arch = go env GOARCH

    Build-Binary -OS $OS -Arch $Arch -Component "server"
    Build-Binary -OS $OS -Arch $Arch -Component "client"

    Write-Info "Binaries built in $BuildDir\${OS}_${Arch}\"
}

# 清理构建产物
function Clean-Build {
    Write-Info "Cleaning build directory..."
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
    }
    Write-Info "Clean completed"
}

# 显示帮助
function Show-Help {
    Write-Host @"
GoTunnel Build Script for Windows

Usage: .\build.ps1 [command] [-Version <version>] [-NoUPX]

Commands:
  all       Build web UI + all platforms (default)
  current   Build web UI + current platform only
  web       Build web UI only
  server    Build server for current platform
  client    Build client for current platform
  clean     Clean build directory
  help      Show this help message

Options:
  -Version  Set version string (default: dev)
  -NoUPX    Disable UPX compression

Target platforms:
  - windows/amd64
  - linux/amd64
  - linux/arm64
  - darwin/amd64 (macOS Intel)
  - darwin/arm64 (macOS Apple Silicon)

Examples:
  .\build.ps1                    # Build all platforms
  .\build.ps1 all -Version 1.0.0 # Build with version
  .\build.ps1 current            # Build current platform only
  .\build.ps1 clean              # Clean build directory
"@
}

# 主函数
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
            "clean" {
                Clean-Build
            }
            { $_ -in "help", "--help", "-h", "/?" } {
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
