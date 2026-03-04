# SelfMD - Claude Code CLI 自動文件產生器

<p align="center">
  <img src="logo.png" alt="SelfMD Logo" width="300">
</p>

**透過 [Claude Code CLI](https://code.claude.com/docs/en/overview) 自動為任何程式碼庫產生結構化、高品質的技術文件。**

[English](./README.md)

---

## 為什麼選擇 SelfMD？

撰寫與維護技術文件是一件痛苦的事。程式碼一改，文件就過時了，而且沒有人想寫。SelfMD 讓 AI 閱讀你的程式碼，自動產生完整的 Wiki 風格技術文件。

- **輕量** — 單一執行檔，無需伺服器、資料庫或 Docker。下載即用。
- **基於 Claude Code CLI** — 並非盜用憑證，而是藉由指揮你的 Claude Code CLI，使用你的 Claude Pro / Max 訂閱，無需額外的 API Key 或計費。
- **高品質輸出** — 產生結構化 Markdown 文件，包含 Mermaid 圖表、原始碼標註與交叉引用連結。
- **增量更新** — 透過 `git diff` 偵測程式碼變更，僅重新產生受影響的頁面，節省時間與成本。
- **多國語系** — 內建支援 11 種語言。先產生一種語言的文件，再用一條指令翻譯成其他語言。
- **內建閱讀器** — 附帶零依賴的靜態 HTML 閱讀器，用瀏覽器開啟 `index.html` 即可瀏覽，無需架設網頁伺服器。
- **容錯機制** — 產生失敗的頁面會寫入佔位內容而非中斷整個流程。重新執行即可自動重試失敗的頁面。
- **並行產生** — 可設定並行度，加速大型專案的文件產生速度。

---

## 快速開始

### 1. 前置需求

- 已安裝 [Claude Code CLI](https://code.claude.com/docs/zh-TW/overview) 並可在終端機中直接呼叫

### 2. 下載

從 [Releases](https://github.com/monkenWu/selfmd-claude-code/releases) 頁面下載適合你平台的執行檔。

| 平台 | 架構 | 檔案 |
|------|------|------|
| macOS | Apple Silicon (arm64) | `selfmd-v1.0.0-macos-arm64` |
| macOS | Intel (amd64) | `selfmd-v1.0.0-macos-amd64` |
| Linux | arm64 | `selfmd-v1.0.0-linux-arm64` |
| Linux | amd64 | `selfmd-v1.0.0-linux-amd64` |
| Windows | arm64 | `selfmd-v1.0.0-windows-arm64.exe` |
| Windows | amd64 | `selfmd-v1.0.0-windows-amd64.exe` |

```bash
# macOS / Linux：賦予執行權限、重新命名並移至 PATH
chmod +x selfmd-v1.0.0-macos-arm64
sudo mv selfmd-v1.0.0-macos-arm64 /usr/local/bin/selfmd
```

```powershell
# Windows (PowerShell)：建立目錄、移動執行檔、加入 PATH
mkdir "$env:USERPROFILE\selfmd"
Rename-Item selfmd-v1.0.0-windows-amd64.exe selfmd.exe
Move-Item selfmd.exe "$env:USERPROFILE\selfmd\selfmd.exe"
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\selfmd", "User")
```

```cmd
:: Windows (CMD)：建立目錄、移動執行檔、加入 PATH
mkdir "%USERPROFILE%\selfmd"
ren selfmd-v1.0.0-windows-amd64.exe selfmd.exe
move selfmd.exe "%USERPROFILE%\selfmd\selfmd.exe"
setx Path "%Path%;%USERPROFILE%\selfmd"
```

完成後即可在任意目錄直接執行 `selfmd`。

### 3. 初始化

進入你的專案根目錄，執行：

```bash
selfmd init
```

工具會掃描你的專案，自動偵測專案類型（Go、Node.js、Python、Rust、Java、Ruby 等），並產生 `selfmd.yaml` 設定檔。

### 4. 產生文件

```bash
selfmd generate
```

就這樣。工具會自動執行：

1. 掃描專案結構與原始碼
2. 呼叫 Claude Code CLI 規劃文件目錄
3. 並行產生每一頁文件內容
4. 輸出完整的靜態文件網站

完成後，用瀏覽器開啟 `.doc-build/index.html` 即可瀏覽文件。

---

## 指令

| 指令 | 說明 |
|------|------|
| `selfmd init` | 偵測專案類型並產生 `selfmd.yaml` |
| `selfmd generate` | 執行完整的文件產生流程 |
| `selfmd update` | 根據 `git diff` 增量更新文件 |
| `selfmd translate` | 將文件翻譯為其他語言 |

### 常用參數

```bash
# 預覽將被掃描的檔案，不呼叫 Claude
selfmd generate --dry-run

# 強制清除輸出目錄，從頭產生所有文件
selfmd generate --clean

# 提高並行度以加速產生
selfmd generate --concurrency 5
```

---

## 設定檔

`selfmd init` 會在專案根目錄產生 `selfmd.yaml`：

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
  language: "zh-TW"                     # 主要文件語言
  secondary_languages: ["en-US"]        # 翻譯目標語言
  clean_before_generate: false

claude:
  model: "sonnet"
  max_concurrent: 3          # 並行呼叫 Claude CLI 的數量
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

### 支援語言

`zh-TW` `zh-CN` `en-US` `ja-JP` `ko-KR` `fr-FR` `de-DE` `es-ES` `pt-BR` `th-TH` `vi-VN`

---

## 運作原理

```
selfmd generate
       │
       ▼
┌──────────────────┐
│  階段 1：掃描      │  依照 include/exclude 規則遍歷專案檔案
└───────┬──────────┘
        ▼
┌──────────────────────┐
│  階段 2：規劃目錄      │  Claude 分析程式碼結構並建立文件大綱
└───────┬──────────────┘
        ▼
┌──────────────────────────┐
│  階段 3：產生頁面內容       │  透過 Claude CLI 並行產生每一頁
└───────┬──────────────────┘
        ▼
┌───────────────────────────────┐
│  階段 4：索引與導航              │  產生索引、側邊欄與靜態 HTML 閱讀器
└───────────────────────────────┘
```

### 增量更新

初次產生完成後，使用 `selfmd update` 保持文件同步：

```bash
# 僅重新產生受近期程式碼變更影響的頁面
selfmd update
```

工具會比較目前的 commit 與上次記錄的 commit，找出變更的檔案，並只重新產生引用這些檔案的文件頁面。

---

## 輸出結構

```
.doc-build/
├── index.html            ← 用瀏覽器開啟此檔案即可瀏覽文件
├── index.md
├── _sidebar.md
├── _catalog.json         ← 目錄快取（供增量更新使用）
├── _last_commit          ← Git commit 記錄
├── _data.js              ← 靜態閱讀器的資料檔
├── overview/
│   └── index.md
├── core-modules/
│   ├── index.md
│   └── authentication/
│       └── index.md
└── ...
```

---

## 從原始碼編譯

需要 Go 1.25+。

```bash
go build -o selfmd .
```

---

## 授權

MIT

---

## 貢獻

歡迎提交 Issue 與 Pull Request！請先開 Issue 討論你想要變更的內容。
