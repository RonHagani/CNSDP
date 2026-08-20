<#
.SYNOPSIS
  Safely, idempotently removes every resource
  scripts/provision-cloudtrail-bridge.ps1 creates (ADR-0006).

.DESCRIPTION
  Removes resources in dependency-safe order:
    1. EventBridge target(s), then the rule itself (a rule cannot be
       deleted while it still has targets attached)
    2. CloudTrail trail: stop logging, then delete
    3. SQS main queue and DLQ
    4. IAM role: its inline policy, then the role itself
    5. S3 bucket: every object (there is no native "empty bucket" S3
       API -- objects are listed and batch-deleted first), then the
       bucket policy, then the bucket

  Every step first checks whether its target exists and skips cleanly if
  not -- re-running this script, or running it after a partially
  completed provisioning run, is safe.

  Every resource this script touches is named deterministically from
  -NamePrefix (the same fixed "cnsdp-cloudtrail-bridge-*" prefix
  provisioning uses) -- this script never discovers or deletes a
  resource by broader search or wildcard; anything it removes is either
  one of these exact named resources, or a child (a target, an inline
  policy, an object) found by listing strictly within one of these exact
  named resources, so nothing outside CNSDP's own naming/resource
  identity is ever touched.

.PARAMETER AwsCliProfile
  Optional --profile passed to every AWS CLI call (should normally match
  whichever operator identity ran the provisioning script).

.PARAMETER NamePrefix
  Must match the -NamePrefix used at provisioning time.

.PARAMETER Region
  Must match the -Region used at provisioning time (default us-east-1).

.EXAMPLE
  ./scripts/teardown-cloudtrail-bridge.ps1
#>
[CmdletBinding()]
param(
    [string]$AwsCliProfile,
    [string]$NamePrefix = "cnsdp-cloudtrail-bridge",
    [string]$Region = "us-east-1"
)

$ErrorActionPreference = "Stop"

# -----------------------------------------------------------------------
# Helpers (identical to scripts/provision-cloudtrail-bridge.ps1's own --
# duplicated rather than factored into a shared module: two ~15-line
# helper blocks is a smaller footprint than a third shared script file
# for this repository's approved file list).
# -----------------------------------------------------------------------

function Write-Section {
    param([Parameter(Mandatory = $true)][string]$Title)
    Write-Host ""
    Write-Host "== $Title ==" -ForegroundColor Cyan
}

