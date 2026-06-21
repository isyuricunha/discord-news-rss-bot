# Discord RSS Bot Windows background task installer.
# Run from an extracted Windows release archive in an elevated PowerShell session.

param(
    [Parameter(Mandatory=$true)]
    [string]$WebhookUrl,

    [string]$InstallPath = "C:\Program Files\DiscordRSSBot",
    [string]$TaskName = "DiscordRSSBot",
    [string]$ExecutablePath = ".\discord-rss-bot.exe"
)

$ErrorActionPreference = "Stop"

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run this installer as Administrator."
}

if (-not (Test-Path $ExecutablePath)) {
    throw "Executable not found: $ExecutablePath"
}

New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $InstallPath "data") | Out-Null

Copy-Item $ExecutablePath -Destination (Join-Path $InstallPath "discord-rss-bot.exe") -Force

$configPath = Join-Path $InstallPath "config.env"
@"
DISCORD_WEBHOOK_URL=$WebhookUrl
RSS_BOT_DATA=$InstallPath\data
INITIAL_SYNC_MODE=skip
LOG_LEVEL=info
LOG_FORMAT=text
"@ | Out-File -FilePath $configPath -Encoding utf8 -Force

$action = New-ScheduledTaskAction `
    -Execute (Join-Path $InstallPath "discord-rss-bot.exe") `
    -Argument "run" `
    -WorkingDirectory $InstallPath
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -RunLevel Highest

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null

Write-Host "Installed Discord RSS Bot scheduled task: $TaskName"
Write-Host "Configuration: $configPath"
Write-Host "Start now: Start-ScheduledTask -TaskName $TaskName"
Write-Host "Stop: Stop-ScheduledTask -TaskName $TaskName"
