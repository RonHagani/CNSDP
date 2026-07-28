<#
.SYNOPSIS
  Loads real audit-event fixtures into a running CNSDP backend for local
  development, via the real ingestion API (POST /v1/audit-events).

.DESCRIPTION
  Development convenience only -- never wired into production code, never
  run automatically, never seeds anything other than whatever -BaseUrl you
  point it at (default: the local docker-compose reference environment on
  this host). See docs/reference-environment.md, "Loading development
  alerts".

  Submits the three already-committed, real Spike-1-captured fixtures used
  throughout this repository's own tests
  (internal/intake/testdata/scenario-1-eventlist.json,
  internal/validation/testdata/scenario{2,3}-valid.json) -- not frontend
  fixtures, and not synthetic data invented for this script.

  Idempotent by construction, not by any special-casing in this script:
  submission admission is keyed by a deterministic source_key derived from
  each event's auditID/auditStage (internal/submission/submission.go,
  migrations/0002_submission_source_key.up.sql). Running this script again
  against the same database resolves each fixture to its existing row
  instead of creating a duplicate alert.

.PARAMETER BaseUrl
  Backend base URL. Defaults to http://127.0.0.1:<APP_PORT>, with APP_PORT
  read from the repository root .env (falling back to 8080, docker-compose.yml's
  own default).

.PARAMETER ApiToken
  Bearer token for the ingestion API. Defaults to API_TOKEN read from the
  repository root .env. Never printed by this script, in output or errors.

.EXAMPLE
  ./scripts/dev-seed-alerts.ps1
#>
param(
    [string]$BaseUrl,
    [string]$ApiToken
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $repoRoot ".env"

function Read-DotEnvValue {
    param([string]$Path, [string]$Key)
    if (-not (Test-Path $Path)) { return $null }
    $line = Get-Content $Path | Where-Object { $_ -match "^\s*$Key\s*=" } | Select-Object -First 1
    if (-not $line) { return $null }
    return ($line -split "=", 2)[1].Trim()
}

if (-not $ApiToken) {
    $ApiToken = Read-DotEnvValue -Path $envFile -Key "API_TOKEN"
}
if (-not $ApiToken) {
    throw "API_TOKEN was not provided and could not be read from $envFile. Copy .env.example to .env and set it (see docs/reference-environment.md), or pass -ApiToken explicitly."
}

if (-not $BaseUrl) {
    $appPort = Read-DotEnvValue -Path $envFile -Key "APP_PORT"
    if (-not $appPort) { $appPort = "8080" }
    $BaseUrl = "http://127.0.0.1:$appPort"
}

Write-Host "Seeding development alerts against $BaseUrl (credential loaded from .env, not displayed)."

$fixtures = @(
    @{ Name = "scenario-1 (Interactive exec request)"; Path = Join-Path $repoRoot "internal/intake/testdata/scenario-1-eventlist.json"; Wrap = $false },
    @{ Name = "scenario-2 (High-risk Pod creation)"; Path = Join-Path $repoRoot "internal/validation/testdata/scenario2-valid.json"; Wrap = $true },
    @{ Name = "scenario-3 (Cluster-admin ClusterRoleBinding grant)"; Path = Join-Path $repoRoot "internal/validation/testdata/scenario3-valid.json"; Wrap = $true }
)

$headers = @{ Authorization = "Bearer $ApiToken" }

foreach ($fixture in $fixtures) {
    if (-not (Test-Path $fixture.Path)) {
        throw "Fixture not found: $($fixture.Path)"
    }
    $raw = (Get-Content -Raw -Path $fixture.Path).Trim()
    if ($fixture.Wrap) {
        $body = '{"kind":"EventList","apiVersion":"audit.k8s.io/v1","items":[' + $raw + ']}'
    } else {
        $body = $raw
    }

    try {
        $response = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/audit-events" -Headers $headers -Body $body -ContentType "application/json"
        $submissionId = $response.results[0].id
        Write-Host "  [ok] $($fixture.Name) -> submission id $submissionId"
    } catch {
        $statusCode = $null
        if ($_.Exception.PSObject.Properties.Match("Response").Count -gt 0 -and $_.Exception.Response) {
            $statusCode = $_.Exception.Response.StatusCode.value__
        }
        throw "Failed to submit $($fixture.Name) (HTTP $statusCode): $($_.Exception.Message)"
    }
}

Write-Host ""
Write-Host "Submitted. The worker loop advances each submission (admitted -> validated ->"
Write-Host "normalized -> evaluated -> alerted -> evidenced) within its poll interval"
Write-Host "(WORKER_POLL_INTERVAL, default 1s) -- reload /alerts in a few seconds."
Write-Host ""
Write-Host "Re-running this script is safe and will not create duplicate alerts: each"
Write-Host "fixture's auditID-derived source key resolves to its existing row."
