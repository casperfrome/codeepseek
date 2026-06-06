# codeepseek

一个轻量的 Windows 桌面启动器，双击即可在后台启动 Moon Bridge，并在内嵌窗口中打开配置面板。无需终端，无需浏览器标签。

> **English summary** — see the [English section](#english) at the bottom.

---

## 架构

```
MoonBridgeSwitcher.exe  ──启动──▶  mbcontrol.exe  ──启动──▶  moonbridge.exe
   (WebView2 窗口)                  (:38450 面板+API)          (:38440 桥服务)
        │                                  ▲
        └──────── 内嵌控制面板 ────────────┘
```

| 组件 | 语言 | 说明 |
|------|------|------|
| `MoonBridgeSwitcher.exe` | C# WinForms + WebView2 | GUI 外壳，负责后端生命周期管理 |
| `mbcontrol.exe` | Go（仅标准库） | HTTP 控制服务器，内嵌面板 HTML，管理 moonbridge 子进程 |
| `moonbridge.exe` | Go | 实际的桥接代理，位于 `backend/` |

---

## 目录结构

```
codeepseek/
├── control/     Go 控制后端源码  →  构建产物：mbcontrol.exe
├── launcher/    C# WinForms 启动器源码
├── backend/     Moon Bridge 后端（moonbridge.exe、Go 源码、面板 HTML 等）
├── build.ps1    一键构建脚本
└── publish/     构建输出目录
```

> `mb_config.json`（含 API Key）、`config.yml`、`*.exe`、`bridge*.log` 均已列入 `.gitignore`，不会被提交。

---

## 环境要求

- Windows 10 / 11（x64）
- [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（Windows 11 已内置）
- 如需从源码构建：[.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0) 与 [Go 1.25+](https://go.dev/dl/)

---

## 构建

### 一键构建（推荐）

```powershell
.\build.ps1
```

脚本会先检查 `go` 和 `dotnet` 是否在 `PATH` 上，然后依次构建 `mbcontrol.exe`（从 `control/`）和 `MoonBridgeSwitcher.exe`（从 `launcher/`），并将两者输出到 `.\publish\`。

| 参数 | 说明 |
|------|------|
| `-Clean` | 构建前清空输出目录 |
| `-Output <目录>` | 指定输出目录（默认 `publish`） |
| `-SkipControl` | 跳过构建 `mbcontrol.exe` |
| `-SkipLauncher` | 跳过构建 `MoonBridgeSwitcher.exe` |

```powershell
.\build.ps1 -Clean -Output dist    # 全新构建到 .\dist
```

> 若 PowerShell 因执行策略报错，执行：`powershell -ExecutionPolicy Bypass -File .\build.ps1`

### 手动构建

```powershell
# 1. Go 控制后端
cd control
go build -o ..\publish\mbcontrol.exe .
cd ..

# 2. C# 启动器（自包含单文件）
dotnet publish launcher\MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

构建完成后 `.\publish\` 中有两个文件，**必须放在同一目录**：

```
publish\
├── MoonBridgeSwitcher.exe   ← 双击运行
└── mbcontrol.exe            ← 必须同级
```

可将整个 `publish\` 目录复制到任意位置使用。每个打标签的 GitHub Release 也会附带预编译二进制。

---

## 后端目录

`mbcontrol.exe` 需要一个**后端目录**，其中必须包含：

- `config.yml` — 桥接配置
- `mb_config.json` — 面板数据源（**含 API Key，不提交**）
- `moonbridge.exe` — 桥接代理二进制

本仓库的 `backend/` 目录已包含上述所有内容（`*.exe` 和 `mb_config.json` 除外，需手动放置或从 [moon-bridge](https://github.com/casperfrome/moon-bridge) 构建）。

启动器按以下优先级自动定位后端目录：

1. 环境变量 `MOON_BRIDGE_DIR`
2. 上次保存的路径（`%LocalAppData%\MoonBridgeSwitcher\config.json`）
3. 自动探测常见位置：`codeepseek\backend\`、`%USERPROFILE%\moon-bridge` 等
4. 以上均未命中时弹出文件夹选择对话框，选择结果自动保存

---

## 使用

1. 双击 `MoonBridgeSwitcher.exe`
2. 若后端目录未被自动探测到，在弹出的对话框中选择包含 `config.yml` 和 `mb_config.json` 的目录，之后不会再询问
3. 稍等数秒，桥服务就绪后控制面板自动载入

**面板功能：**

| 功能 | 说明 |
|------|------|
| 切换模型 | 无需重启，直接切换当前模型 |
| 编辑配置 | 修改 API Key、端点、推理参数等 |
| Codex 路由 | 在原生 GPT 与 Moon Bridge 之间切换全局 `~/.codex/config.toml` |
| DeepSeek 余额 | 查询余额及上次刷新后的消费额 |
| 启动 Codex | 以指定工作目录启动 `codex app` |
| 重启桥服务 | 不关闭面板的情况下重启 `moonbridge.exe` |

关闭窗口即停止——程序会自动清理整个进程树（`mbcontrol` + `moonbridge`）。

---

## 独立运行 mbcontrol

```powershell
mbcontrol.exe -root "D:\my-backend" -port 38450 -no-browser
```

| 参数 | 说明 |
|------|------|
| `-root <路径>` | 后端目录；默认当前目录 |
| `-port <端口>` | 监听端口；默认 `38450` |
| `-no-browser` | 不自动打开浏览器，隐藏桥服务窗口 |

---

## 安全提示

`mb_config.json` 含有真实 API Key，已列入 `.gitignore`，**永远不会被提交**。请勿将其内容粘贴到仓库任何地方。

---

## 许可证

[MIT](LICENSE)

---

<a name="english"></a>

# codeepseek — English

A lightweight Windows desktop launcher + control panel for **Moon Bridge**. Double-click to start the bridge in the background and open a clean WebView2 window for switching models, editing config, toggling Codex routing, and checking your DeepSeek balance — no terminal, no browser tab.

## Architecture

```
MoonBridgeSwitcher.exe  ──starts──▶  mbcontrol.exe  ──starts──▶  moonbridge.exe
   (WebView2 window)                  (:38450 panel+API)          (:38440 bridge)
        │                                    ▲
        └──────── embeds panel ──────────────┘
```

## Repo layout

```
codeepseek/
├── control/     Go control backend source  →  mbcontrol.exe
├── launcher/    C# WinForms launcher source
├── backend/     Moon Bridge backend (Go source, moonbridge.exe, panel HTML)
├── build.ps1    One-click build script
└── publish/     Build output
```

## Prerequisites

- Windows 10 / 11 (x64)
- [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (built into Windows 11)
- To build from source: [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0) and [Go 1.25+](https://go.dev/dl/)

## Build

```powershell
.\build.ps1                      # build everything into .\publish
.\build.ps1 -Clean -Output dist  # clean build into .\dist
```

Or manually:

```powershell
cd control && go build -o ..\publish\mbcontrol.exe . && cd ..
dotnet publish launcher\MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

Keep `mbcontrol.exe` next to `MoonBridgeSwitcher.exe` — the launcher looks there first.

## Usage

1. Double-click `MoonBridgeSwitcher.exe`.
2. On first run, pick the backend folder (must contain `config.yml` and `mb_config.json`) if auto-detection misses it. Your choice is saved.
3. The control panel loads in the window once the bridge is ready.

**Panel features:** switch models · edit config (API keys, endpoints) · toggle Codex routing between native GPT and Moon Bridge · check DeepSeek balance · launch `codex app` · restart the bridge.

Close the window to shut down the full process tree.

## Backend directory

The launcher resolves the backend folder in order: `MOON_BRIDGE_DIR` env var → saved config → auto-detected locations (including `codeepseek\backend\`) → folder picker on first run.

The repo's `backend/` directory contains everything except the binaries (`moonbridge.exe`) and the secrets file (`mb_config.json`), which you supply separately.

## Security

`mb_config.json` holds real API keys and is in `.gitignore` — it is **never committed**. Do not paste it into the repository.

## License

[MIT](LICENSE)
