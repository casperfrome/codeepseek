# 🌙 Moon Bridge Switcher

A lightweight Windows desktop launcher for the **moon-bridge** model‑config panel.
It starts the moon-bridge backend in the background, waits for its local control
server, and embeds the configuration panel inside a single, clean WebView2 window —
so you don't have to keep a terminal and a browser tab open just to switch models.

> **Heads up:** this app is only the *launcher / shell*. It does **not** include the
> moon-bridge backend itself. You must already have a moon-bridge installation (the
> folder containing `config.yml` and `mb_control.ps1`). See
> [Backend requirement](#backend-requirement).

---

## Features

- One double-click to start the backend and open the config panel.
- Reuses an already-running backend instead of starting a second one.
- Cleans up the backend process tree on exit (no orphaned `moonbridge.exe`).
- Backend location is configurable — env var, saved config, auto-detect, or a first‑run folder picker.
- Self-contained single-file `.exe` (no .NET install required to *run* it).

## Backend requirement

This launcher talks to a separate **moon-bridge** backend. The backend folder must
contain:

- `config.yml`
- `mb_control.ps1`

The launcher runs `mb_control.ps1` via a hidden PowerShell process and expects the
control server to come up on **`http://127.0.0.1:38450/`**.

## Prerequisites

- Windows 10 / 11 (x64)
- [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (pre-installed on current Windows; otherwise install the Evergreen runtime)
- A working moon-bridge backend (see above)
- To **build** from source: [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)

## Build

```powershell
# Restore + build
dotnet build src/MoonBridgeSwitcher.csproj -c Release

# Produce a self-contained single-file exe in ./publish
dotnet publish src/MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

The resulting executable is `publish/MoonBridgeSwitcher.exe`.

## Configuration

The launcher resolves the backend folder in this order:

1. **Environment variable** `MOON_BRIDGE_DIR` — highest priority.
2. **Saved config** at `%LocalAppData%\MoonBridgeSwitcher\config.json`.
3. **Auto-detection** of common locations (e.g. `D:\moon_bridge_main\moon-bridge`, `%USERPROFILE%\moon-bridge`, a `moon-bridge` folder next to the exe).
4. **First-run folder picker** — if none of the above resolve, you're prompted to select the folder; your choice is saved to `config.json`.

`config.json` example:

```json
{
  "BackendDir": "D:\\moon_bridge_main\\moon-bridge",
  "Port": 38450
}
```

## Usage

1. Launch `MoonBridgeSwitcher.exe`.
2. On first run, point it at your moon-bridge backend folder if it isn't auto-detected.
3. The config panel opens once the backend is ready. Closing the window stops the backend it started.

## License

[MIT](LICENSE)

---

<a name="中文"></a>

# 🌙 Moon Bridge Switcher（中文说明）

一个轻量的 Windows 桌面启动器，用于打开 **moon-bridge** 的模型配置面板。
它在后台启动 moon-bridge 后端，等待本地控制服务器就绪，然后把配置面板嵌入到一个
简洁的 WebView2 窗口中——无需为了切换模型而一直开着终端和浏览器标签页。

> **注意：** 本程序只是*启动器 / 外壳*，**不包含** moon-bridge 后端本身。你需要
> 已经安装好 moon-bridge（即包含 `config.yml` 和 `mb_control.ps1` 的目录）。详见
> [后端依赖](#后端依赖)。

## 功能

- 双击即可启动后端并打开配置面板。
- 若后端已在运行，则直接复用，不会重复启动。
- 退出时清理后端进程树（不会残留 `moonbridge.exe`）。
- 后端目录可配置——环境变量、已保存配置、自动探测，或首次运行时的文件夹选择对话框。
- 自包含单文件 `.exe`（运行时无需安装 .NET）。

<a name="后端依赖"></a>

## 后端依赖

本启动器需要一个独立的 **moon-bridge** 后端。后端目录必须包含：

- `config.yml`
- `mb_control.ps1`

启动器会通过隐藏的 PowerShell 进程运行 `mb_control.ps1`，并等待控制服务器在
**`http://127.0.0.1:38450/`** 上启动。

## 环境要求

- Windows 10 / 11（x64）
- [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（新版 Windows 已自带，否则请安装 Evergreen 运行时）
- 可用的 moon-bridge 后端（见上）
- 如需**从源码构建**：[.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)

## 构建

```powershell
# 还原 + 构建
dotnet build src/MoonBridgeSwitcher.csproj -c Release

# 在 ./publish 生成自包含单文件 exe
dotnet publish src/MoonBridgeSwitcher.csproj -c Release -r win-x64 -o publish
```

生成的可执行文件为 `publish/MoonBridgeSwitcher.exe`。

## 配置

启动器按以下顺序解析后端目录：

1. **环境变量** `MOON_BRIDGE_DIR`——优先级最高。
2. **已保存配置**：`%LocalAppData%\MoonBridgeSwitcher\config.json`。
3. **自动探测**常见位置（如 `D:\moon_bridge_main\moon-bridge`、`%USERPROFILE%\moon-bridge`、exe 同级的 `moon-bridge` 目录等）。
4. **首次运行文件夹选择**：以上都未命中时弹出对话框让你选择，选择结果会保存到 `config.json`。

## 使用

1. 运行 `MoonBridgeSwitcher.exe`。
2. 首次运行时，如未自动探测到，请指向你的 moon-bridge 后端目录。
3. 后端就绪后会自动打开配置面板。关闭窗口会停止由它启动的后端。

## 许可证

[MIT](LICENSE)
