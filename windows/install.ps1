# Discord RSS Bot - Windows Service Installer
# Run as Administrator

param(
    [Parameter(Mandatory=$true)]
    [string]$WebhookUrl,
    
    [string]$InstallPath = "C:\Program Files\DiscordRSSBot",
    [string]$ServiceName = "DiscordRSSBot",
    [string]$ServiceDisplayName = "Discord RSS Bot Service"
)

Write-Host "Installing Discord RSS Bot as Windows Service..." -ForegroundColor Green

# Check if running as administrator
if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Error "This script must be run as Administrator. Exiting..."
    exit 1
}

# Create installation directory
Write-Host "Creating installation directory: $InstallPath"
New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallPath\data" | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallPath\logs" | Out-Null

# Copy bot files
Write-Host "Copying bot files..."
Copy-Item "bot_service.py" -Destination "$InstallPath\bot_service.py"
Copy-Item "requirements.txt" -Destination "$InstallPath\requirements.txt"

# Install Python dependencies
Write-Host "Installing Python dependencies..."
& python -m pip install -r "$InstallPath\requirements.txt"

# Create config file
$configContent = @"
# Discord RSS Bot Configuration
DISCORD_WEBHOOK_URL=$WebhookUrl
CHECK_INTERVAL=300
POST_DELAY=3
COOLDOWN_DELAY=60
MAX_POST_LENGTH=1900
MAX_CONTENT_LENGTH=800
RSS_BOT_DATA=$InstallPath\data
RSS_BOT_LOGS=$InstallPath\logs
"@

$configContent | Out-File -FilePath "$InstallPath\config.env" -Encoding UTF8

# Create wrapper script for service
$wrapperContent = @"
@echo off
cd /d "$InstallPath"
set RSS_BOT_CONFIG=$InstallPath\config.env
python bot_service.py
"@

$wrapperContent | Out-File -FilePath "$InstallPath\run_bot.bat" -Encoding ASCII

# Install as Windows Service using NSSM (if available) or sc command
Write-Host "Installing Windows Service..."

# Try to use NSSM first (Non-Sucking Service Manager)
$nssmPath = Get-Command nssm -ErrorAction SilentlyContinue
if ($nssmPath) {
    Write-Host "Using NSSM to install service..."
    & nssm install $ServiceName "$InstallPath\run_bot.bat"
    & nssm set $ServiceName DisplayName "$ServiceDisplayName"
    & nssm set $ServiceName Description "Automated Discord RSS Bot that monitors Brazilian news feeds"
    & nssm set $ServiceName Start SERVICE_AUTO_START
    & nssm set $ServiceName AppDirectory "$InstallPath"
    & nssm set $ServiceName AppStdout "$InstallPath\logs\service.log"
    & nssm set $ServiceName AppStderr "$InstallPath\logs\service_error.log"
} else {
    Write-Host "NSSM not found. Using sc command (basic service installation)..."
    Write-Warning "For better service management, consider installing NSSM: https://nssm.cc/"
    
    # Create a PowerShell service wrapper
    $serviceWrapper = @"
# Service wrapper for Discord RSS Bot
Set-Location "$InstallPath"
`$env:RSS_BOT_CONFIG = "$InstallPath\config.env"
& python bot_service.py
"@
    
    $serviceWrapper | Out-File -FilePath "$InstallPath\service_wrapper.ps1" -Encoding UTF8
    
    # Register service
    & sc.exe create $ServiceName binPath= "powershell.exe -ExecutionPolicy Bypass -File `"$InstallPath\service_wrapper.ps1`"" DisplayName= "$ServiceDisplayName" start= auto
}

# Set proper permissions
Write-Host "Setting permissions..."
$acl = Get-Acl $InstallPath
$accessRule = New-Object System.Security.AccessControl.FileSystemAccessRule("NETWORK SERVICE", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow")
$acl.SetAccessRule($accessRule)
Set-Acl $InstallPath $acl

Write-Host "Installation completed!" -ForegroundColor Green
Write-Host ""
Write-Host "Service Management Commands:" -ForegroundColor Yellow
Write-Host "  Start service:   net start $ServiceName"
Write-Host "  Stop service:    net stop $ServiceName"
Write-Host "  Service status:  sc query $ServiceName"
Write-Host ""
Write-Host "Log files location: $InstallPath\logs" -ForegroundColor Cyan
Write-Host "Config file: $InstallPath\config.env" -ForegroundColor Cyan
Write-Host ""
Write-Host "To start the service now, run: net start $ServiceName" -ForegroundColor Green
