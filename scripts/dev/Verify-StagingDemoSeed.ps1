# Read-only staging demo seed verification (Windows).
param(
  [string]$Base = "http://161.104.53.221",
  [string]$Tenant = "74519f22-ff9b-4a8b-8fff-a958c689682f"
)

$h = @{ "X-Tenant-ID" = $Tenant }

function Test-ReadOnly($Id, $Url, $ExpectPattern) {
  try {
    $r = Invoke-WebRequest -UseBasicParsing -Uri $Url -Headers $h -TimeoutSec 20
    $ok = ($r.StatusCode -eq 200) -and ($r.Content -match $ExpectPattern)
    if ($ok) { "PASS $Id StatusCode=$($r.StatusCode)" } else { "CHECK $Id StatusCode=$($r.StatusCode) pattern=$ExpectPattern" }
  } catch {
    $code = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { "ERR" }
    "FAIL $Id StatusCode=$code"
  }
}

Write-Output "=== Verify-StagingDemoSeed ==="
try {
  $health = Invoke-WebRequest -UseBasicParsing -Uri "$Base/health" -TimeoutSec 20
  Write-Output "PASS VFY-001 health=$($health.StatusCode)"
} catch {
  Write-Output "FAIL VFY-001 health"
}

Test-ReadOnly "VFY-002" "$Base/api/v1/transport-orders?tenant_id=$Tenant&limit=10" "DEMO-TO"
Test-ReadOnly "VFY-003" "$Base/api/v1/shipments?tenant_id=$Tenant&limit=10" "DEMO-SH"
Test-ReadOnly "VFY-004" "$Base/api/v1/billing-registers?tenant_id=$Tenant&limit=10" "DEMO-BR"
Test-ReadOnly "VFY-005" "$Base/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER" "entity_type"
