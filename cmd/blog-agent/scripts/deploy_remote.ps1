param(
    [string]$Server,
    [string]$User = "root",
    [string]$RemoteDir = "/opt/blog-agent",
    [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

function Read-Required([string]$Prompt, [string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) { $Value = Read-Host $Prompt }
    if ([string]::IsNullOrWhiteSpace($Value)) { throw "$Prompt is required." }
    return $Value.Trim()
}

function Assert-SafeValue([string]$Value, [string]$Pattern, [string]$Name) {
    if ($Value -notmatch $Pattern) { throw "$Name contains unsupported characters." }
}

$Server = Read-Required "Remote IP or hostname" $Server
$User = Read-Required "SSH username" $User
$RemoteDir = Read-Required "Remote deployment directory" $RemoteDir
Assert-SafeValue $Server '^[A-Za-z0-9.-]+$' "Remote IP or hostname"
Assert-SafeValue $User '^[A-Za-z0-9_-]+$' "SSH username"
Assert-SafeValue $RemoteDir '^/[A-Za-z0-9._/-]+$' "Remote deployment directory"
if ($Port -lt 1 -or $Port -gt 65535) { throw "Port must be between 1 and 65535." }

$scriptDir = Split-Path -Parent $PSCommandPath
$appDir = Split-Path -Parent $scriptDir
$stagingDir = Join-Path $env:TEMP ("blog-agent-deploy-" + [guid]::NewGuid().ToString("N"))
$archivePath = Join-Path $env:TEMP ("blog-agent-" + [guid]::NewGuid().ToString("N") + ".tar.gz")
$target = "$User@$Server"

try {
    New-Item -ItemType Directory -Path $stagingDir | Out-Null

    Write-Host "[1/4] Building Linux binary..." -ForegroundColor Cyan
    Push-Location $appDir
    try {
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags="-s -w" -o (Join-Path $stagingDir "blog-agent") .
        if ($LASTEXITCODE -ne 0) { throw "Linux build failed." }
    }
    finally {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
        Pop-Location
    }

    Copy-Item -LiteralPath (Join-Path $appDir "templates") -Destination (Join-Path $stagingDir "templates") -Recurse
    Copy-Item -LiteralPath (Join-Path $appDir "statics") -Destination (Join-Path $stagingDir "statics") -Recurse
    Copy-Item -LiteralPath (Join-Path $appDir "sys_conf.md") -Destination (Join-Path $stagingDir "sys_conf.md.dist")
    New-Item -ItemType Directory -Path (Join-Path $stagingDir "scripts") | Out-Null
    Copy-Item -LiteralPath (Join-Path $scriptDir "start.sh") -Destination (Join-Path $stagingDir "scripts/start.sh")
    Copy-Item -LiteralPath (Join-Path $scriptDir "stop.sh") -Destination (Join-Path $stagingDir "scripts/stop.sh")
    Copy-Item -LiteralPath (Join-Path $scriptDir "show.sh") -Destination (Join-Path $stagingDir "scripts/show.sh")
    Copy-Item -LiteralPath (Join-Path $scriptDir "restart.sh") -Destination (Join-Path $stagingDir "scripts/restart.sh")

    Write-Host "[2/4] Creating release archive..." -ForegroundColor Cyan
    tar.exe -C $stagingDir -czf $archivePath blog-agent templates statics scripts sys_conf.md.dist
    if ($LASTEXITCODE -ne 0) { throw "Release archive creation failed." }

    Write-Host "[3/4] Uploading to $target (OpenSSH will request the password)..." -ForegroundColor Cyan
    scp.exe $archivePath "${target}:/tmp/blog-agent-release.tar.gz"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "SFTP upload failed; retrying with legacy SCP protocol..." -ForegroundColor Yellow
        scp.exe -O $archivePath "${target}:/tmp/blog-agent-release.tar.gz"
    }
    if ($LASTEXITCODE -ne 0) { throw "Release archive upload failed." }

    $remoteCommand = @(
        "set -eu"
        "APP_DIR='$RemoteDir'"
        "PORT='$Port'"
        "ARCHIVE='/tmp/blog-agent-release.tar.gz'"
        'mkdir -p "$APP_DIR"/logs "$APP_DIR"/data'
        'if command -v fuser > /dev/null 2>&1; then'
        '  fuser -k "$PORT/tcp" || true'
        'elif command -v lsof > /dev/null 2>&1; then'
        '  PORT_PIDS=$(lsof -ti "tcp:$PORT" || true)'
        '  if [ -n "$PORT_PIDS" ]; then kill $PORT_PIDS; fi'
        'else'
        '  echo "Cannot check port $PORT: install psmisc (fuser) or lsof." >&2'
        '  exit 1'
        'fi'
        'pgrep -f "^$APP_DIR/blog-agent" | xargs -r kill || true'
        'sleep 1'
        'tar -xzf "$ARCHIVE" -C "$APP_DIR"'
        'rm -f "$ARCHIVE"'
        'if [ ! -f "$APP_DIR/sys_conf.md" ]; then'
        '  mv "$APP_DIR/sys_conf.md.dist" "$APP_DIR/sys_conf.md"'
        'else'
        '  rm -f "$APP_DIR/sys_conf.md.dist"'
        'fi'
        'chmod +x "$APP_DIR/blog-agent" "$APP_DIR"/scripts/*.sh'
        'if [ ! -f "$APP_DIR/data/go_blog.db" ]; then'
        '  if [ ! -d "$APP_DIR/blogs_txt" ]; then'
        '    echo "SQLite is not initialized and remote blogs_txt is missing." >&2'
        '    exit 1'
        '  fi'
        '  echo "First deployment detected: migrating remote Markdown data into SQLite..."'
        '  "$APP_DIR/blog-agent" migrate-sqlite "$APP_DIR/sys_conf.md"'
        'fi'
        'nohup "$APP_DIR/blog-agent" "$APP_DIR/sys_conf.md" -port "$PORT" > "$APP_DIR/logs/server.stdout.log" 2>&1 < /dev/null &'
        'sleep 2'
        'pgrep -f "^$APP_DIR/blog-agent" > /dev/null'
        'echo "Deployment completed: $APP_DIR"'
    ) -join "`n"

    $remotePayload = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($remoteCommand))
    Write-Host "[4/4] Installing and restarting the remote service (OpenSSH will request the password again)..." -ForegroundColor Cyan
    ssh.exe $target "echo '$remotePayload' | base64 -d | bash"
    if ($LASTEXITCODE -ne 0) { throw "Remote deployment or startup failed." }

    Write-Host "Deployment successful: ${target}:$RemoteDir" -ForegroundColor Green
}
finally {
    Remove-Item -LiteralPath $stagingDir -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $archivePath -Force -ErrorAction SilentlyContinue
}
