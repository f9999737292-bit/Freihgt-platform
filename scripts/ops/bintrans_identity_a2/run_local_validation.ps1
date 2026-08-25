# Isolated local validation for BINTRANS identity Wave A2.1 (Windows PowerShell).
# Uses disposable Compose project bintrans_a21_validate only.
# STAGING_USE=FORBIDDEN  PRODUCTION_USE=FORBIDDEN
# Does NOT touch freight_postgres / freight_postgres_data.
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
Set-Location $Root

$ComposeProject = "bintrans_a21_validate"
$ComposeBase = "infrastructure/docker-compose/docker-compose.yml"
$ComposeOverlay = "infrastructure/docker-compose/docker-compose.a21-validate.yml"
function Invoke-Compose {
    param(
        [string[]]$Command,
        [switch]$AllowFailure
    )
    if ($null -eq $Command -or $Command.Count -eq 0) {
        Fail "Invoke-Compose requires Command arguments"
    }
    $dockerArgs = @(
        "compose", "-p", $ComposeProject,
        "-f", $ComposeBase,
        "-f", $ComposeOverlay
    ) + $Command
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & docker @dockerArgs 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    $text = ($out | Out-String)
    if (-not $AllowFailure -and $code -ne 0) {
        Fail ("docker compose failed ($code): " + $text)
    }
    return $text
}

function Invoke-ComposeDown {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    $dockerArgs = @(
        "compose", "-p", $ComposeProject,
        "-f", $ComposeBase,
        "-f", $ComposeOverlay,
        "down", "-v", "--remove-orphans"
    )
    & docker @dockerArgs 2>$null | Out-Null
    Start-Sleep -Seconds 5
    $remaining = docker volume ls -q --filter "name=bintrans_a21_validate" 2>$null
    if ($remaining) {
        foreach ($vol in $remaining) {
            docker volume rm -f $vol 2>$null | Out-Null
        }
        Start-Sleep -Seconds 2
    }
    $ErrorActionPreference = $prev
}
$TenantId = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$AdminUserId = "8541a3a3-bde7-4fed-9501-37b9953bf904"
$OpsDir = Join-Path $Root "scripts/ops/bintrans_identity_a2"
$GatewayUrl = "http://localhost:18080"
$IdentityUrl = "http://localhost:18081"
$CompanyUrl = "http://localhost:18082"
$Bash = "C:\Program Files\Git\bin\bash.exe"
$PgContainer = "bintrans_a21_validate_postgres"
$MigrateDb = "postgres://freight:freight_password@postgres:5432/freight_platform?sslmode=disable"
$MigrateForceUsed = $false
$MigrateForceScope = "NONE"
$MockGatewayJob = $null
$LitAdminLegacy = 'admin@7rights.local'
$LitAdminCanonical = 'admin@bintrans.local'
$LitShipperCanonical = 'shipper@bintrans.local'
$LitCarrierCanonical = 'carrier@bintrans.local'
$LitForwarderCanonical = 'forwarder@bintrans.local'
$LitConsigneeCanonical = 'consignee@bintrans.local'
$LitOldDomainLike = '%@7rights.local'

function Step([string]$Message) { Write-Host "==> $Message" -ForegroundColor Cyan }
function Pass([string]$Message) { Write-Host "OK: $Message" -ForegroundColor Green }
function Fail([string]$Message) { Write-Host "FAIL: $Message" -ForegroundColor Red; throw $Message }

function Wait-PostgresReady {
    Step "Wait for PostgreSQL health"
    for ($i = 1; $i -le 45; $i++) {
        $prev = $ErrorActionPreference
        $ErrorActionPreference = "SilentlyContinue"
        docker exec $PgContainer pg_isready -U freight -d freight_platform 2>$null | Out-Null
        $ready = ($LASTEXITCODE -eq 0)
        $ErrorActionPreference = $prev
        if ($ready) {
            Pass "PostgreSQL ready"
            return
        }
        Start-Sleep -Seconds 2
    }
    Fail "postgres not ready"
}