function ConvertTo-WindowsCommandLineArgument {
    <#
    Quotes one argument for a native Windows process command line,
    following the documented C runtime / CommandLineToArgvW parsing
    rules (the same rules .NET's own ProcessStartInfo.ArgumentList
    applies where that property is available).

    Needed here because:
      - ArgumentList is not available on every .NET Framework build
        Windows PowerShell 5.1 can run against (confirmed absent in
        this environment), so ProcessStartInfo.Arguments must be built
        as a single, correctly-escaped command-line string instead, and
      - PowerShell's own "& aws @args" array-splat invocation silently
        strips embedded double-quote characters from an argument before
        aws.exe ever sees them, which corrupts any argument carrying
        inline JSON -- the root cause this remediation pass fixes (see
        Invoke-AwsCliRaw and Write-JsonToTempFile).
    #>
    param([Parameter(Mandatory = $true)][AllowEmptyString()][string]$Argument)

    if ($Argument.Length -gt 0 -and $Argument -notmatch '[\s"]') {
        return $Argument
    }

    $sb = New-Object System.Text.StringBuilder
    [void]$sb.Append('"')
    for ($i = 0; $i -lt $Argument.Length; $i++) {
        $backslashes = 0
        while ($i -lt $Argument.Length -and $Argument[$i] -eq '\') {
            $backslashes++
            $i++
        }
        if ($i -eq $Argument.Length) {
            [void]$sb.Append('\' * ($backslashes * 2))
            break
        }
        elseif ($Argument[$i] -eq '"') {
            [void]$sb.Append('\' * ($backslashes * 2 + 1))
            [void]$sb.Append('"')
        }
        else {
            [void]$sb.Append('\' * $backslashes)
            [void]$sb.Append($Argument[$i])
        }
    }
    [void]$sb.Append('"')
    return $sb.ToString()
}

function Invoke-AwsCliRaw {
    <#
    Runs the AWS CLI as a native process via System.Diagnostics.Process,
    with stdout and stderr captured into SEPARATE, unmangled strings and
    the exit code preserved exactly. Never throws -- callers decide what
    a given ExitCode/StdOut/StdErr combination means.

    Deliberately does not use PowerShell's own native-command
    redirection operators ("2>&1" or "2>file"): both were found,
    empirically, to wrap ordinary stderr output from a native process in
    a formatted ErrorRecord and, under $ErrorActionPreference = "Stop",
    to abort the script even on a successful (exit 0) call that merely
    wrote something to stderr -- which would make a routine AWS CLI
    warning look like a script failure, or corrupt JSON stdout that a
    caller merges with stderr before parsing. Going through
    ProcessStartInfo directly avoids both problems.

    Reads stdout and stderr via concurrent async tasks, started
    immediately after the process starts and before any blocking wait --
    never via two sequential ReadToEnd() calls. Sequential ReadToEnd()
    (stdout fully, then stderr) is a classic Process pipe-buffer deadlock:
    if the child writes enough to the stream not yet being read to fill
    its OS pipe buffer, the child blocks on that write, can never exit,
    the in-progress ReadToEnd() on the other stream (which only returns
    at EOF, i.e. child exit) never returns, and stderr is never reached
    either. Starting both reads as async tasks first lets the CLR drain
    both pipes independently of WaitForExit(), so neither stream's buffer
    can ever back up regardless of how much either one writes.
    #>
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $fullArgs = $Arguments + @("--region", $Region)
    if ($AwsCliProfile) { $fullArgs += @("--profile", $AwsCliProfile) }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = "aws"
    $psi.Arguments = (($fullArgs | ForEach-Object { ConvertTo-WindowsCommandLineArgument $_ }) -join ' ')
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true

    $proc = [System.Diagnostics.Process]::Start($psi)
    $stdOutTask = $proc.StandardOutput.ReadToEndAsync()
    $stdErrTask = $proc.StandardError.ReadToEndAsync()
    $proc.WaitForExit()
    $stdOut = $stdOutTask.GetAwaiter().GetResult()
    $stdErr = $stdErrTask.GetAwaiter().GetResult()

    return @{ ExitCode = $proc.ExitCode; StdOut = $stdOut; StdErr = $stdErr }
}

function Invoke-AwsCli {
    <#
    Runs the AWS CLI and throws (with a bounded stderr excerpt) on a
    nonzero exit code. Returns stdout only: stderr never flows into the
    returned value, so it can never corrupt a caller's ConvertFrom-Json
    (Invoke-AwsCliJson) the way a merged stdout+stderr stream could.
    #>
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $result = Invoke-AwsCliRaw -Arguments $Arguments
    if ($result.ExitCode -ne 0) {
        throw "aws $($Arguments -join ' ') failed (exit $($result.ExitCode)):`n$($result.StdErr)"
    }
    return $result.StdOut
}

function Invoke-AwsCliJson {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $out = Invoke-AwsCli -Arguments $Arguments
    if ([string]::IsNullOrWhiteSpace([string]$out)) { return $null }
    return ($out | ConvertFrom-Json)
}

function Test-AwsResourceExists {
    <#
    Runs a read-only describe/get/head command and reports whether the
    resource exists by matching the ACTUAL, documented not-found error
    AWS returns for that specific command (-NotFoundPattern) -- never by
    exit code alone. Only that recognized not-found signature is
    treated as "does not exist"; any other failure (permission denied,
    throttling, network error, malformed request, invalid credentials,
    or anything unrecognized) is propagated as a thrown error, since
    silently treating an inaccessible-but-possibly-existing resource as
    absent is unsafe: teardown could report "nothing to do" (and skip
    a resource that is actually still there) when it never actually got
    a real answer.
    #>
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$NotFoundPattern
    )
    $result = Invoke-AwsCliRaw -Arguments $Arguments
    if ($result.ExitCode -eq 0) { return $true }
    if ($result.StdErr -match $NotFoundPattern) { return $false }
    throw "aws $($Arguments -join ' ') failed in an unexpected way while checking whether the resource exists (exit $($result.ExitCode)):`n$($result.StdErr)"
}

function Write-JsonToTempFile {
    <#
    AWS CLI policy-document and other JSON arguments are more reliable
    read from file://<path> than passed inline (Windows PowerShell 5.1's
    native-argument passing silently strips embedded double quotes from
    inline arguments -- see ConvertTo-WindowsCommandLineArgument) -- this
    writes $Json to a temp file and returns its path; callers are
    responsible for removing it.

    Written via [System.IO.File]::WriteAllText with an explicit,
    BOM-less UTF8Encoding: Set-Content -Encoding utf8 on Windows
    PowerShell 5.1 prepends a UTF-8 byte-order mark, which some JSON
    readers (including, historically, some AWS CLI/botocore versions)
    reject as invalid JSON.
    #>
    param([Parameter(Mandatory = $true)][string]$Json)
    $path = [System.IO.Path]::GetTempFileName()
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($path, $Json, $utf8NoBom)
    return $path
}

# -----------------------------------------------------------------------
# Preflight
# -----------------------------------------------------------------------

if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    throw "AWS CLI ('aws') not found on PATH."
}

