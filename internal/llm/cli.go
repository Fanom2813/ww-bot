package llm

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// cliSandboxDir returns (and lazily creates) an app-owned empty directory used
// as the working directory for every CLI invocation. Doing this isolates the
// subprocess from whatever folder the user launched WWAssist from:
//
//   - Gemini CLI's "trust" check is meant to block silent loading of
//     .gemini/settings.json, .toml commands, .env, MCP configs, and project
//     memory from an untrusted workspace. Pointing it at an empty app-owned
//     directory makes the GEMINI_CLI_TRUST_WORKSPACE=true bypass safe, because
//     there is nothing in that workspace to load.
//   - Claude Code and Codex have similar config-file extensibility; the same
//     isolation keeps them from picking up anything in CWD.
var (
	sandboxOnce sync.Once
	sandboxDir  string
)

func cliSandboxDir() string {
	sandboxOnce.Do(func() {
		base, err := os.UserConfigDir()
		if err != nil {
			// Fall back to a temp dir — still empty and app-owned for this run.
			tmp, terr := os.MkdirTemp("", "wwbot-cli-*")
			if terr != nil {
				return
			}
			sandboxDir = tmp
			return
		}
		d := filepath.Join(base, "ww-bot", "cli-sandbox")
		if err := os.MkdirAll(d, 0o700); err != nil {
			return
		}
		sandboxDir = d
	})
	return sandboxDir
}

// CLIAgent drives an installed coding-agent CLI in non-interactive mode, using
// the user's own subscription (no API key). The exact flags differ per tool and
// per version; the known-agent constructors below encode sensible defaults that
// can be overridden in settings.
type CLIAgent struct {
	name           string
	bin            string
	args           []string
	promptViaStdin bool     // if false, the prompt is appended as the final arg
	extraEnv       []string // extra KEY=VALUE entries appended to os.Environ()
}

// NewCLIAgent builds a CLI provider. If promptViaStdin is true the prompt is
// piped to the process's stdin; otherwise it is appended as the last argument.
func NewCLIAgent(name, bin string, args []string, promptViaStdin bool) *CLIAgent {
	return &CLIAgent{name: name, bin: bin, args: args, promptViaStdin: promptViaStdin}
}

// ClaudeCLI runs Claude Code in print mode with --permission-mode dontAsk.
// Per the official docs, dontAsk *silently denies* any tool not explicitly
// allow-listed and never prompts — exactly what we want for a chat reply that
// should never take OS-level actions. No latency cost.
func ClaudeCLI() *CLIAgent {
	return NewCLIAgent("claude-code", "claude", []string{"-p", "--permission-mode", "dontAsk"}, true)
}

// CodexCLI runs Codex in exec mode using the configuration the Codex docs
// explicitly recommend for non-interactive runs:
//
//   - `-c approval_policy=never` — the docs literally say "use never for
//     non-interactive runs" (on-failure is deprecated).
//   - `--sandbox read-only` — pin the sandbox profile so model-generated shell
//     commands cannot write or reach the network even if the default changes.
//   - `--ignore-user-config` — do not load ~/.codex/config.toml, which can
//     define MCP servers, hooks, and rule overrides that would otherwise be
//     pulled into our subprocess.
func CodexCLI() *CLIAgent {
	return NewCLIAgent("codex", "codex",
		[]string{"exec", "-c", "approval_policy=never", "--sandbox", "read-only", "--ignore-user-config"},
		true)
}

// GeminiCLI runs Gemini in headless mode locked to "plan" (read-only) approval
// mode so any tool the model tries to invoke is denied. The Gemini CLI also
// refuses to run in an "untrusted" workspace in headless mode; we set
// GEMINI_CLI_TRUST_WORKSPACE=true (safe because cmd.Dir is an empty,
// app-owned sandbox that has no .gemini/.env/.toml/MCP config to load).
// GeminiCLI runs Gemini in headless -p mode with the prompt piped via stdin.
//
// stdin (not a positional arg) is important on Windows: the npm install ships
// a `gemini.cmd` shim, and cmd.exe argument handling mangles prompts that
// contain quotes, newlines, or special characters (and Go's escaping for
// .bat/.cmd was tightened further by CVE-2024-24576). Stdin sidesteps all of
// that. The Gemini docs confirm `-p` accepts content on stdin in addition to
// the flag value.
//
// On --approval-mode: the choices are {default, auto_edit, yolo, plan}. Only
// `plan` blocks tools, but it slows replies (deliberation). The headless
// default — prompt-then-fail with no terminal attached — gives the same "no
// tool execution" guarantee with no latency cost, so we leave it unset.
// GEMINI_CLI_TRUST_WORKSPACE=true is safe because cmd.Dir is an empty,
// app-owned sandbox with no project config to load.
func GeminiCLI() *CLIAgent {
	a := NewCLIAgent("gemini-cli", "gemini", []string{"-p"}, true)
	a.extraEnv = []string{"GEMINI_CLI_TRUST_WORKSPACE=true"}
	return a
}

func (c *CLIAgent) Name() string { return c.name }

// Available reports whether the CLI binary is on PATH.
func (c *CLIAgent) Available(_ context.Context) bool {
	_, err := exec.LookPath(c.bin)
	return err == nil
}

// Complete runs the CLI with the flattened prompt and returns its stdout.
func (c *CLIAgent) Complete(ctx context.Context, req Request) (string, error) {
	prompt := flatten(req)

	args := append([]string{}, c.args...)
	if !c.promptViaStdin {
		args = append(args, prompt)
	}

	log.Printf("[cli] %s: running %q args=%v stdin=%v cwd=%q",
		c.name, c.bin, args, c.promptViaStdin, cliSandboxDir())
	cmd := exec.CommandContext(ctx, c.bin, args...)
	// Isolate the subprocess workspace so project-local config files
	// (.gemini/, .env, .toml commands, MCP configs, project memory) from
	// wherever the user launched WWAssist cannot influence the CLI.
	if sb := cliSandboxDir(); sb != "" {
		cmd.Dir = sb
	}
	if len(c.extraEnv) > 0 {
		cmd.Env = append(os.Environ(), c.extraEnv...)
	}
	hideConsoleWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if c.promptViaStdin {
		cmd.Stdin = strings.NewReader(prompt)
	}

	if err := cmd.Run(); err != nil {
		// Surface the actual stderr (truncated) in the app log so failures
		// are diagnosable from ww-bot.log without enabling debug builds.
		log.Printf("[cli] %s: run failed: %v; stderr: %s", c.name, err, truncate(stderr.String(), 800))
		return "", fmt.Errorf("llm %s: run: %w: %s", c.name, err, truncate(stderr.String(), 300))
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		// Exited 0 but said nothing — treat as failure so the registry falls
		// through, and surface stderr so the command can be fixed.
		return "", fmt.Errorf("llm %s: empty output (check the command/args/prompt mode); stderr: %s",
			c.name, truncate(stderr.String(), 400))
	}
	return out, nil
}

// flatten renders a Request into a single prompt string for CLIs that take one.
func flatten(req Request) string {
	var b strings.Builder
	if req.System != "" {
		b.WriteString(req.System)
		b.WriteString("\n\n")
	}
	for _, m := range req.Messages {
		switch m.Role {
		case Assistant:
			b.WriteString("Assistant: ")
		case System:
			b.WriteString("System: ")
		default:
			b.WriteString("User: ")
		}
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