function Get-CoreTableCount {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    $count = (docker exec $PgContainer psql -U freight -d freight_platform -t -A -c `
        "SELECT count(*) FROM information_schema.tables WHERE table_schema='core';" 2>$null).Trim()
    $ErrorActionPreference = $prev
    if ($count -match '^\d+$') { return [int]$count }
    return 0
}

function Invoke-MigrateUp {
    param([switch]$VolumeRecreateAttempted)

    Step "Apply schema migrations (isolated disposable DB)"
    $out = Invoke-Compose -AllowFailure -Command @("--profile", "tools", "run", "--rm", "migrate",
        "-path=/migrations", "-database", $MigrateDb, "up")
    if ($out -match 'Dirty database version (\d+)') {
        $dirtyVersion = [int]$Matches[1]
        $coreTables = Get-CoreTableCount
        if ($coreTables -gt 0) {
            Fail "migrate dirty at version $dirtyVersion with existing core schema ($coreTables tables); refusing automatic force"
        }
        Step "Recover interrupted disposable validation migrate (dirty version $dirtyVersion, empty schema)"
        $forceOut = Invoke-Compose -AllowFailure -Command @("--profile", "tools", "run", "--rm", "migrate",
            "-path=/migrations", "-database", $MigrateDb, "force", "$dirtyVersion")
        if ($forceOut -match 'error:' -and $forceOut -notmatch 'no change') {
            Fail "migrate force failed during disposable recovery: $forceOut"
        }
        $script:MigrateForceUsed = $true
        $script:MigrateForceScope = "DISPOSABLE_VALIDATION_ONLY"
        $out = Invoke-Compose -AllowFailure -Command @("--profile", "tools", "run", "--rm", "migrate",
            "-path=/migrations", "-database", $MigrateDb, "up")
    }
    if ((Get-CoreTableCount) -eq 0) {
        if ($VolumeRecreateAttempted) {
            Fail "core schema missing after disposable volume recreate and migrate up"
        }
        Step "Disposable DB missing core schema; recreating isolated volume once"
        Invoke-ComposeDown
        Invoke-Compose -Command @("up", "-d", "postgres") | Out-Null
        Start-Sleep -Seconds 8
        Wait-PostgresReady
        Invoke-MigrateUp -VolumeRecreateAttempted
        return
    }
    if ($out -notmatch '\d+/u' -and $out -notmatch 'no change') {
        Fail "unexpected migrate output: $out"
    }
    Pass "schema migrations applied"
}

function Invoke-PsqlScalar([string]$Sql) {
    return (docker exec $PgContainer psql -U freight -d freight_platform -t -A -c $Sql).Trim()
}

function Sql-WithTenant([string]$Body) {
    return $Body.Replace('{tenant}', $TenantId)
}

function Invoke-PsqlFile {
    param(
        [string]$Path,
        [switch]$AllowFailure
    )
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = Get-Content $Path -Raw -Encoding UTF8 | docker exec -i $PgContainer psql -U freight -d freight_platform -v ON_ERROR_STOP=1 -f - 2>&1
    $ErrorActionPreference = $prev
    $text = ($out | Out-String)
    if (-not $AllowFailure -and ($text -match "ERROR:")) {
        Fail $text
    }
    return $text
}

function Assert-IsolationConfig {
    Step "Verify isolated validation configuration"
    $overlay = Get-Content (Join-Path $Root $ComposeOverlay) -Raw
    foreach ($pair in @(
            @("55432:5432", "postgres port"),
            @("18081:8081", "identity port"),
            @("18082:8082", "company port"),
            @("18080:8080", "gateway port"),
            @("bintrans_a21_validate_data", "disposable volume")
        )) {
        if ($overlay -notmatch [regex]::Escape($pair[0])) {
            Fail "isolation config missing $($pair[1]): $($pair[0])"
        }
    }
    if ($overlay -match '(?m)^\s*-\s*freight_postgres_data:|freight_postgres_data:\s*$') {
        Fail "overlay mounts freight_postgres_data"
    }
    Pass "isolation configuration verified"
}

function Start-MockGateway {
    Step "Start mock gateway for seed health/login checks (localhost:18080)"
    $script:MockGatewayJob = Start-Job -ScriptBlock {
        Add-Type -AssemblyName System.Net.Http
        $listener = New-Object System.Net.HttpListener
        $listener.Prefixes.Add("http://localhost:18080/")
        $listener.Start()
        $client = New-Object System.Net.Http.HttpClient
        try {
            while ($listener.IsListening) {
                $ctx = $listener.GetContext()
                try {
                    if ($ctx.Request.HttpMethod -eq "GET" -and $ctx.Request.Url.AbsolutePath -eq "/health") {
                        $bytes = [Text.Encoding]::UTF8.GetBytes('{"status":"ok"}')
                        $ctx.Response.StatusCode = 200
                        $ctx.Response.ContentType = "application/json"
                        $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                    }
                    elseif ($ctx.Request.HttpMethod -eq "POST" -and $ctx.Request.Url.AbsolutePath -eq "/api/v1/auth/login") {
                        $reader = New-Object IO.StreamReader($ctx.Request.InputStream)
                        $body = $reader.ReadToEnd()
                        $content = New-Object System.Net.Http.StringContent($body, [Text.Encoding]::UTF8, "application/json")
                        $resp = $client.PostAsync("http://localhost:18081/v1/auth/login", $content).GetAwaiter().GetResult()
                        $respBody = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
                        $ctx.Response.StatusCode = [int]$resp.StatusCode
                        $ctx.Response.ContentType = "application/json"
                        $outBytes = [Text.Encoding]::UTF8.GetBytes($respBody)
                        $ctx.Response.OutputStream.Write($outBytes, 0, $outBytes.Length)
                    }
                    else {
                        $ctx.Response.StatusCode = 404
                    }
                }
                finally {
                    $ctx.Response.Close()
                }
            }
        }
        finally {
            $listener.Stop()
        }
    }
    Start-Sleep -Seconds 2
    try {
        $health = Invoke-WebRequest -Uri "$GatewayUrl/health" -UseBasicParsing -TimeoutSec 5
        if ($health.StatusCode -ne 200) { Fail "mock gateway health check failed" }
    }
    catch {
        Fail "mock gateway unavailable on $GatewayUrl"
    }
    Pass "mock gateway ready"
}

function Stop-MockGateway {
    if ($null -ne $MockGatewayJob) {
        Stop-Job $MockGatewayJob -ErrorAction SilentlyContinue
        Remove-Job $MockGatewayJob -Force -ErrorAction SilentlyContinue
        $script:MockGatewayJob = $null
    }
}

function Wait-IdentityCompanyHealthy {
    Step "Wait for identity/company services"
    for ($i = 1; $i -le 60; $i++) {
        try {
            $h1 = Invoke-WebRequest -Uri "$IdentityUrl/health" -UseBasicParsing -TimeoutSec 2
            $h2 = Invoke-WebRequest -Uri "$CompanyUrl/health" -UseBasicParsing -TimeoutSec 2
            if ($h1.StatusCode -eq 200 -and $h2.StatusCode -eq 200) {
                Pass "identity/company healthy"
                return
            }
        }
        catch { }
        Start-Sleep -Seconds 2
    }
    Fail "identity/company services not healthy"
}

function Reset-Stack {
    Step "Reset isolated validation stack"
    Invoke-ComposeDown
    Invoke-Compose -Command @("up", "-d", "postgres") | Out-Null
    Start-Sleep -Seconds 8
    Wait-PostgresReady
    Invoke-MigrateUp
}

function Start-SeedStack {
    Reset-Stack
    Start-MockGateway
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        Wait-PostgresReady
        $serviceOut = Invoke-Compose -AllowFailure -Command @("up", "-d", "identity-service", "company-service")
        if ($serviceOut -notmatch 'dependency failed' -and $serviceOut -notmatch 'exited \(137\)') {
            break
        }
        if ($attempt -eq 3) {
            Fail "identity/company services failed to start after postgres retries"
        }
        Step "Retry seed stack service startup (attempt $($attempt + 1))"
        Invoke-Compose -Command @("up", "-d", "postgres") | Out-Null
        Start-Sleep -Seconds 10
    }
    Wait-IdentityCompanyHealthy
}

function Invoke-BashSeed {
    param(
        [string]$ScriptName,
        [switch]$AllowFailure
    )
    if (-not (Test-Path $Bash)) {
        Fail "Git Bash required for seed scripts: $Bash"
    }
    $wt = ($Root -replace '\\', '/')
    if ($wt -match '^[A-Za-z]:') { $wt = "/" + $wt.Substring(0,1).ToLower() + $wt.Substring(2) }
    $cmd = @(
        "cd '$wt'",
        "export POSTGRES_CONTAINER='$PgContainer'",
        "export IDENTITY_SERVICE_URL='$IdentityUrl'",
        "export COMPANY_SERVICE_URL='$CompanyUrl'",
        "export API_GATEWAY_URL='$GatewayUrl'",
        "bash scripts/dev/$ScriptName"
    ) -join " && "
    & $Bash -lc $cmd
    if (-not $AllowFailure -and $LASTEXITCODE -ne 0) {
        Fail "seed script failed: $ScriptName (exit $LASTEXITCODE)"
    }
}

function Assert-UserCount([string]$Email, [string]$Expected) {
    $sql = Sql-WithTenant "SELECT count(*) FROM core.users WHERE tenant_id='{tenant}'::uuid AND deleted_at IS NULL AND lower(email)=lower('$Email');"
    $actual = Invoke-PsqlScalar $sql
    if ($actual -ne $Expected) {
        Fail "expected $Expected user(s) for $Email, got $actual"
    }
}

function Test-IdentityLogin([string]$Email, [string]$Password) {
    $body = @{ tenant_id = $TenantId; email = $Email; password = $Password } | ConvertTo-Json
    try {
        $resp = Invoke-WebRequest -Uri "$IdentityUrl/v1/auth/login" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing
        return ($resp.StatusCode -eq 200)
    }
    catch {
        return $false
    }
}

function Stop-Stack {
    Step "Cleanup isolated validation stack"
    Stop-MockGateway
    Invoke-ComposeDown
}

try {
    Step "Check Docker daemon"
    docker info | Out-Null
    Pass "Docker daemon available"

    Assert-IsolationConfig

    Step "Phase 1 - migration toolkit tests (disposable DB)"
    Reset-Stack

    Invoke-PsqlFile (Join-Path $OpsDir "fixtures/legacy_dev_identity.sql") | Out-Null
    Invoke-PsqlFile (Join-Path $OpsDir "preflight.sql") | Out-Null

    $adminBefore = Invoke-PsqlScalar (Sql-WithTenant "SELECT id FROM core.users WHERE tenant_id='{tenant}'::uuid AND lower(email)='$LitAdminLegacy';")
    $tenantBefore = Invoke-PsqlScalar (Sql-WithTenant "SELECT id FROM core.tenants WHERE id='{tenant}'::uuid;")
    $companyBefore = Invoke-PsqlScalar (Sql-WithTenant "SELECT id FROM core.companies WHERE tenant_id='{tenant}'::uuid LIMIT 1;")
    $memBefore = Invoke-PsqlScalar "SELECT count(*) FROM core.company_memberships WHERE user_id='$AdminUserId'::uuid;"
    $rbacBefore = Invoke-PsqlScalar "SELECT count(*) FROM core.user_roles WHERE user_id='$AdminUserId'::uuid;"

    Invoke-PsqlFile (Join-Path $OpsDir "migrate.sql") | Out-Null

    $adminAfter = Invoke-PsqlScalar (Sql-WithTenant "SELECT id FROM core.users WHERE tenant_id='{tenant}'::uuid AND lower(email)='$LitAdminCanonical';")
    $memAfter = Invoke-PsqlScalar "SELECT count(*) FROM core.company_memberships WHERE user_id='$AdminUserId'::uuid;"
    $rbacAfter = Invoke-PsqlScalar "SELECT count(*) FROM core.user_roles WHERE user_id='$AdminUserId'::uuid;"

    if ($adminBefore -ne $adminAfter) { Fail "admin UUID changed during migration" }
    if ($memBefore -ne $memAfter) { Fail "membership count changed" }
    if ($rbacBefore -ne $rbacAfter) { Fail "RBAC count changed" }

    foreach ($email in @($LitAdminCanonical, $LitShipperCanonical, $LitCarrierCanonical, $LitForwarderCanonical, $LitConsigneeCanonical)) {
        Assert-UserCount $email "1"
    }
    $old7 = Invoke-PsqlScalar (Sql-WithTenant "SELECT count(*) FROM core.users WHERE tenant_id='{tenant}'::uuid AND deleted_at IS NULL AND lower(email) LIKE '$LitOldDomainLike';")
    if ($old7 -ne '0') { Fail ('expected 0 legacy-domain users after migration, got ' + $old7) }

    Invoke-PsqlFile (Join-Path $OpsDir "rollback.sql") | Out-Null
    $rollbackEmail = Invoke-PsqlScalar "SELECT email FROM core.users WHERE id='$AdminUserId'::uuid;"
    $rollbackCode = Invoke-PsqlScalar (Sql-WithTenant "SELECT code FROM core.tenants WHERE id='{tenant}'::uuid;")
    if ($rollbackEmail -ne $LitAdminLegacy -or $rollbackCode -ne "dev-7rights") {
        Fail "rollback did not restore legacy identity"
    }
    Pass "legacy migration + rollback"

    Reset-Stack
    Invoke-PsqlFile (Join-Path $OpsDir "fixtures/legacy_dev_identity.sql") | Out-Null
    Invoke-PsqlFile (Join-Path $OpsDir "fixtures/collision_extra_user.sql") | Out-Null

    $legacyBefore = Invoke-PsqlScalar "SELECT email FROM core.users WHERE id='$AdminUserId'::uuid;"
    $collisionBefore = Invoke-PsqlScalar "SELECT email FROM core.users WHERE id='22222222-2222-4222-8222-222222222222'::uuid;"
    $migrateColl = Invoke-PsqlFile (Join-Path $OpsDir "migrate.sql") -AllowFailure
    if ($migrateColl -notmatch "DUPLICATE_TARGET_EMAIL_POLICY=FAIL_CLOSED") {
        Fail "collision migration should abort with FAIL_CLOSED"
    }
    $legacyAfter = Invoke-PsqlScalar "SELECT email FROM core.users WHERE id='$AdminUserId'::uuid;"
    $collisionAfter = Invoke-PsqlScalar "SELECT email FROM core.users WHERE id='22222222-2222-4222-8222-222222222222'::uuid;"
    $legacy7Count = Invoke-PsqlScalar (Sql-WithTenant "SELECT count(*) FROM core.users WHERE tenant_id='{tenant}'::uuid AND lower(email)='$LitAdminLegacy';")
    if ($legacyBefore -ne $legacyAfter -or $legacyAfter -ne $LitAdminLegacy) { Fail "partial mutation after collision abort (legacy email)" }
    if ($collisionBefore -ne $collisionAfter -or $collisionAfter -ne $LitAdminCanonical) { Fail "collision target user changed" }
    if ($legacy7Count -ne "1") { Fail "partial mutation after collision abort (legacy count)" }
    Pass "collision fail-closed verified"

    Step "Phase 2 - canonical seed validation (fresh disposable DB)"
    Start-SeedStack

    Invoke-BashSeed "seed_dev_admin.sh"
    Invoke-BashSeed "seed_demo_data.sh" -AllowFailure
    Invoke-BashSeed "seed_dev_admin.sh"
    Invoke-BashSeed "seed_demo_data.sh" -AllowFailure

    foreach ($email in @($LitAdminCanonical, $LitShipperCanonical, $LitCarrierCanonical, $LitForwarderCanonical, $LitConsigneeCanonical)) {
        Assert-UserCount $email "1"
    }
    $old7Seed = Invoke-PsqlScalar (Sql-WithTenant "SELECT count(*) FROM core.users WHERE tenant_id='{tenant}'::uuid AND deleted_at IS NULL AND lower(email) LIKE '$LitOldDomainLike';")
    if ($old7Seed -ne '0') { Fail ('expected 0 legacy-domain users after seed, got ' + $old7Seed) }
    $tenantCode = Invoke-PsqlScalar (Sql-WithTenant "SELECT code FROM core.tenants WHERE id='{tenant}'::uuid;")
    if ($tenantCode -ne "dev-bintrans") { Fail "expected tenant code dev-bintrans, got $tenantCode" }

    if (-not (Test-IdentityLogin $LitAdminCanonical "Admin123456!")) { Fail ('login failed for ' + $LitAdminCanonical) }
    if (-not (Test-IdentityLogin $LitShipperCanonical "Demo123456!")) { Fail ('login failed for ' + $LitShipperCanonical) }
    if (-not (Test-IdentityLogin $LitCarrierCanonical "Demo123456!")) { Fail ('login failed for ' + $LitCarrierCanonical) }
    if (-not (Test-IdentityLogin $LitForwarderCanonical "Demo123456!")) { Fail ('login failed for ' + $LitForwarderCanonical) }
    if (-not (Test-IdentityLogin $LitConsigneeCanonical "Demo123456!")) { Fail ('login failed for ' + $LitConsigneeCanonical) }
    Pass "seed idempotency + login validation"

    Write-Host ""
    Write-Host "A21_LOCAL_VALIDATION=PASS"
    Write-Host "WINDOWS_VALIDATOR=PASS"
    Write-Host "MIGRATE_FORCE_USED=$MigrateForceUsed"
    Write-Host "MIGRATE_FORCE_SCOPE=$MigrateForceScope"
}
finally {
    Stop-Stack
}
