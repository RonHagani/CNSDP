<#
.SYNOPSIS
  Idempotent provisioning for CNSDP's live AWS CloudTrail delivery bridge
  (ADR-0006, docs/adr/0006-aws-cloudtrail-delivery-and-intake-topology.md).

.DESCRIPTION
  Creates, or safely re-applies configuration to, exactly the approved
  resources for the smallest live-delivery slice:

    - an S3 bucket for CloudTrail's required log destination, with a
      public-access block and the exact CloudTrail-required bucket
      policy (scoped to this one trail's ARN via aws:SourceArn)
    - a single-region CloudTrail trail (us-east-1, write-only management
      events, global service events included) + StartLogging
    - an EventBridge rule on the default bus (source: aws.iam,
      detail-type: "AWS API Call via CloudTrail", state: ENABLED)
    - an SQS main queue + DLQ, with a redrive policy (maxReceiveCount 50)
      and a main-queue resource policy restricted to this exact
      EventBridge rule's ARN (confused-deputy prevention via
      aws:SourceArn) -- no EventBridge execution role is created; the
      rule's SQS target relies on that resource-based policy instead
    - cnsdp-cloudtrail-bridge-role: a dedicated IAM role the bridge
      assumes at runtime for short-lived STS credentials, scoped to
      exactly sqs:ReceiveMessage/DeleteMessage/GetQueueAttributes on the
      one main queue

  This script deliberately does NOT create an IAM user or any long-lived
  AWS access key. A static-key fallback is out of scope for this slice
  (a separately reviewed future change).

  Every resource is named with the fixed "cnsdp-cloudtrail-bridge-*"
  prefix (see $NamePrefix) and is created only if it does not already
  exist; every policy/attribute-setting call is a natural AWS "upsert"
  (Put*/Set*) and is re-applied unconditionally on every run, so this
  script is safe to re-run to converge a partial or manually-edited
  prior run back to the approved configuration.

  Every AWS CLI JSON parameter (event selectors, queue attributes,
  the EventBridge pattern, the EventBridge target list, IAM/S3 policy
  documents) is passed via a temp file and AWS CLI's own file://
  loading, never inline on the command line: Windows PowerShell 5.1's
  native-argument passing silently strips embedded double-quote
  characters from inline arguments before aws.exe ever sees them, which
  corrupts inline JSON. See Write-JsonToTempFile.

.PARAMETER OperatorPrincipalArn
  The STABLE IAM role or user ARN (arn:aws:iam::<account-id>:role/... or
  arn:aws:iam::<account-id>:user/...) authorized to assume
  cnsdp-cloudtrail-bridge-role. This must NOT be an STS assumed-role
  session ARN (arn:aws:sts::<account-id>:assumed-role/...) -- those are
  ephemeral per-login-session identifiers, not a durable IAM trust
  principal, and this script refuses to accept one. If you authenticate
  via AWS SSO / IAM Identity Center, this is your permission set's
  underlying IAM role ARN, which you can find with e.g.:
    aws iam list-roles --query "Roles[?contains(RoleName,'<permission-set-name>')].Arn" --output text

.PARAMETER AwsCliProfile
  Optional --profile passed to every AWS CLI call this script makes --
  the PROVISIONING operator's own credentials (distinct from the
  bridge's runtime role). Defaults to whatever profile/credentials are
  already active in this shell.

.PARAMETER NamePrefix
  Naming prefix for every resource this script creates or looks up.
  Changing it from the default means teardown must be run with the same
  value to find the same resources.

.PARAMETER Region
  Fixed by the approved design at "us-east-1" (IAM/root CloudTrail
  events are always recorded there) -- overriding it is not a supported
  configuration for this slice.

.EXAMPLE
  ./scripts/provision-cloudtrail-bridge.ps1 -OperatorPrincipalArn arn:aws:iam::123456789012:role/MyAdminRole
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$OperatorPrincipalArn,

    [string]$AwsCliProfile,

    [string]$NamePrefix = "cnsdp-cloudtrail-bridge",

    [string]$Region = "us-east-1"
)

$ErrorActionPreference = "Stop"

