package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// projectName is hardcoded, never read from an ambient COMPOSE_PROJECT_NAME
// or a directory-derived default: every single compose invocation this
// harness issues is scoped to exactly this project, so it can never
// address, mutate, stop, or tear down a developer's own (differently- or
// default-named) Compose project or database.
const projectName = "cnsdp-verify"

// sensitiveEnvDenylist names repository-specific variables that must never
// leak from the calling shell's ambient environment into a verification
// compose invocation. Discovered empirically (not assumed): rendering the
// merged config from this repo's own shell without an explicit --env-file
// showed WORKER_POLL_INTERVAL resolving to the developer's real local
// override from the repo root's real .env, via Compose's default .env
// auto-load from the current directory -- proving ambient values reach
// Compose variable substitution even when unrelated to the two variables
// the harness explicitly sets. An explicit --env-file (confirmed
// empirically to suppress the default .env lookup) is the primary
// defense; this denylist removes the same variables from the subprocess's
// own environment as well, since shell-exported values take precedence
// over --env-file per Compose's own documented precedence order.
var sensitiveEnvDenylist = []string{
	"API_TOKEN", "POSTGRES_PASSWORD", "DATABASE_URL", "APP_PORT",
	"WORKER_POLL_INTERVAL", "MAX_BODY_BYTES", "HTTP_ADDR",
	"COMPOSE_PROJECT_NAME",
}

// Compose drives the dedicated cnsdp-verify Compose project: the base
// docker-compose.yml plus docker-compose.verify.yml's port-isolation
// override, always under projectName, always with an explicit,
// harness-generated --env-file. No method on this type ever omits any of
// the three (-f, -f, -p, --env-file) flags.
type Compose struct {
	repoRoot   string
	envFile    string // temp file; removed by Close
	apiToken   string
	pgPassword string
}

// NewCompose resolves the repository root, generates fresh random
// credentials, and writes them to a dedicated temp env file -- never the
// repository's own .env, never a value read from the calling shell. Call
// Close when done to remove the temp file.
func NewCompose() (*Compose, error) {
	root, err := repoRootDir()
	if err != nil {
		return nil, wrapEnvErr("resolve repository root", err)
	}

	apiToken, err := randomHex(32)
	if err != nil {
		return nil, wrapEnvErr("generate API token", err)
	}
	pgPassword, err := randomHex(32)
	if err != nil {
		return nil, wrapEnvErr("generate database password", err)
	}

	envFile, err := os.CreateTemp("", "cnsdp-verify-*.env")
	if err != nil {
		return nil, wrapEnvErr("create verification env file", err)
	}
	defer envFile.Close()
	if _, err := fmt.Fprintf(envFile, "API_TOKEN=%s\nPOSTGRES_PASSWORD=%s\n", apiToken, pgPassword); err != nil {
		os.Remove(envFile.Name())
		return nil, wrapEnvErr("write verification env file", err)
	}

	return &Compose{
		repoRoot:   root,
		envFile:    envFile.Name(),
		apiToken:   apiToken,
		pgPassword: pgPassword,
	}, nil
}

// Close removes the temp credentials file. It never stops or tears down
// the compose project itself -- callers invoke Down explicitly so cleanup
// ordering (down before removing the file it depends on) is always
// visible at the call site.
func (c *Compose) Close() {
	if c.envFile != "" {
		os.Remove(c.envFile)
	}
}

func (c *Compose) APIToken() string { return c.apiToken }

// PostgresPassword returns the generated password for the app's own
// privileged "cnsdp" database role -- used only once, to bootstrap the
// harness's dedicated read-only role (see dbaccess.go); every subsequent
// query uses that read-only role's own credentials, never this one.
func (c *Compose) PostgresPassword() string { return c.pgPassword }

// repoRootDir locates the repository root relative to this source file's
// own location (two directories above test/verification), independent of
// the process's current working directory when the harness is invoked.
func repoRootDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("determine source file location")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err != nil {
		return "", fmt.Errorf("resolved repo root %s does not contain docker-compose.yml: %w", root, err)
	}
	return root, nil
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Compose) baseArgs() []string {
	return []string{
		"compose",
		"-f", filepath.Join(c.repoRoot, "docker-compose.yml"),
		"-f", filepath.Join(c.repoRoot, "docker-compose.verify.yml"),
		"-p", projectName,
		"--env-file", c.envFile,
	}
}

