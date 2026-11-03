# tharness

[![release](https://img.shields.io/github/v/tag/Amirhossein-Mirzaei23/Tharness?sort=semver&label=release)](https://github.com/Amirhossein-Mirzaei23/Tharness/releases)
[![go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![license](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A single-binary coding-agent CLI harness. Configure it with plain API keys (no OAuth), chat with it in the terminal, let it run tools on your codebase, and resume past sessions — all from one static binary for Linux, macOS, and Windows.

**Sections:** [Install](#install) · [Quick start](#quick-start) · [Providers](#providers) · [Configuration](#configuration) · [Commands](#commands) · [Permissions](#permissions) · [Themes](#themes) · [How it works](#how-it-works) · [Development](#development)

## What it does

tharness runs a coding agent in your terminal. You give it a prompt; it streams an answer, calls tools (read, write, edit, bash, glob, grep, todo, browser) against your workspace, and saves every session as a JSONL transcript you can resume later.

It talks to any OpenAI-compatible chat-completions endpoint: Z.ai, OpenAI, Anthropic, Google Gemini, OpenRouter, Groq, Mistral, DeepSeek, xAI, Together AI, a local Ollama, or your own endpoint. Providers are configured through `tharness setup` with an API key — the harness never performs OAuth flows.

| | |
|---|---|
| **Setup** | Interactive wizard or fully flagged one-liner: provider, API key, browser, theme |
| **Modes** | Interactive chat TUI, one-shot `-p` prompts, session resume |
| **Safety** | Per-tool permission policies (`auto` / `ask` / `deny`) for bash, write, edit, browser |
| **Theming** | 5 TUI color palettes with live preview in the wizard |
| **Storage** | `~/.tharness/` — config (chmod 0600), session transcripts, todo state |

## Install

**Linux (Ubuntu/Debian/...)**

```bash
curl -fsSL https://raw.githubusercontent.com/Amirhossein-Mirzaei23/Tharness/main/install.sh | bash
```

**macOS (Apple Silicon)**

```bash
curl -fsSL https://raw.githubusercontent.com/Amirhossein-Mirzaei23/Tharness/main/install-macos.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/Amirhossein-Mirzaei23/Tharness/main/install.ps1 | iex
```

Each script downloads the release asset, verifies its sha256 against `checksums.txt`, and installs `tharness` on your PATH. To pin a version: `THARNESS_VERSION=0.0.1 ./install.sh`. To choose a directory: `PREFIX=~/.local/bin ./install.sh`.

Manual download from [releases/tag/v0.0.1](https://github.com/Amirhossein-Mirzaei23/Tharness/releases/tag/v0.0.1):

| Asset | OS | Arch |
|---|---|---|
| `tharness_0.0.1_linux_amd64.tar.gz` | Linux | x86_64 |
| `tharness_0.0.1_darwin_arm64.tar.gz` | macOS | Apple Silicon |
| `tharness_0.0.1_windows_amd64.zip` | Windows | x86_64 |

**Build from source:** Go 1.27+, no cgo.

```bash
git clone https://github.com/Amirhossein-Mirzaei23/Tharness.git
cd Tharness
go build -o tharness ./cmd/tharness
```

## Quick start

**1. Configure** — pick a provider, paste its API key, choose a browser and a theme:

```bash
tharness setup
```

The wizard asks, in order: provider → API key (input masked; skipped for Ollama) → base URL (prefilled with the provider default) → model (prefilled) → browser (auto-detected candidates) → theme (live preview) → confirm. The config is written to `~/.tharness/config.yaml` with `0600` permissions because it stores keys.

Or skip the wizard entirely:

```bash
tharness setup --provider openai --api-key sk-... --browser firefox --theme dracula
```

**2. Chat interactively:**

```bash
tharness
```

Type a message, press `Enter` to send. While the agent runs you'll see a spinner, streamed text, and tool-call cards; `Ctrl+C` cancels the run (or quits when idle).

**3. Or run one-shot prompts:**

```bash
tharness -p "summarize this repo"
```

One-shot runs read the same config. With no API key configured yet, the provider rejects the request (401 auth error) — run `tharness setup` first. A `THARNESS_API_KEY` (and optionally `THARNESS_BASE_URL`) environment variable overrides the config file.

**4. Sessions** are stored automatically under `~/.tharness/sessions/<id>.jsonl`. List them and resume:

```bash
tharness sessions list
tharness -p "continue where we left off" --resume <session-id>
```

## Providers

Every provider works through its OpenAI-compatible endpoint with an API key. Keys are stored per provider, so you can configure several and switch with one command.

| Provider | Name (`--provider`) | Key env var | Default model |
|---|---|---|---|
| Z.ai (GLM) | `zai` | `ZAI_API_KEY` | `glm-5.3-flash` |
| OpenAI | `openai` | `OPENAI_API_KEY` | `gpt-4o-mini` |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` | `claude-sonnet-4-20250514` |
| Google Gemini | `gemini` | `GEMINI_API_KEY` | `gemini-2.5-flash` |
| OpenRouter | `openrouter` | `OPENROUTER_API_KEY` | `openai/gpt-4o-mini` |
| Groq | `groq` | `GROQ_API_KEY` | `llama-3.3-70b-versatile` |
| Mistral AI | `mistral` | `MISTRAL_API_KEY` | `mistral-large-latest` |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` | `deepseek-chat` |
| xAI (Grok) | `xai` | `XAI_API_KEY` | `grok-2-latest` |
| Together AI | `together` | `TOGETHER_API_KEY` | `meta-llama/Llama-3.3-70B-Instruct-Turbo` |
| Ollama (local) | `ollama` | — no key needed | chosen per install |
| Custom endpoint | `custom` | any (set with `--api-key`) | chosen per install |

`custom` requires `--base-url` and works with any OpenAI-compatible server (vLLM, LiteLLM, LM Studio, ...). Switch providers non-interactively:

```bash
tharness setup --provider groq --api-key gsk_...
tharness setup --provider ollama   # keyless, http://localhost:11434/v1
```

## Configuration

Everything lives in `~/.tharness/config.yaml` (override location with the global `--config` flag). Example of a real generated file:

```yaml
browser:
    command: firefox
model: ""
permissions:
    bash: auto
    browser: auto
    edit: auto
    write: auto
provider: openai
providers:
    openai:
        api_key: sk-YOUR-KEY
        model: gpt-4o-mini
theme: dracula
```

| Field | Meaning |
|---|---|
| `provider` | Which provider entry is active |
| `providers.<name>.api_key` | API key for that provider |
| `providers.<name>.base_url` | Endpoint override (provider defaults built in) |
| `providers.<name>.model` | Model pinned for that provider |
| `browser.command` | Browser executable; empty = auto-detect at use time |
| `theme` | TUI palette (`dark`, `light`, `dracula`, `nord`, `gruvbox`) |
| `permissions.*` | Per-tool policy: `auto`, `ask`, or `deny` |
| `model`, `base_url`, `api_key_env` | Legacy flat fields, still honored for provider resolution |

Resolution precedence, highest first:

1. `THARNESS_BASE_URL` / `THARNESS_API_KEY` environment variables
2. The stored `providers.<provider>` entry
3. The provider's own well-known env var (e.g. `OPENAI_API_KEY`)
4. Legacy flat config fields (`base_url`, `api_key_env`)
5. Provider defaults

`THARNESS_DATA_DIR` moves the data directory (sessions, config dir) somewhere else.

## Commands

| Command | What it does |
|---|---|
| `tharness` | Interactive chat TUI (default when no arguments) |
| `tharness -p "prompt"` | One-shot run, prints the final answer |
| `tharness -p "..." --resume <id>` | One-shot run seeded with a stored session |
| `tharness chat` | Same TUI, explicit |
| `tharness run -p "prompt"` | Same one-shot, explicit |
| `tharness sessions list` | List stored session ids |
| `tharness setup` | Provider keys, browser, theme (wizard or flags) |
| `tharness --version` | Print version |

Global flag `--config <path>` points any command at a different config file.

## Permissions

Tools that touch your system — `bash`, `write`, `edit`, `browser` — go through the permission gate. Each has a policy in the config:

- `auto` — run without asking (the default)
- `ask` — require interactive approval; in the chat TUI the run pauses for your decision
- `deny` — never run

One-shot (`-p`) runs are non-interactive: an `ask` policy denies the call with a clear error rather than hanging. Unknown tools are not gated.

## Themes

Pick during `tharness setup` (arrow keys move, the wizard renders a live preview) or set `theme` in the config:

| Theme | Palette |
|---|---|
| `dark` | ANSI colors tuned for dark terminals |
| `light` | ANSI colors tuned for light terminals |
| `dracula` | Dracula-inspired purples and pinks |
| `nord` | Nord frost blues and muted grays |
| `gruvbox` | Gruvbox warm earth tones |

All palettes use ANSI/256-color codes and degrade on plain terminals.

## How it works

```
you › prompt
  └─> config resolve (env > file > provider defaults)
        └─> agent loop (internal/agent)
              ├─> provider stream (OpenAI-compatible /chat/completions, SSE)
              └─> tool calls → permission gate → tools registry
                    (read / write / edit / bash / glob / grep / todo / browser)
  └─> transcript appended to ~/.tharness/sessions/<id>.jsonl
```

The loop accumulates streamed deltas, merges tool-call fragments, dispatches tool executions, and feeds results back to the model until it produces a final answer. Resuming replays a stored transcript as message history.

## Platform notes

- **Linux (Ubuntu)** — fully exercised; auto-detects `google-chrome`, `chromium`, `brave`, `edge`, `firefox`, `opera` (and `$BROWSER`).
- **macOS** — Apple Silicon build; Intel binaries are not published for v0.0.1. Auto-detect uses `open` (your default browser).
- **Windows** — install lands in `%LOCALAPPDATA%\Programs\tharness`; use Windows Terminal (the TUI relies on ConPTY). Auto-detect uses `rundll32` (default browser). The bash tool runs commands through `cmd /c`; on timeout only the shell process is killed, not grandchildren.
- Timeouts kill the whole process group on Unix (`SIGKILL` to `-pid`), bounded by the tool's `WaitDelay`.

## Development

```bash
go vet ./...
go test ./...        # 10 packages
go build -o tharness ./cmd/tharness
```

Cross-compile (no cgo anywhere):

```bash
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o dist/tharness-darwin-arm64  ./cmd/tharness
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/tharness-windows-amd64.exe ./cmd/tharness
```

Release procedure: bump `version` in `internal/cli/root.go`, commit, tag `vX.Y.Z`, build the three assets into `dist/`, generate `checksums.txt`, then `gh release create vX.Y.Z dist/*`. The install scripts pick up any version via `THARNESS_VERSION`.

## Limitations

- API keys only, by design — no OAuth device flows.
- `browser` tool opens URLs; it cannot read page content back.
- Windows timeout kill is best-effort (shell process only).
- Ollama and `custom` providers ship without a default model; type one during setup.

## License

[MIT](LICENSE) © 2026 Amirhossein Mirzaei
