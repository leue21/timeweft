# timeweft installer for Windows.
#
#   irm https://raw.githubusercontent.com/leue21/timeweft/main/install.ps1 | iex
#
# Downloads the release zip for this CPU, verifies its SHA-256 against the
# release's checksums.txt, installs timeweft.exe and adds the directory to the
# user PATH. Default directory: %LOCALAPPDATA%\Programs\timeweft
#
# Environment: TIMEWEFT_VERSION, TIMEWEFT_INSTALL_DIR, and
# TIMEWEFT_DOWNLOAD_URL (base URL holding the archives, e.g. a mirror).

function Install-Timeweft {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    $repo = 'leue21/timeweft'

    $version = $env:TIMEWEFT_VERSION
    $binDir = if ($env:TIMEWEFT_INSTALL_DIR) { $env:TIMEWEFT_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\timeweft' }
    $baseUrl = $env:TIMEWEFT_DOWNLOAD_URL

    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        'X64'   { 'amd64' }
        'Arm64' { 'arm64' }
        default { throw "timeweft-install: unsupported CPU architecture: $_" }
    }

    if (-not $version) {
        $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
        if (-not $version) { throw 'timeweft-install: no published release found' }
    }
    $num = $version.TrimStart('v')
    if (-not $baseUrl) { $baseUrl = "https://github.com/$repo/releases/download/v$num" }
    $archive = "timeweft_${num}_windows_${arch}.zip"

    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("timeweft-" + [System.Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "timeweft-install: downloading timeweft $version (windows/$arch)"
        Invoke-WebRequest "$baseUrl/$archive" -OutFile (Join-Path $tmp $archive) -UseBasicParsing
        Invoke-WebRequest "$baseUrl/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

        $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { ($_ -split '\s+')[1] -eq $archive }
        if (-not $line) { throw "timeweft-install: $archive is not listed in checksums.txt" }
        $expected = ($line -split '\s+')[0].ToLower()
        $actual = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) { throw "timeweft-install: checksum mismatch for $archive (expected $expected, got $actual)" }

        Expand-Archive (Join-Path $tmp $archive) -DestinationPath (Join-Path $tmp 'x')
        New-Item -ItemType Directory -Force -Path $binDir | Out-Null
        Copy-Item (Join-Path $tmp 'x\timeweft.exe') (Join-Path $binDir 'timeweft.exe') -Force
    }
    finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    $exe = Join-Path $binDir 'timeweft.exe'
    Write-Host "timeweft-install: installed $(& $exe version) to $exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not (($userPath -split ';') -contains $binDir)) {
        $newPath = (@($userPath, $binDir) | Where-Object { $_ }) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "timeweft-install: added $binDir to your user PATH (open a new terminal)"
    }
    Write-Host 'timeweft-install: next: timeweft setup claude'
}

Install-Timeweft