// subprocessEnv starts from the calling process's own environment (Docker
// CLI needs PATH, USERPROFILE/HOME, SYSTEMROOT, TEMP, and similar to
// function on Windows) and removes exactly the names in
// sensitiveEnvDenylist, then sets the two generated credentials
// explicitly -- belt-and-suspenders alongside the explicit --env-file,
// which is the primary, empirically confirmed defense.
func (c *Compose) subprocessEnv() []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+2)
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if slices.Contains(sensitiveEnvDenylist, name) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "API_TOKEN="+c.apiToken, "POSTGRES_PASSWORD="+c.pgPassword)
	return out
}

// run executes `docker <args...>` (args should start with "compose" for
// every compose subcommand) with a bounded timeout, the curated
// subprocess environment, and cwd set to the repository root (compose
// file paths are absolute already; the build context in docker-compose.yml
// is relative to the compose file's own directory, not cwd, so this is for
// consistency, not correctness). Returns combined context: any non-zero
// exit or spawn failure is wrapped as an *EnvironmentError, since a
// docker/compose invocation failing is an environment condition, never a
// product/AC finding.
func (c *Compose) run(ctx context.Context, timeout time.Duration, op string, args ...string) (stdout, stderr string, err error) {
	runCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, "docker", args...)
	cmd.Dir = c.repoRoot
	cmd.Env = c.subprocessEnv()

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout, stderr = outBuf.String(), errBuf.String()
	if runErr != nil {
		return stdout, stderr, envErrorf(op, "docker %s: %w (stderr: %s)", strings.Join(args, " "), runErr, strings.TrimSpace(stderr))
	}
	return stdout, stderr, nil
}

const composeCmdTimeout = 3 * time.Minute

// PreflightClean removes any leftover cnsdp-verify project from a
// previously interrupted run, scoped strictly to projectName -- it can
// never affect a differently-named project. Errors are tolerated (there
// may be nothing to clean up) but logged by the caller if non-nil.
func (c *Compose) PreflightClean(ctx context.Context) error {
	args := append(c.baseArgs(), "down", "--volumes", "--remove-orphans")
	_, _, err := c.run(ctx, composeCmdTimeout, "preflight cleanup", args...)
	return err // caller decides whether "nothing to clean up" is worth logging
}

// Up builds (if needed) and starts the isolated stack in detached mode.
func (c *Compose) Up(ctx context.Context) error {
	args := append(c.baseArgs(), "up", "-d", "--build", "--wait", "--wait-timeout", "180")
	_, _, err := c.run(ctx, 5*time.Minute, "compose up", args...)
	return err
}

// Down stops and removes the isolated stack. removeVolumes also drops the
// named Postgres volume, so a subsequent run starts from an empty
// database rather than accumulated state from a prior run.
func (c *Compose) Down(ctx context.Context, removeVolumes bool) error {
	args := append(c.baseArgs(), "down", "--remove-orphans")
	if removeVolumes {
		args = append(args, "--volumes")
	}
	_, _, err := c.run(ctx, composeCmdTimeout, "compose down", args...)
	return err
}

// Kill sends SIGKILL to service's container -- Compose's own default
// signal for `kill`, deliberately bypassing cmd/platform's SIGTERM-driven
// graceful shutdown path entirely, since AC-019 specifically calls for an
// abrupt, non-graceful interruption. The base compose file's own
// `restart: unless-stopped` policy (unchanged by this harness) is what
// brings the container back -- Kill only ever removes a process, it never
// itself starts anything.
func (c *Compose) Kill(ctx context.Context, service string) error {
	args := append(c.baseArgs(), "kill", service)
	_, _, err := c.run(ctx, 30*time.Second, "compose kill "+service, args...)
	return err
}

// StartService issues `docker compose start service` -- a lightweight
// request that the container (which must already exist, e.g. left
// stopped by Kill) run again, with no rebuild and no health-wait
// blocking. Deliberately distinct from Up (which additionally builds and
// blocks on --wait-timeout): a recovery-procedure revival step must never
// have its own success/failure conflated with a bounded internal
// wait-timeout unrelated to AC-019's own 15-minute RTO budget, which the
// caller (RunRecoveryPhase's waitReady polling loop) measures and
// verdicts independently.
func (c *Compose) StartService(ctx context.Context, service string) error {
	args := append(c.baseArgs(), "start", service)
	_, _, err := c.run(ctx, 30*time.Second, "compose start "+service, args...)
	return err
}