Write-Section "Resolving AWS account ID"
$callerIdentity = Invoke-AwsCliJson -Arguments @("sts", "get-caller-identity", "--output", "json")
$accountId = $callerIdentity.Account
if (-not $accountId) { throw "Could not resolve AWS account ID from 'aws sts get-caller-identity'." }
Write-Host "  Account: $accountId"

$bucketName     = "$NamePrefix-logs-$accountId"
$trailName      = "$NamePrefix-trail"
$ruleName       = "$NamePrefix-rule"
$queueName      = "$NamePrefix-queue"
$dlqName        = "$NamePrefix-dlq"
$roleName       = "$NamePrefix-role"

# -----------------------------------------------------------------------
# 1. EventBridge target(s), then the rule.
# -----------------------------------------------------------------------

Write-Section "[1/5] EventBridge rule: $ruleName"
if (Test-AwsResourceExists -Arguments @("events", "describe-rule", "--name", $ruleName) -NotFoundPattern 'ResourceNotFoundException') {
    $existingTargets = Invoke-AwsCliJson -Arguments @("events", "list-targets-by-rule", "--rule", $ruleName)
    if ($existingTargets -and $existingTargets.Targets -and $existingTargets.Targets.Count -gt 0) {
        $targetIds = @($existingTargets.Targets | ForEach-Object { $_.Id })
        Invoke-AwsCli -Arguments (@("events", "remove-targets", "--rule", $ruleName, "--ids") + $targetIds) | Out-Null
        Write-Host "  removed $($targetIds.Count) target(s)"
    }
    Invoke-AwsCli -Arguments @("events", "delete-rule", "--name", $ruleName) | Out-Null
    Write-Host "  rule deleted"
}
else {
    Write-Host "  '$ruleName' not found -- skipping"
}

# -----------------------------------------------------------------------
# 2. CloudTrail trail: stop logging, then delete.
# -----------------------------------------------------------------------

Write-Section "[2/5] CloudTrail trail: $trailName"
if (Test-AwsResourceExists -Arguments @("cloudtrail", "get-trail-status", "--name", $trailName) -NotFoundPattern 'TrailNotFoundException') {
    try {
        Invoke-AwsCli -Arguments @("cloudtrail", "stop-logging", "--name", $trailName) | Out-Null
    }
    catch {
        Write-Warning "  stop-logging failed (continuing to delete-trail regardless): $_"
    }
    Invoke-AwsCli -Arguments @("cloudtrail", "delete-trail", "--name", $trailName) | Out-Null
    Write-Host "  trail stopped and deleted"
}
else {
    Write-Host "  '$trailName' not found -- skipping"
}

