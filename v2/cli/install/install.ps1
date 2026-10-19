# Mindfries CLI installer for Windows PowerShell.
#
# Install (the candidate runs this once):
#   irm https://<v2-domain>/install.ps1 | iex
# then, in any terminal:
#   mindfries start
#
# It downloads the release's checksums, installs mindfries only if the
# binary's SHA-256 matches (a tampered binary is refused and deleted), puts
# it in %LOCALAPPDATA%\Mindfries\bin and adds that folder to the user's PATH
# (no admin rights needed). A binary that already matches the release isn't
# downloaded again. Given a setup code, as a dashboard command can do:
#   powershell -ExecutionPolicy Bypass -c "& ([scriptblock]::Create((irm https://<v2-domain>/install.ps1))) <SETUP-CODE>"
# it then runs `mindfries start <SETUP-CODE>`.
#
# Environment (for testing and mirrors):
#   MINDFRIES_DOWNLOAD        release base URL, or a local folder (default below)
#   MINDFRIES_INSTALL_DIR     where the binary goes (default %LOCALAPPDATA%\Mindfries\bin)
#   MINDFRIES_NO_MODIFY_PATH  1 leaves PATH alone
#   MINDFRIES_PATH_KEY        registry key holding the user PATH (default HKCU:\Environment; tests)
param([string]$Code = "")

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# Run as a file, `exit` sets the exit code. Run through `irm | iex`, it would
# close the candidate's PowerShell window, so stop with an error instead.
$asFile = [bool]$PSCommandPath
function Fail($msg) {
    if ($asFile) {
        [Console]::Error.WriteLine("mindfries installer: $msg")
        exit 1
    }
    throw "mindfries installer: $msg"
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
$bin = Join-Path $dir "mindfries.exe"

# A local folder works as the release too, for dev-install.ps1.
function Fetch($file, $out) {
    if (Test-Path -LiteralPath $base -PathType Container) {
        Copy-Item -LiteralPath (Join-Path $base $file) -Destination $out
    } else {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$file" -OutFile $out
    }
}

try {
    try {
        Fetch "checksums.txt" (Join-Path $tmp "checksums.txt")
    } catch {
        Fail "couldn't download the release checksums from $base. Check your connection and run the install command again."
    }
    $want = $null
    foreach ($line in Get-Content (Join-Path $tmp "checksums.txt")) {
        $parts = $line -split "\s+"
        if ($parts.Count -ge 2 -and ($parts[1] -eq $name -or $parts[1] -eq "*$name")) { $want = $parts[0].ToLower() }
    }
    if (-not $want) { Fail "the release has no build for windows/$arch." }

    if ((Test-Path $bin) -and ((Get-FileHash -Algorithm SHA256 $bin).Hash.ToLower() -eq $want)) {
        Write-Output "mindfries is up to date."
    } else {
        Write-Output "Downloading mindfries for windows/$arch..."
        $dl = Join-Path $tmp $name
        try {
            Fetch $name $dl
        } catch {
            Fail "couldn't download mindfries. Check your connection and run the install command again."
        }
        $got = (Get-FileHash -Algorithm SHA256 $dl).Hash.ToLower()
        if ($got -ne $want) {
            Remove-Item -Force $dl
            Fail "the download doesn't match the release checksum, so it was deleted and not run. Run the install command again; if this keeps happening, tell the hiring team."
        }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        Move-Item -Force $dl $bin
        Write-Output "Installed $bin"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# Adds $dir to the user's PATH in the registry, keeping entries like
# %USERPROFILE%\bin unexpanded, and tells Windows the environment changed so
# new terminals see it. Returns whether anything changed.
function Add-UserPath($dir) {
    $key = if ($env:MINDFRIES_PATH_KEY) { $env:MINDFRIES_PATH_KEY } else { "HKCU:\Environment" }
    if (-not (Test-Path $key)) { New-Item -Path $key -Force | Out-Null }
    $current = (Get-Item -Path $key).GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $parts = @($current -split ";" | Where-Object { $_ -ne "" })
    foreach ($p in $parts) {
        if ([Environment]::ExpandEnvironmentVariables($p).TrimEnd("\") -ieq $dir.TrimEnd("\")) { return $false }
    }
    New-ItemProperty -Path $key -Name "Path" -Value ((@($parts) + $dir) -join ";") -PropertyType ExpandString -Force | Out-Null
    if (-not $env:MINDFRIES_PATH_KEY) {
        # Setting any user variable through .NET broadcasts WM_SETTINGCHANGE,
        # which is what makes Explorer (and so new terminals) re-read PATH.
        [Environment]::SetEnvironmentVariable("MINDFRIES_PATH_REFRESH", "1", "User")
        [Environment]::SetEnvironmentVariable("MINDFRIES_PATH_REFRESH", $null, "User")
    }
    return $true
}

$onPath = ($env:PATH -split ";" | Where-Object { $_.TrimEnd("\") -ieq $dir.TrimEnd("\") }).Count -gt 0
if ($env:MINDFRIES_NO_MODIFY_PATH -eq "1") {
    if (-not $onPath) { Write-Output "Add $dir to your PATH to run 'mindfries' from any terminal." }
} else {
    if (Add-UserPath $dir) {
        Write-Output "Added $dir to your PATH."
    }
    if (-not $onPath) {
        # This window too, when the installer runs in it (irm | iex).
        $env:PATH = "$env:PATH;$dir"
        Write-Output "Terminals that were already open won't find 'mindfries' yet: open a new terminal window."
    }
}
$found = Get-Command mindfries -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($found -and ($found.Source -ine $bin)) {
    Write-Output "Note: another mindfries comes first on your PATH ($($found.Source)). Remove it, or run $bin."
}

if ($Code) {
    & $bin start $Code
    if ($asFile) { exit $LASTEXITCODE }
    return
}
Write-Output ""
Write-Output "Next, in any terminal:  mindfries start"