// Port discovers the actual host port Docker assigned to service's
// containerPort -- never hardcoded or guessed, since docker-compose.verify.yml
// deliberately requests a dynamically assigned port.
func (c *Compose) Port(ctx context.Context, service string, containerPort int) (string, error) {
	args := append(c.baseArgs(), "port", service, strconv.Itoa(containerPort))
	out, _, err := c.run(ctx, 15*time.Second, "compose port "+service, args...)
	if err != nil {
		return "", err
	}
	// Expected form: "0.0.0.0:54321" (possibly with a trailing newline, and
	// possibly more than one line if bound on multiple interfaces --
	// docker-compose.verify.yml binds 127.0.0.1 only, so exactly one line
	// is expected; using the last field after ':' from the first line
	// either way.
	line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	idx := strings.LastIndex(line, ":")
	if idx < 0 || idx == len(line)-1 {
		return "", envErrorf("compose port "+service, "unexpected `docker compose port` output %q", out)
	}
	port := line[idx+1:]
	if _, err := strconv.Atoi(port); err != nil {
		return "", envErrorf("compose port "+service, "unexpected `docker compose port` output %q", out)
	}
	return port, nil
}

// ContainerID resolves service's running container id, for Kill-target
// verification and `docker stats`.
func (c *Compose) ContainerID(ctx context.Context, service string) (string, error) {
	args := append(c.baseArgs(), "ps", "-q", service)
	out, _, err := c.run(ctx, 15*time.Second, "compose ps "+service, args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", envErrorf("compose ps "+service, "no running container found for service %q", service)
	}
	return id, nil
}

// ConfigYAML renders the effective merged configuration -- the exact
// artifact the isolation invariants (IsolationReport) are checked against,
// and the same command an operator would run by hand to inspect it.
func (c *Compose) ConfigYAML(ctx context.Context) (string, error) {
	args := append(c.baseArgs(), "config")
	out, _, err := c.run(ctx, 30*time.Second, "compose config", args...)
	return out, err
}

// composeConfigPort mirrors the subset of `docker compose config`'s
// rendered port-mapping shape this harness needs to verify.
type composeConfigPort struct {
	HostIP    string `yaml:"host_ip"`
	Target    int    `yaml:"target"`
	Published string `yaml:"published"`
}

type composeConfigService struct {
	Ports []composeConfigPort `yaml:"ports"`
}

type composeConfigDoc struct {
	Name     string                          `yaml:"name"`
	Services map[string]composeConfigService `yaml:"services"`
}

// IsolationReport is the harness's own machine-checked proof of the
// isolation invariants the design requires -- never assumed, always
// verified against the actual rendered config for this run.
type IsolationReport struct {
	ProjectName           string   `json:"projectName"`
	AppPortCount          int      `json:"appPortCount"`
	AppHostIP             string   `json:"appHostIp"`
	AppPortDynamic        bool     `json:"appPortDynamic"`
	DevelopmentPortAbsent bool     `json:"developmentPortAbsent"` // no published:"8080" entry anywhere for app
	PostgresPortCount     int      `json:"postgresPortCount"`
	PostgresHostIP        string   `json:"postgresHostIp"`
	PostgresPortDynamic   bool     `json:"postgresPortDynamic"`
	Satisfied             bool     `json:"satisfied"`
	Problems              []string `json:"problems,omitempty"`
	RawConfig             string   `json:"-"` // kept out of the JSON report body; attached as a separate artifact
}

// ValidateIsolation renders the merged config and checks every invariant
// the design requires: the base file's fixed app host port is absent,
// exactly one app port publication exists on a dynamic port, and Postgres
// (published here as the harness's one deliberate, documented exception)
// is also on a dynamic, 127.0.0.1-scoped port. The harness must not
// proceed to Up if this reports Satisfied == false.
func (c *Compose) ValidateIsolation(ctx context.Context) (*IsolationReport, error) {
	raw, err := c.ConfigYAML(ctx)
	if err != nil {
		return nil, err
	}
	return validateIsolationFromConfig(raw)
}