# -----------------------------------------------------------------------
# Helpers
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
    absent is unsafe: provisioning could then attempt to create/adopt
    something that already exists, and teardown could report "nothing
    to do" when it actually never got a real answer.
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

function Test-StableIamPrincipalArn {
    param([Parameter(Mandatory = $true)][string]$Arn)

    if ($Arn -match '^arn:aws:sts::\d{12}:assumed-role/') {
        throw @"
-OperatorPrincipalArn must be a STABLE IAM role or user ARN, not an STS
assumed-role session ARN. Got: '$Arn'

Assumed-role session ARNs (arn:aws:sts::<account-id>:assumed-role/<role-name>/<session-name>)
are ephemeral -- a new one is minted every time a role is assumed, including
on every 'aws sso login' -- and cannot be used as a durable IAM
trust-policy principal; the trust relationship would silently stop
matching the next time you log in.

Supply the underlying IAM role ARN instead. For an AWS SSO / IAM
Identity Center permission set, find it with:
  aws iam list-roles --query "Roles[?contains(RoleName,'<permission-set-name>')].Arn" --output text
For a plain IAM user: arn:aws:iam::<account-id>:user/<your-username>
"@
    }
    if ($Arn -notmatch '^arn:aws:iam::\d{12}:(role|user)/.+$') {
        throw "-OperatorPrincipalArn must match arn:aws:iam::<12-digit-account-id>:role/<name> or arn:aws:iam::<12-digit-account-id>:user/<name>. Got: '$Arn'"
    }
}

# -----------------------------------------------------------------------
# Preflight
# -----------------------------------------------------------------------

Test-StableIamPrincipalArn -Arn $OperatorPrincipalArn

if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    throw "AWS CLI ('aws') not found on PATH. Install it and authenticate (e.g. 'aws configure sso') before running this script."
}

Write-Section "Resolving AWS account ID"
# Only the numeric Account field is used -- never the Arn field, which
# would be the operator's own (possibly assumed-role, possibly ephemeral)
# session identity, not a fit substitute for -OperatorPrincipalArn.
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
$rolePolicyName = "$NamePrefix-sqs-policy"

# The trail's and role's ARNs are deterministic from account id + region
# + the fixed name this script chooses -- computed directly rather than
# fetched after creation, which also resolves the bucket-policy/trail
# creation-order dependency below (the bucket policy's aws:SourceArn
# condition must name the trail before the trail exists).
$trailArn = "arn:aws:cloudtrail:${Region}:${accountId}:trail/$trailName"
$roleArn  = "arn:aws:iam::${accountId}:role/$roleName"

# -----------------------------------------------------------------------
# 1. S3 bucket (CloudTrail's required log destination -- this slice's
#    delivery path is EventBridge/SQS, not S3; nothing ever reads this
#    bucket's contents).
# -----------------------------------------------------------------------

Write-Section "[1/7] S3 bucket: $bucketName"
if (Test-AwsResourceExists -Arguments @("s3api", "head-bucket", "--bucket", $bucketName) -NotFoundPattern '\(404\)') {
    Write-Host "  already exists"
}
else {
    if ($Region -eq "us-east-1") {
        # us-east-1 is the one region where --create-bucket-configuration
        # must be omitted entirely -- passing LocationConstraint=us-east-1
        # explicitly is rejected by S3.
        Invoke-AwsCli -Arguments @("s3api", "create-bucket", "--bucket", $bucketName) | Out-Null
    }
    else {
        Invoke-AwsCli -Arguments @("s3api", "create-bucket", "--bucket", $bucketName, "--create-bucket-configuration", "LocationConstraint=$Region") | Out-Null
    }
    Write-Host "  created"
}

Write-Host "  applying public-access block (idempotent)"
Invoke-AwsCli -Arguments @(
    "s3api", "put-public-access-block", "--bucket", $bucketName,
    "--public-access-block-configuration", "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
) | Out-Null

