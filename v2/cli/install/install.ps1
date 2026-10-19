# Mindfries CLI installer for Windows PowerShell.
#
# The dashboard's setup command runs it as:
#   powershell -ExecutionPolicy Bypass -c "& ([scriptblock]::Create((irm https://<v2-domain>/install.ps1))) <SETUP-CODE>"
#
# It downloads the release's checksums, installs mindfries only if the
# binary's SHA-256 matches (a tampered binary is refused and deleted), then
# runs `mindfries start <SETUP-CODE>`. A binary that already matches the
# release isn't downloaded again.
#
# Environment (for testing and mirrors):
#   MINDFRIES_DOWNLOAD     release base URL (default below)
#   MINDFRIES_INSTALL_DIR  where the binary goes (default %LOCALAPPDATA%\Mindfries\bin)
param([string]$Code = "")

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

function Fail($msg) {
    [Console]::Error.WriteLine("mindfries installer: $msg")
    exit 1
}

$base = if ($env:MINDFRIES_DOWNLOAD) { $env:MINDFRIES_DOWNLOAD } else { "https://get.v2.mindfries.example/cli/latest" }
$dir = if ($env:MINDFRIES_INSTALL_DIR) { $env:MINDFRIES_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Mindfries\bin" }

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $arch = "amd64" }
    "ARM64" { $arch = "arm64" }
    default { Fail "this processor ($($env:PROCESSOR_ARCHITECTURE)) isn't supported. Contact the hiring team before your assessment." }
}
$name = "mindfries_windows_$arch.exe"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("mindfries-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")
    } catch {
        Fail "couldn't download the release checksums from $base. Check your connection and run the setup command again."
    }
    $want = $null
    foreach ($line in Get-Content (Join-Path $tmp "checksums.txt")) {
        $parts = $line -split "\s+"
        if ($parts.Count -ge 2 -and ($parts[1] -eq $name -or $parts[1] -eq "*$name")) { $want = $parts[0].ToLower() }
    }
    if (-not $want) { Fail "the release has no build for windows/$arch." }

    $bin = Join-Path $dir "mindfries.exe"
    if ((Test-Path $bin) -and ((Get-FileHash -Algorithm SHA256 $bin).Hash.ToLower() -eq $want)) {
        Write-Output "mindfries is up to date."
    } else {
        Write-Output "Downloading mindfries for windows/$arch..."
        $dl = Join-Path $tmp $name
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile $dl
        } catch {
            Fail "couldn't download mindfries. Check your connection and run the setup command again."
        }
        $got = (Get-FileHash -Algorithm SHA256 $dl).Hash.ToLower()
        if ($got -ne $want) {
            Remove-Item -Force $dl
            Fail "the download doesn't match the release checksum, so it was deleted and not run. Run the setup command again; if this keeps happening, tell the hiring team."
        }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        Move-Item -Force $dl $bin
        Write-Output "Installed $bin"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

if (-not (($env:PATH -split ";") -contains $dir)) {
    Write-Output "Tip: add $dir to your PATH to run 'mindfries' from any terminal."
}
if ($Code) {
    & $bin start $Code
    exit $LASTEXITCODE
}
