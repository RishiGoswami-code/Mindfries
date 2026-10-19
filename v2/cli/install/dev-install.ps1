# Builds mindfries from this checkout and installs it on your PATH through
# install.ps1, exactly as a candidate's install would, so you can then type
# `mindfries start` in any terminal. For developers; needs Go.
#
#   powershell -ExecutionPolicy Bypass -File v2/cli/install/dev-install.ps1
#   powershell -ExecutionPolicy Bypass -File v2/cli/install/dev-install.ps1 -Api http://localhost:8787/v2/cli
#
# -Api is baked into the build (default: the fake API hosted on Render). The
# build also trusts the fake API's development signing key from that server,
# which only dev builds can do.
param([string]$Api = "https://mindfries-cli.onrender.com/v2/cli")

$ErrorActionPreference = "Stop"
$cli = Split-Path -Parent $PSScriptRoot

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $arch = "amd64" }
    "ARM64" { $arch = "arm64" }
    default { throw "unsupported processor $($env:PROCESSOR_ARCHITECTURE)" }
}
$name = "mindfries_windows_$arch.exe"
$dist = Join-Path ([IO.Path]::GetTempPath()) ("mindfries-dev-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $dist | Out-Null

try {
    $commit = (git -C $cli rev-parse --short=12 HEAD 2>$null)
    if (-not $commit) { $commit = "unknown" }
    $pkg = "github.com/mindfries/v2/cli/internal"
    $flags = "-X $pkg/buildinfo.DefaultAPI=$Api -X $pkg/buildinfo.TrustDevKey=1 -X $pkg/buildinfo.Commit=$commit"
    Write-Output "Building mindfries for $Api..."
    Push-Location $cli
    try {
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags $flags -o (Join-Path $dist $name) ./cmd/mindfries
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    } finally {
        Pop-Location
    }
    $sum = (Get-FileHash -Algorithm SHA256 (Join-Path $dist $name)).Hash.ToLower()
    Set-Content -Encoding ascii -Path (Join-Path $dist "checksums.txt") -Value "$sum  $name"

    $env:MINDFRIES_DOWNLOAD = $dist
    & (Join-Path $PSScriptRoot "install.ps1")
} finally {
    Remove-Item Env:\MINDFRIES_DOWNLOAD -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $dist -ErrorAction SilentlyContinue
}