Write-Host "  applying CloudTrail bucket policy (scoped to this trail's ARN and AWSLogs/$accountId/*)"
$bucketPolicy = @{
    Version   = "2012-10-17"
    Statement = @(
        @{
            Sid       = "AWSCloudTrailAclCheck"
            Effect    = "Allow"
            Principal = @{ Service = "cloudtrail.amazonaws.com" }
            Action    = "s3:GetBucketAcl"
            Resource  = "arn:aws:s3:::$bucketName"
            Condition = @{ StringEquals = @{ "aws:SourceArn" = $trailArn } }
        },
        @{
            Sid       = "AWSCloudTrailWrite"
            Effect    = "Allow"
            Principal = @{ Service = "cloudtrail.amazonaws.com" }
            Action    = "s3:PutObject"
            Resource  = "arn:aws:s3:::$bucketName/AWSLogs/$accountId/*"
            Condition = @{
                StringEquals = @{
                    "aws:SourceArn" = $trailArn
                    "s3:x-amz-acl"  = "bucket-owner-full-control"
                }
            }
        }
    )
} | ConvertTo-Json -Depth 10

$policyFile = Write-JsonToTempFile -Json $bucketPolicy
try {
    Invoke-AwsCli -Arguments @("s3api", "put-bucket-policy", "--bucket", $bucketName, "--policy", "file://$policyFile") | Out-Null
}
finally {
    Remove-Item $policyFile -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------
# 2. CloudTrail trail: single-region (us-east-1), write-only management
#    events, global service events included.
# -----------------------------------------------------------------------

Write-Section "[2/7] CloudTrail trail: $trailName"
if (Test-AwsResourceExists -Arguments @("cloudtrail", "get-trail-status", "--name", $trailName) -NotFoundPattern 'TrailNotFoundException') {
    Write-Host "  already exists"
}
else {
    Invoke-AwsCli -Arguments @(
        "cloudtrail", "create-trail",
        "--name", $trailName,
        "--s3-bucket-name", $bucketName,
        "--no-is-multi-region-trail",
        "--include-global-service-events"
    ) | Out-Null
    Write-Host "  created"
}

Write-Host "  applying event selectors (write-only management events)"
$eventSelectorsJson = '[{"ReadWriteType":"WriteOnly","IncludeManagementEvents":true,"DataResources":[]}]'
$eventSelectorsFile = Write-JsonToTempFile -Json $eventSelectorsJson
try {
    Invoke-AwsCli -Arguments @(
        "cloudtrail", "put-event-selectors",
        "--trail-name", $trailName,
        "--event-selectors", "file://$eventSelectorsFile"
    ) | Out-Null
}
finally {
    Remove-Item $eventSelectorsFile -ErrorAction SilentlyContinue
}

Write-Host "  starting logging (idempotent)"
Invoke-AwsCli -Arguments @("cloudtrail", "start-logging", "--name", $trailName) | Out-Null

# -----------------------------------------------------------------------
# 3. SQS DLQ, then main queue (main queue's redrive policy needs the
#    DLQ's ARN, so the DLQ must exist first).
# -----------------------------------------------------------------------

Write-Section "[3/7] SQS DLQ: $dlqName"
if (Test-AwsResourceExists -Arguments @("sqs", "get-queue-url", "--queue-name", $dlqName) -NotFoundPattern 'NonExistentQueue') {
    $dlqUrl = (Invoke-AwsCliJson -Arguments @("sqs", "get-queue-url", "--queue-name", $dlqName)).QueueUrl
    Write-Host "  already exists: $dlqUrl"
}
else {
    $dlqUrl = (Invoke-AwsCliJson -Arguments @("sqs", "create-queue", "--queue-name", $dlqName)).QueueUrl
    Write-Host "  created: $dlqUrl"
}
$dlqArn = (Invoke-AwsCliJson -Arguments @("sqs", "get-queue-attributes", "--queue-url", $dlqUrl, "--attribute-names", "QueueArn")).Attributes.QueueArn

Write-Host "  applying MessageRetentionPeriod=1209600 (14 days, idempotent -- reapplied on every run)"
$dlqAttrsJson = @{ MessageRetentionPeriod = "1209600" } | ConvertTo-Json -Compress
$dlqAttrsFile = Write-JsonToTempFile -Json $dlqAttrsJson
try {
    Invoke-AwsCli -Arguments @("sqs", "set-queue-attributes", "--queue-url", $dlqUrl, "--attributes", "file://$dlqAttrsFile") | Out-Null
}
finally {
    Remove-Item $dlqAttrsFile -ErrorAction SilentlyContinue
}

Write-Section "[4/7] SQS main queue: $queueName"
if (Test-AwsResourceExists -Arguments @("sqs", "get-queue-url", "--queue-name", $queueName) -NotFoundPattern 'NonExistentQueue') {
    $queueUrl = (Invoke-AwsCliJson -Arguments @("sqs", "get-queue-url", "--queue-name", $queueName)).QueueUrl
    Write-Host "  already exists: $queueUrl"
}
else {
    $queueUrl = (Invoke-AwsCliJson -Arguments @("sqs", "create-queue", "--queue-name", $queueName)).QueueUrl
    Write-Host "  created: $queueUrl"
}
$queueArn = (Invoke-AwsCliJson -Arguments @("sqs", "get-queue-attributes", "--queue-url", $queueUrl, "--attribute-names", "QueueArn")).Attributes.QueueArn

Write-Host "  applying VisibilityTimeout=30 and RedrivePolicy (maxReceiveCount=50 -> DLQ)"
$redrivePolicy = @{ deadLetterTargetArn = $dlqArn; maxReceiveCount = 50 } | ConvertTo-Json -Compress
$queueAttrsJson = @{
    VisibilityTimeout = "30"
    RedrivePolicy     = $redrivePolicy
} | ConvertTo-Json -Compress
$queueAttrsFile = Write-JsonToTempFile -Json $queueAttrsJson
try {
    Invoke-AwsCli -Arguments @("sqs", "set-queue-attributes", "--queue-url", $queueUrl, "--attributes", "file://$queueAttrsFile") | Out-Null
}
finally {
    Remove-Item $queueAttrsFile -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------
# 4. EventBridge rule + main-queue resource policy + target.
# -----------------------------------------------------------------------

Write-Section "[5/7] EventBridge rule: $ruleName"
$eventPattern = '{"source":["aws.iam"],"detail-type":["AWS API Call via CloudTrail"],"detail":{"eventSource":["iam.amazonaws.com"]}}'
$eventPatternFile = Write-JsonToTempFile -Json $eventPattern
try {
    $ruleArn = (Invoke-AwsCliJson -Arguments @(
            "events", "put-rule",
            "--name", $ruleName,
            "--event-pattern", "file://$eventPatternFile",
            "--state", "ENABLED"
        )).RuleArn
}
finally {
    Remove-Item $eventPatternFile -ErrorAction SilentlyContinue
}
Write-Host "  rule ARN: $ruleArn"

Write-Host "  applying main-queue resource policy (sqs:SendMessage restricted to this exact rule ARN via aws:SourceArn)"
$queuePolicyDoc = @{
    Version   = "2012-10-17"
    Statement = @(
        @{
            Sid       = "AllowEventBridgeSendMessage"
            Effect    = "Allow"
            Principal = @{ Service = "events.amazonaws.com" }
            Action    = "sqs:SendMessage"
            Resource  = $queueArn
            Condition = @{ ArnEquals = @{ "aws:SourceArn" = $ruleArn } }
        }
    )
} | ConvertTo-Json -Depth 10 -Compress
$queuePolicyAttrsJson = @{ Policy = $queuePolicyDoc } | ConvertTo-Json -Compress
$queuePolicyAttrsFile = Write-JsonToTempFile -Json $queuePolicyAttrsJson
try {
    Invoke-AwsCli -Arguments @("sqs", "set-queue-attributes", "--queue-url", $queueUrl, "--attributes", "file://$queuePolicyAttrsFile") | Out-Null
}
finally {
    Remove-Item $queuePolicyAttrsFile -ErrorAction SilentlyContinue
}

Write-Host "  wiring the SQS target (no EventBridge execution role: the resource policy above is what authorizes delivery)"
# Hand-built one-element JSON array for maximum PowerShell-version
# compatibility (ConvertTo-Json's -AsArray switch is not available on
# Windows PowerShell 5.1); $queueArn is AWS-generated, not
# operator-supplied free text, so no escaping concern applies here.
$targetsJson = '[{"Id":"' + "$NamePrefix-queue-target" + '","Arn":"' + $queueArn + '"}]'
$targetsFile = Write-JsonToTempFile -Json $targetsJson
try {
    Invoke-AwsCli -Arguments @("events", "put-targets", "--rule", $ruleName, "--targets", "file://$targetsFile") | Out-Null
}
finally {
    Remove-Item $targetsFile -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------
# 5. cnsdp-cloudtrail-bridge-role: the bridge's own runtime identity.
#    No IAM user, no access key -- short-lived STS credentials only,
#    obtained by assuming this role. See constraint 1 of the approved
#    implementation plan.
# -----------------------------------------------------------------------

Write-Section "[6/7] IAM role: $roleName"
$trustPolicy = @{
    Version   = "2012-10-17"
    Statement = @(
        @{
            Effect    = "Allow"
            Principal = @{ AWS = $OperatorPrincipalArn }
            Action    = "sts:AssumeRole"
        }
    )
} | ConvertTo-Json -Depth 10

$trustFile = Write-JsonToTempFile -Json $trustPolicy
try {
    if (Test-AwsResourceExists -Arguments @("iam", "get-role", "--role-name", $roleName) -NotFoundPattern 'NoSuchEntity') {
        Write-Host "  already exists -- updating trust policy"
        Invoke-AwsCli -Arguments @("iam", "update-assume-role-policy", "--role-name", $roleName, "--policy-document", "file://$trustFile") | Out-Null
    }
    else {
        Invoke-AwsCli -Arguments @(
            "iam", "create-role",
            "--role-name", $roleName,
            "--assume-role-policy-document", "file://$trustFile",
            "--description", "CNSDP CloudTrail bridge runtime role (ADR-0006). Assumable only by the operator principal supplied at provisioning time. Grants exactly sqs:ReceiveMessage/DeleteMessage/GetQueueAttributes on the single bridge queue -- no other permission."
        ) | Out-Null
        Write-Host "  created"
    }
}
finally {
    Remove-Item $trustFile -ErrorAction SilentlyContinue
}

Write-Host "  applying inline permission policy (sqs:ReceiveMessage/DeleteMessage/GetQueueAttributes, main queue only)"
$permissionPolicy = @{
    Version   = "2012-10-17"
    Statement = @(
        @{
            Effect   = "Allow"
            Action   = @("sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes")
            Resource = $queueArn
        }
    )
} | ConvertTo-Json -Depth 10
$permFile = Write-JsonToTempFile -Json $permissionPolicy
try {
    Invoke-AwsCli -Arguments @("iam", "put-role-policy", "--role-name", $roleName, "--policy-name", $rolePolicyName, "--policy-document", "file://$permFile") | Out-Null
}
finally {
    Remove-Item $permFile -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------
# 6. Summary -- non-secret values only. CNSDP_API_TOKEN is never printed
#    or created by this script.
# -----------------------------------------------------------------------

Write-Section "[7/7] Provisioning complete"
Write-Host @"

Non-secret values for cmd/cloudtrail-bridge/.env (see .env.example):

  AWS_REGION=$Region
  AWS_PROFILE=<a profile in ~/.aws/config with role_arn=$roleArn and source_profile=<your own SSO/CLI profile>>
  SQS_QUEUE_URL=$queueUrl
  CNSDP_ENDPOINT=http://127.0.0.1:8080/v1/cloudtrail-events   # adjust to your running CNSDP instance

Resource names/ARNs (for reference and for teardown):
  S3 bucket:         $bucketName
  CloudTrail trail:  $trailName
                     $trailArn
  EventBridge rule:  $ruleName
                     $ruleArn
  SQS main queue:    $queueName
                     $queueArn
  SQS DLQ:           $dlqName
                     $dlqArn
  IAM role:          $roleName
                     $roleArn

CNSDP_API_TOKEN is NOT printed or created by this script -- obtain the
existing value from your local CNSDP .env (the same shared bearer token
already governing /v1/audit-events, /v1/alerts, and every other
product-exposed path).

Next step: add a profile to ~/.aws/config that assumes $roleArn from
your own authenticated identity, then see docs/reference-environment.md,
"AWS CloudTrail live delivery (bridge)".
"@
