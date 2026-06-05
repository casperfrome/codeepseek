# 🌙 Moon Bridge Switcher

A lightweight Windows desktop launcher + control panel for **moon-bridge**.
Double-click it to start the bridge in the background and get a clean window for
switching models, editing your config, toggling Codex routing, and checking your
DeepSeek balance — no terminal, no browser tab.

It has two parts:

- **`MoonBridgeSwitcher.exe`** — a small C# WinForms + WebView2 window (the GUI shell).
- **`mbcontrol.exe`** — a single-file Go control server (port `38450`) that serves the
  panel, owns the `moonbridge.exe` child process, and exposes the panel's JSON API.
  This is a Go rewrite of the old `mb_control.ps1` PowerShell script.

```
MoonBridgeSwitcher.exe  ──starts──▶  mbcontrol.exe  ──starts──▶  moonbridge.exe
   (WebView2 window)                  (:38450 panel+API)          (:38440 bridge)
        │                                   ▲
        └────────── embeds panel ───────────┘
```

> **Heads up:** this project is the *launcher + control layer*. It does **not** include
> the moon-bridge backend itself. You must already have a moon-bridge installation — the
> folder containing `config.yml`, `mb_config.json`, and `moonbridge.exe`. See
> [Backend requirement](#backend-requirement).

---

## Features

- One double-click starts the bridge and opens the config panel.
- Reuses an already-running control server instead of starting a second one.
- Cleans up the `mbcontrol` → `moonbridge` process tree on exit.
- Go control backend with **zero external dependencies** (stdlib only); the panel UI is embedded in the binary.
- Backend location is configurable — env var, saved config, auto-detect, or a first-run folder picker.

## Backend requirement

`mbcontrol.exe` operates on a **moon-bridge** backend directory, which must contain:

- `config.yml` — generated bridge config
- `mb_config.json` — the panel's editable source of truth (**holds API keys**)
- `moonbridge.exe` — the bridge binary (or the moon-bridge Go source, to fall back to `go run ./cmd/moonbridge`)

The control server listens on **`http://127.0.0.1:38450/`** and expects the bridge on
**`127.0.0.1:38440`**.

## Prerequisites

- Windows 10 / 11 (x64)
- [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (pre-installed on current Windows)
- A working moon-bridge backend (see above)
- To **build** from source: [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0) and [Go 1.25+](https://go.dev/dl/)

## Build

```powershell
# 1. Go control backend -> mbcontrol.exe
cd control
go build -o ..\mbcontrol.exe .
cd ..

# 2. C# launcher -> self-contained single-file exe in .\publish
dotnet publish launcher/MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

Place `mbcontrol.exe` **next to `MoonBridgeSwitcher.exe`** (the launcher looks there
first, then falls back to the backend directory). Pre-built binaries for both are
attached to each tagged GitHub release.

## Configuration

The launcher resolves the backend folder in this order:

1. **Environment variable** `MOON_BRIDGE_DIR` — highest priority.
2. **Saved config** at `%LocalAppData%\MoonBridgeSwitcher\config.json`.
3. **Auto-detection** of common locations (e.g. `D:\moon_bridge_main\moon-bridge`, `%USERPROFILE%\moon-bridge`, a `moon-bridge` folder next to the exe).
4. **First-run folder picker** — if none of the above resolve, you're prompted to pick the folder; your choice is saved.

`mbcontrol.exe` also takes flags when run standalone:
`-root <backend dir>` (default: current directory), `-port <n>` (default `38450`),
`-no-browser` (don't open a browser; hide the bridge window).

## Usage

1. Launch `MoonBridgeSwitcher.exe`.
2. On first run, point it at your moon-bridge backend folder if it isn't auto-detected.
3. The panel opens once the bridge is ready. Closing the window stops the backend it started.

## Security note

`mb_config.json` contains real API keys and lives in your moon-bridge backend directory —
**not** in this repo. It (and `config.yml`, `bridge*.log`) is listed in `.gitignore`
so it can never be committed by accident. Never paste your `mb_config.json` into the repo.

## License

[MIT](LICENSE)

---

<a name="中文"></a>

# 🌙 Moon Bridge Switcher（中文说明）

一个轻量的 Windows 桌面启动器 + 控制面板，用于 **moon-bridge**。双击即可在后台启动桥服务，
并打开一个简洁窗口来切换模型、编辑配置、切换 Codex 路由、查询 DeepSeek 余额——无需终端，
无需浏览器标签页。

它由两部分组成：

- **`MoonBridgeSwitcher.exe`** —— 一个小型 C# WinForms + WebView2 窗口（GUI 外壳）。
- **`mbcontrol.exe`** —— 一个单文件 Go 控制服务器（端口 `38450`），负责提供面板、管理
  `moonbridge.exe` 子进程，并暴露面板所需的 JSON API。它是旧的 `mb_control.ps1`
  PowerShell 脚本的 **Go 重写版**。

```
MoonBridgeSwitcher.exe  ──启动──▶  mbcontrol.exe  ──启动──▶  moonbridge.exe
   (WebView2 窗口)                  (:38450 面板+API)          (:38440 桥服务)
```

> **注意：** 本项目是*启动器 + 控制层*，**不包含** moon-bridge 后端本身。你需要已经
> 安装好 moon-bridge——即包含 `config.yml`、`mb_config.json` 和 `moonbridge.exe`
> 的目录。详见[后端依赖](#后端依赖)。

## 功能

- 双击即可启动桥服务并打开配置面板。
- 若控制服务器已在运行，则直接复用，不会重复启动。
- 退出时清理 `mbcontrol` → `moonbridge` 进程树。
- Go 控制后端**零外部依赖**（仅标准库）；面板 UI 已嵌入二进制。
- 后端目录可配置——环境变量、已保存配置、自动探测，或首次运行的文件夹选择对话框。

<a name="后端依赖"></a>

## 后端依赖

`mbcontrol.exe` 在一个 **moon-bridge** 后端目录上工作，该目录必须包含：

- `config.yml` —— 生成的桥配置
- `mb_config.json` —— 面板的可编辑数据源（**含 API Key**）
- `moonbridge.exe` —— 桥二进制（或 moon-bridge Go 源码，用于回退到 `go run ./cmd/moonbridge`）

控制服务器监听 **`http://127.0.0.1:38450/`**，桥服务在 **`127.0.0.1:38440`**。

## 环境要求

- Windows 10 / 11（x64）
- [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（新版 Windows 已自带）
- 可用的 moon-bridge 后端（见上）
- 如需**从源码构建**：[.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0) 与 [Go 1.25+](https://go.dev/dl/)

## 构建

```powershell
# 1. Go 控制后端 -> mbcontrol.exe
cd control
go build -o ..\mbcontrol.exe .
cd ..

# 2. C# 启动器 -> 自包含单文件 exe，输出到 .\publish
dotnet publish launcher/MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

把 `mbcontrol.exe` 放在 **`MoonBridgeSwitcher.exe` 同级目录**（启动器先在此查找，
再回退到后端目录）。每个打了 tag 的 GitHub Release 都会附带两个预编译二进制。

## 配置

启动器按以下顺序解析后端目录：

1. **环境变量** `MOON_BRIDGE_DIR`——优先级最高。
2. **已保存配置**：`%LocalAppData%\MoonBridgeSwitcher\config.json`。
3. **自动探测**常见位置（如 `D:\moon_bridge_main\moon-bridge`、`%USERPROFILE%\moon-bridge`、exe 同级的 `moon-bridge` 目录）。
4. **首次运行文件夹选择**：以上都未命中时弹出对话框让你选择，选择结果会被保存。

`mbcontrol.exe` 独立运行时支持命令行参数：`-root <后端目录>`（默认当前目录）、
`-port <端口>`（默认 `38450`）、`-no-browser`（不打开浏览器、隐藏桥窗口）。

## 使用

1. 运行 `MoonBridgeSwitcher.exe`。
2. 首次运行时，如未自动探测到，请指向你的 moon-bridge 后端目录。
3. 桥就绪后会自动打开面板。关闭窗口会停止由它启动的后端。

## 安全提示

`mb_config.json` 含有真实 API Key，位于你的 moon-bridge 后端目录中——**不在**本仓库内。
它（以及 `config.yml`、`bridge*.log`）已列入 `.gitignore`，不会被误提交。请勿把
`mb_config.json` 粘贴进仓库。

## 许可证

[MIT](LICENSE)
