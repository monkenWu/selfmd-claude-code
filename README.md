# SelfMD - Auto Documentation Generator for Claude Code CLI

<p align="center">
  <img src="logo.png" alt="SelfMD Logo" width="300">
</p>

**Automatically generate structured, high-quality technical documentation for any codebase — powered by [Claude Code CLI](https://code.claude.com/docs/en/overview).**

[繁體中文](./README_zh-TW.md)

---

## Why SelfMD?

Writing and maintaining technical documentation is painful. It falls out of date the moment code changes, and no one wants to write it. SelfMD solves this by letting AI read your code and generate comprehensive Wiki-style documentation automatically.

- **Lightweight** — A single binary. No server, no database, no Docker. Just download and run.
- **Built on Claude Code CLI** — Not stealing credentials, but using your existing Claude Pro / Max subscription by commanding your Claude Code CLI. No separate API key or billing required.
- **High-Quality Output** — Generates structured Markdown with Mermaid diagrams, source code annotations, and cross-linked references.
- **Incremental Updates** — Detects code changes via `git diff` and only regenerates affected pages, saving time and cost.
- **Multi-Language** — Supports 11 languages out of the box. Generate docs in one language, then translate to others with a single command.
- **Built-in Viewer** — Ships with a zero-dependency static HTML viewer. Open `index.html` in any browser — no web server needed.
- **Fault-Tolerant** — Failed pages get placeholder content instead of aborting the entire run. Re-run to retry only the failed pages.
- **Parallel Generation** — Configurable concurrency for faster documentation generation on large projects.

![SelfMD Demo Docs](https://monkenwu.github.io/selfmd-claude-code)

---

## Quick Start

### 1. Prerequisites

- [Claude Code CLI](https://code.claude.com/docs/en/overview) installed and available in your `PATH`

### 2. Download

Download the latest binary for your platform from the [Releases](https://github.com/monkenWu/selfmd-claude-code/releases) page.

| Platform | Architecture | File |
|----------|-------------|------|
| macOS | Apple Silicon (arm64) | `selfmd-macos-arm64` |
| macOS | Intel (amd64) | `selfmd-macos-amd64` |
| Linux | arm64 | `selfmd-linux-arm64` |
| Linux | amd64 | `selfmd-linux-amd64` |
| Windows | arm64 | `selfmd-windows-arm64.exe` |
| Windows | amd64 | `selfmd-windows-amd64.exe` |

```bash
# macOS / Linux: make it executable, rename, and move to PATH
chmod +x selfmd-macos-arm64
sudo mv selfmd-macos-arm64 /usr/local/bin/selfmd
```

```powershell
# Windows (PowerShell): create a directory, move the binary, and add to PATH
mkdir "$env:USERPROFILE\selfmd"
Rename-Item selfmd-windows-amd64.exe selfmd.exe
Move-Item selfmd.exe "$env:USERPROFILE\selfmd\selfmd.exe"
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\selfmd", "User")
```

```cmd
:: Windows (CMD): create a directory, move the binary, and add to PATH
mkdir "%USERPROFILE%\selfmd"
ren selfmd-windows-amd64.exe selfmd.exe
move selfmd.exe "%USERPROFILE%\selfmd\selfmd.exe"
setx Path "%Path%;%USERPROFILE%\selfmd"
```

After this, you can simply run `selfmd` from anywhere.

### 3. Initialize

Navigate to your project root and run:

```bash
selfmd init
```

This scans your project, detects the project type (Go, Node.js, Python, Rust, Java, Ruby, etc.), and generates a `selfmd.yaml` configuration file.

### 4. Generate Documentation

```bash
selfmd generate
```

That's it. The tool will:

1. Scan your project structure and source code
2. Call Claude Code CLI to plan a documentation catalog
3. Generate each documentation page in parallel
4. Output a complete static documentation site

When finished, open `.doc-build/index.html` in your browser to view the docs.

---

## Commands

| Command | Description |
|---------|-------------|
| `selfmd init` | Detect project type and generate `selfmd.yaml` |
| `selfmd generate` | Run the full documentation generation pipeline |
| `selfmd update` | Incrementally update docs based on `git diff` |
| `selfmd translate` | Translate documentation into secondary languages |

### Common Flags

```bash
# Preview which files will be scanned, without calling Claude
selfmd generate --dry-run

# Force-clean the output directory and regenerate everything
selfmd generate --clean

# Increase concurrency for faster generation
selfmd generate --concurrency 5
```

---

## Configuration

`selfmd init` generates a `selfmd.yaml` file at your project root:

```yaml
project:
  name: "My Project"
  type: "backend"         # backend | frontend | fullstack | library | cli
  description: ""
targets:
  include:
    - "src/**"
    - "cmd/**"
    - "internal/**"
  exclude:
    - "vendor/**"
    - "node_modules/**"
  entry_points:
    - "main.go"

output:
  dir: ".doc-build"
  language: "zh-TW"                     # Primary documentation language
  secondary_languages: ["en-US"]        # Additional languages for translation
  clean_before_generate: false

claude:
  model: "sonnet"
  max_concurrent: 3          # Parallel Claude CLI calls
  timeout_seconds: 1800
  max_retries: 2
  allowed_tools:
    - "Read"
    - "Glob"
    - "Grep"

git:
  enabled: true
  base_branch: "main"
```

### Supported Languages

`zh-TW` `zh-CN` `en-US` `ja-JP` `ko-KR` `fr-FR` `de-DE` `es-ES` `pt-BR` `th-TH` `vi-VN`

---

## How It Works

```
selfmd generate
       │
       ▼
┌─────────────────┐
│  Phase 1: Scan  │  Traverse project files based on include/exclude rules
└───────┬─────────┘
        ▼
┌─────────────────────────┐
│  Phase 2: Plan Catalog  │  Claude analyzes the codebase and creates a documentation outline
└───────┬─────────────────┘
        ▼
┌──────────────────────────────┐
│  Phase 3: Generate Pages     │  Each page is generated in parallel via Claude CLI
└───────┬──────────────────────┘
        ▼
┌─────────────────────────────────┐
│  Phase 4: Index & Navigation    │  Generate index, sidebar, and static HTML viewer
└─────────────────────────────────┘
```

### Incremental Updates

After the initial generation, use `selfmd update` to keep docs in sync:

```bash
# Only regenerate pages affected by recent code changes
selfmd update
```

The tool compares your current commit against the last recorded commit, identifies changed files, and regenerates only the documentation pages that reference those files.

---

## Output Structure

```
.doc-build/
├── index.html            ← Open this to browse the docs
├── index.md
├── _sidebar.md
├── _catalog.json         ← Catalog cache (for incremental updates)
├── _last_commit          ← Git commit record
├── _data.js              ← Data bundle for the static viewer
├── overview/
│   └── index.md
├── core-modules/
│   ├── index.md
│   └── authentication/
│       └── index.md
└── ...
```

---

## Building from Source

Requires Go 1.25+.

```bash
go build -o selfmd .
```

---

## License

MIT

---

## Contributing

Issues and pull requests are welcome! Please open an issue first to discuss what you'd like to change.
