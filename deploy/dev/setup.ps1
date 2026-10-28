[CmdletBinding()]
param(
    [string]$Distro = 'Ubuntu-24.04',
    [string]$Repo = 'https://github.com/Khaymah/dhow.git',
    [string]$Path = 'src/dhow',
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$SetupArgs = @()
)

$ErrorActionPreference = 'Stop'

function Stop-Setup([string]$Message) {
    Write-Host "dev-setup: $Message" -ForegroundColor Red
    exit 1
}

if ($Path -match '(^/)|(\.\.)') {
    Stop-Setup 'Path is relative to your Linux home folder and must not contain ..'
}

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    Stop-Setup 'WSL is not available. It needs Windows 10 version 2004 or newer, or Windows 11.'
}

$installed = @(& wsl.exe --list --quiet 2>$null) |
    ForEach-Object { ($_ -replace "`0", '').Trim() } |
    Where-Object { $_ }

if ($installed -notcontains $Distro) {
    Write-Host "Installing $Distro in WSL2. When it opens, create your Linux user, then run this script again."
    Write-Host 'Windows may ask you to restart first.'
    & wsl.exe --install --distribution $Distro
    exit 0
}

& wsl.exe --distribution $Distro --cd '~' --exec test -d "$Path/.git"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Cloning $Repo into ~/$Path inside $Distro"
    & wsl.exe --distribution $Distro --cd '~' --exec git clone -- $Repo $Path
    if ($LASTEXITCODE -ne 0) {
        Stop-Setup 'git clone failed'
    }
} else {
    Write-Host "Updating ~/$Path inside $Distro"
    & wsl.exe --distribution $Distro --cd '~' --exec git -C $Path pull --ff-only
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Could not update ~/$Path (local changes or another branch); the setup runs from it as it is." -ForegroundColor Yellow
    }
}

& wsl.exe --distribution $Distro --cd '~' --exec bash "$Path/deploy/dev/setup.sh" @SetupArgs
if ($LASTEXITCODE -ne 0) {
    Stop-Setup "the setup inside $Distro failed"
}

Write-Host ''
Write-Host "make and the tools run inside $Distro, not in PowerShell. From PowerShell:"
Write-Host "  Database container:  wsl -d $Distro --cd ~ -- bash -lc 'cd $Path && make dev-db'"
Write-Host "  Start the server:    wsl -d $Distro --cd ~ -- bash -lc 'cd $Path && make dev'"
Write-Host "Or open a shell there with: wsl -d $Distro --cd ~, then cd $Path"
Write-Host 'In VS Code, the WSL extension opens it from there. The browser on Windows reaches http://localhost:8080.'