# -----------------------------------------------------------------------
# 3. SQS main queue and DLQ.
# -----------------------------------------------------------------------

Write-Section "[3/5] SQS queues: $queueName, $dlqName"
foreach ($qName in @($queueName, $dlqName)) {
    if (Test-AwsResourceExists -Arguments @("sqs", "get-queue-url", "--queue-name", $qName) -NotFoundPattern 'NonExistentQueue') {
        $qUrl = (Invoke-AwsCliJson -Arguments @("sqs", "get-queue-url", "--queue-name", $qName)).QueueUrl
        Invoke-AwsCli -Arguments @("sqs", "delete-queue", "--queue-url", $qUrl) | Out-Null
        Write-Host "  deleted '$qName'"
    }
    else {
        Write-Host "  '$qName' not found -- skipping"
    }
}

# -----------------------------------------------------------------------
# 4. IAM role: inline polic(ies), then the role.
# -----------------------------------------------------------------------

Write-Section "[4/5] IAM role: $roleName"
if (Test-AwsResourceExists -Arguments @("iam", "get-role", "--role-name", $roleName) -NotFoundPattern 'NoSuchEntity') {
    $inlinePolicies = Invoke-AwsCliJson -Arguments @("iam", "list-role-policies", "--role-name", $roleName)
    foreach ($pName in @($inlinePolicies.PolicyNames)) {
        Invoke-AwsCli -Arguments @("iam", "delete-role-policy", "--role-name", $roleName, "--policy-name", $pName) | Out-Null
        Write-Host "  removed inline policy '$pName'"
    }
    Invoke-AwsCli -Arguments @("iam", "delete-role", "--role-name", $roleName) | Out-Null
    Write-Host "  role deleted"
}
else {
    Write-Host "  '$roleName' not found -- skipping"
}

# Note: this script only ever removes the default role-based credential
# path above. If the separately reviewed, optional IAM-user/access-key
# fallback is ever introduced in a future change, its own teardown must
# be added there -- it does not exist in this slice (constraint 1 of the
# approved implementation plan), so there is nothing of that kind to
# detect or remove here.

# -----------------------------------------------------------------------
# 5. S3 bucket: every object, then the bucket policy, then the bucket.
# -----------------------------------------------------------------------

Write-Section "[5/5] S3 bucket: $bucketName"
if (Test-AwsResourceExists -Arguments @("s3api", "head-bucket", "--bucket", $bucketName) -NotFoundPattern '\(404\)') {
    $deleted = 0
    while ($true) {
        $listing = Invoke-AwsCliJson -Arguments @("s3api", "list-objects-v2", "--bucket", $bucketName, "--max-keys", "1000")
        if (-not $listing -or -not $listing.Contents -or $listing.Contents.Count -eq 0) { break }

        $keys = @($listing.Contents | ForEach-Object { @{ Key = $_.Key } })
        $deleteDoc = @{ Objects = $keys; Quiet = $true } | ConvertTo-Json -Depth 5
        $delFile = Write-JsonToTempFile -Json $deleteDoc
        try {
            Invoke-AwsCli -Arguments @("s3api", "delete-objects", "--bucket", $bucketName, "--delete", "file://$delFile") | Out-Null
        }
        finally {
            Remove-Item $delFile -ErrorAction SilentlyContinue
        }
        $deleted += $keys.Count

        if (-not $listing.IsTruncated) { break }
    }
    Write-Host "  removed $deleted object(s)"

    try {
        Invoke-AwsCli -Arguments @("s3api", "delete-bucket-policy", "--bucket", $bucketName) | Out-Null
    }
    catch {
        Write-Warning "  could not remove bucket policy explicitly (continuing -- bucket deletion removes it regardless): $_"
    }

    Invoke-AwsCli -Arguments @("s3api", "delete-bucket", "--bucket", $bucketName) | Out-Null
    Write-Host "  bucket deleted"
}
else {
    Write-Host "  '$bucketName' not found -- skipping"
}

Write-Section "Teardown complete"
