# Codex 接入 DeepSeek（Windows / Moon Bridge）使用说明

> 原理：Codex CLI 只会说 OpenAI Responses 协议，DeepSeek 暴露的是 Anthropic 协议，二者不能直连。
> Moon Bridge 是中间桥：**Codex → http://127.0.0.1:38440/v1 → Moon Bridge 转换 → DeepSeek**。

---

## 一、环境要求（已为你核查）

| 组件 | 你的状态 | 不满足时怎么装 |
|------|----------|----------------|
| Go 1.25+ | ✅ go1.26.0（32 位，可用） | https://go.dev/dl/ ，建议装 **windows/amd64 64 位**；或 `scoop install go` |
| Node.js / npm | ✅ v24 / 11 | https://nodejs.org |
| Codex CLI | ✅ 0.118.0 | `npm install -g @openai/codex` |

> 32 位 Go 能正常用（本项目用纯 Go 的 SQLite，不需要 C 编译器），只是性能略低，想换 64 位可自行重装。

---

## 二、已为你处理的两件事

1. **config.yml 升级到 v5**：你原来的是旧版 v4 格式（顶层 `provider:` 嵌套），当前程序只认 v5，会直接加载失败。
   - 已把原文件备份为 `config.yml.v4.bak`
   - 已转换为 v5（保留你的 api_key、deepseek-v4-pro 参数、reasoning 档位）
2. **使用独立 Codex 配置目录**：`D:\moon_bridge_main\.codex`，**不动你现有的全局 `C:\Users\zevro\.codex`**。

---

## 三、完整流程（文字版）

### 步骤 0 — 检查环境
双击 `0_check_env.bat`，确认 Go / Node / Codex 都显示版本号。

### 步骤 1 — 编译 Moon Bridge（一次性）
双击 `1_build.bat`，在仓库根生成 `moonbridge.exe`。
（之后只要不更新源码就不用再编译；没编译也行，其它脚本会自动回落到 `go run`，只是每次慢几秒。）

### 步骤 2 — 启动 Moon Bridge 服务 + 模型配置面板
双击 `2_start_bridge.bat`。它会：
- 启动控制服务器（本窗口，端口 38450）并由它拉起桥服务（独立窗口，看到 `Transform server listening on 127.0.0.1:38440` 即成功）；
- **自动弹出浏览器配置面板** `http://127.0.0.1:38450`。

在面板里即可**切换模型**：选模型 → 填对应 Provider 的 API Key → 点「保存并重启」，桥会自动重启并（按勾选）同步重新生成 Codex 配置。

> **这个控制窗口要一直开着**；关闭它或 Ctrl+C 会同时停掉桥服务。
> 面板是配置的唯一编辑入口（写 `mb_config.json` 并生成 `config.yml`，旧文件备份为 `config.yml.bak`）；手改 `config.yml` 不会回灌面板。

（可选）自测服务是否通——另开 PowerShell：
```powershell
$b = @{ model="moonbridge"; input="你好，一句话介绍自己"; max_output_tokens=100 } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:38440/v1/responses -Method Post -ContentType "application/json" -Body $b
```
返回里有 `"status":"completed"` 就说明 DeepSeek 已经通了。

### 步骤 3 — 生成 Codex 配置（一次性）
双击 `3_gen_codex_config.bat`。它会在 `D:\moon_bridge_main\.codex` 下生成两份文件：
- `config.toml` —— Codex 的模型/Provider 配置（指向本地 38440）
- `models_catalog.json` —— 模型能力描述（上下文窗口、推理档位等）

### 步骤 4 — 启动 Codex
双击 `4_run_codex.bat`（默认在脚本所在目录运行 Codex）。
想在某个项目里用 Codex，可把该**项目文件夹直接拖到 `4_run_codex.bat` 上**，或命令行：
```bat
4_run_codex.bat "D:\你的项目路径"
```
Codex 启动后随便提问，如果 **2_start_bridge.bat 那个服务窗口出现 `POST /v1/responses` 日志**，说明 Codex 已成功走 Moon Bridge 调 DeepSeek。

---

## 四、日常使用顺序

- **一次性**：`0_check_env` → `1_build` → `3_gen_codex_config`
- **每次用**：开 `2_start_bridge`（保持开着）→ 再开 `4_run_codex`
- config.yml 改了 → 重跑 `2`（重启服务）
- 模型/能力字段改了 → 重跑 `3`（重新生成 Codex 配置）

---

## 五、排错速查

| 现象 | 原因 / 解决 |
|------|-------------|
| `connection refused` | Moon Bridge 没起，先跑 `2_start_bridge.bat` |
| `401` / `403` | DeepSeek api_key 不对，检查 `config.yml` |
| `402 payment required` | DeepSeek 余额不足，去官网充值 |
| Codex 报 `config.toml ... unexpected key` | 别手动拼生成命令；用 `3_gen_codex_config.bat`（已把命令分开） |
| Codex 报找不到 `models_catalog.json` | 确认 `3` 里 `-codex-home` 与 `4` 里 `CODEX_HOME` 都是 `D:\moon_bridge_main\.codex` |
| `cannot unmarshal` / `field ... not found` | config.yml 格式/缩进问题（必须 2 空格，不能用 Tab） |

---

## 七、用 Codex 桌面 App（图形界面，非终端）

Codex 官方有桌面 App，用 **`codex app`** 启动（你之前问的就是它）。要求 codex 版本够新：
- codex 已升级到 **0.137.0**（含 `codex app`，已实测 SUPPORTED）。日后如需再升级：
  ```powershell
  npm install -g @openai/codex@latest
  ```

然后用 **`5_run_codex_app.bat`** 启动桌面 App：
1. 保持 `2_start_bridge.bat` 服务窗口开着；
2. 双击 `5_run_codex_app.bat`（它会设好 `CODEX_HOME`、检查 `codex app` 是否可用、再启动桌面 App）。
   - **首次启动会自动下载并安装 Codex Desktop 桌面程序**（会弹安装界面，跟着点完即可），之后再运行就直接打开。

> ⚠️ 待确认：桌面 App 和终端版用的是同一套 `~/.codex/config.toml`（含 `model_providers` + `wire_api="responses"`），按设计**应当**同样走 moon-bridge→DeepSeek。但桌面 App 首启可能会引导你登录 ChatGPT；升级后我们实测一下，若它强制登录/不认自定义 provider，再想办法。
>
> 终端版（`4_run_codex.bat`）继续可用，作为备选。

## 六、想换模型 / 加能力？

**换模型**：直接用 `2_start_bridge.bat` 弹出的配置面板（选模型 + 填 Key + 「保存并重启」），无需手改文件。

进阶能力仍可手改 `config.yml` 再重启服务，参考仓库 `CookBook_Win.md`：
- 换 Provider（如 Claude/Kimi）：菜谱 3
- 打开 DeepSeek V4 深度推理：菜谱 4（本配置已开 `deepseek_v4` 扩展）
- 看图（Visual）：菜谱 5
- 联网搜索（Tavily）：菜谱 6
- Prompt 缓存：菜谱 7