// validateIsolationFromConfig is the pure parsing/assertion half of
// ValidateIsolation, split out so it is unit-testable against a captured
// `docker compose config` transcript without requiring Docker itself --
// see compose_test.go, which uses the actual output captured while
// designing docker-compose.verify.yml.
func validateIsolationFromConfig(raw string) (*IsolationReport, error) {
	var doc composeConfigDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, envErrorf("parse compose config", "unmarshal rendered config: %w", err)
	}

	report := &IsolationReport{ProjectName: doc.Name, RawConfig: raw}

	app, ok := doc.Services["app"]
	if !ok {
		report.Problems = append(report.Problems, `service "app" absent from rendered config`)
	} else {
		report.AppPortCount = len(app.Ports)
		if len(app.Ports) != 1 {
			report.Problems = append(report.Problems, fmt.Sprintf("app has %d port publications, want exactly 1", len(app.Ports)))
		}
		for _, p := range app.Ports {
			report.AppHostIP = p.HostIP
			report.AppPortDynamic = p.Published == "" || p.Published == "0"
			if p.HostIP != "127.0.0.1" {
				report.Problems = append(report.Problems, fmt.Sprintf("app port host_ip = %q, want 127.0.0.1", p.HostIP))
			}
			if p.Target != 8080 {
				report.Problems = append(report.Problems, fmt.Sprintf("app port target = %d, want 8080", p.Target))
			}
			if !report.AppPortDynamic {
				report.Problems = append(report.Problems, fmt.Sprintf("app port published = %q, want empty/dynamic", p.Published))
			}
			if p.Published == "8080" {
				report.Problems = append(report.Problems, "app port published = \"8080\" -- the normal development host port is present in the verification config")
			}
		}
	}
	// The base file's fixed port never appears anywhere in the raw text
	// either, as a second, independent check beyond the structured
	// per-service parse above.
	report.DevelopmentPortAbsent = !strings.Contains(raw, `published: "8080"`) && !strings.Contains(raw, "127.0.0.1:8080:8080")
	if !report.DevelopmentPortAbsent {
		report.Problems = append(report.Problems, "the literal base-file development port (8080) appears in the rendered verification config")
	}

	pg, ok := doc.Services["postgres"]
	if !ok {
		report.Problems = append(report.Problems, `service "postgres" absent from rendered config`)
	} else {
		report.PostgresPortCount = len(pg.Ports)
		if len(pg.Ports) != 1 {
			report.Problems = append(report.Problems, fmt.Sprintf("postgres has %d port publications, want exactly 1 (the harness's one documented internal-only exception)", len(pg.Ports)))
		}
		for _, p := range pg.Ports {
			report.PostgresHostIP = p.HostIP
			report.PostgresPortDynamic = p.Published == "" || p.Published == "0"
			if p.HostIP != "127.0.0.1" {
				report.Problems = append(report.Problems, fmt.Sprintf("postgres port host_ip = %q, want 127.0.0.1", p.HostIP))
			}
			if p.Target != 5432 {
				report.Problems = append(report.Problems, fmt.Sprintf("postgres port target = %d, want 5432", p.Target))
			}
			if !report.PostgresPortDynamic {
				report.Problems = append(report.Problems, fmt.Sprintf("postgres port published = %q, want empty/dynamic", p.Published))
			}
		}
	}

	if doc.Name != projectName {
		report.Problems = append(report.Problems, fmt.Sprintf("rendered project name = %q, want %q", doc.Name, projectName))
	}

	report.Satisfied = len(report.Problems) == 0
	return report, nil
}

// dockerStats is the subset of `docker stats --format {{json .}}`'s output
// this harness parses.
type dockerStats struct {
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
}

// Stats samples one instantaneous CPU/memory reading for containerID.
// Evidence collection only (AC-023) -- no threshold is applied here or by
// any caller; see resource_sampling.go.
func (c *Compose) Stats(ctx context.Context, containerID string) (cpuPercent float64, memBytes uint64, err error) {
	statsCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(statsCtx, "docker", "stats", "--no-stream", "--format", "{{json .}}", containerID)
	cmd.Env = c.subprocessEnv()
	out, runErr := cmd.Output()
	if runErr != nil {
		return 0, 0, wrapEnvErr("docker stats", runErr)
	}

	var s dockerStats
	if err := json.Unmarshal(bytes.TrimSpace(out), &s); err != nil {
		return 0, 0, wrapEnvErr("docker stats", err)
	}

	cpuPercent, err = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s.CPUPerc), "%"), 64)
	if err != nil {
		return 0, 0, wrapEnvErr("docker stats", fmt.Errorf("parse CPUPerc %q: %w", s.CPUPerc, err))
	}

	memPart, _, _ := strings.Cut(s.MemUsage, "/")
	memBytes, err = parseDockerMemory(strings.TrimSpace(memPart))
	if err != nil {
		return 0, 0, wrapEnvErr("docker stats", fmt.Errorf("parse MemUsage %q: %w", s.MemUsage, err))
	}
	return cpuPercent, memBytes, nil
}
