using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.WinForms;

namespace MoonBridgeSwitcher;

/// <summary>
/// Main window: resolves the backend location, ensures the backend is running, then
/// embeds the moon-bridge config panel in a WebView2. The backend is torn down on close.
/// </summary>
sealed class MainForm : Form
{
    readonly AppConfig _config = AppConfig.LoadAndResolve();
    readonly BackendController _backend;

    readonly WebView2 _web = new() { Dock = DockStyle.Fill };
    readonly Label _status = new()
    {
        Dock = DockStyle.Fill,
        TextAlign = ContentAlignment.MiddleCenter,
        Text = "正在启动 Moon Bridge…",
        ForeColor = Color.White,
        Font = new Font("Microsoft YaHei", 12f)
    };

    public MainForm()
    {
        _backend = new BackendController(_config);

        Text = "🌙 Moon Bridge 模型配置";
        Width = 940;
        Height = 920;
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = ColorTranslator.FromHtml("#0f1320");

        // _status fills the window; _web sits on top and is revealed once ready.
        _web.Visible = false;
        Controls.Add(_web);
        Controls.Add(_status);

        Load += OnLoad;
        FormClosing += OnClosing;
    }

    async void OnLoad(object? sender, EventArgs e)
    {
        // Resolve the backend folder; prompt on first run if it couldn't be auto-detected.
        if (!AppConfig.IsValidBackendDir(_config.BackendDir) && !_config.PromptForBackendDir(this))
        {
            MessageBox.Show(this,
                "No moon-bridge backend folder was selected.\n\n" +
                $"Set it via the {AppConfig.EnvVar} environment variable, or pick the folder " +
                "containing config.yml and mb_config.json on next launch.",
                "Moon Bridge", MessageBoxButtons.OK, MessageBoxIcon.Error);
            Close();
            return;
        }

        // Start the hidden PowerShell backend only if 38450 isn't already serving.
        if (!await _backend.PingAsync())
        {
            try
            {
                _backend.Start();
            }
            catch (Exception ex)
            {
                _status.Text = "无法启动后端 powershell：\n" + ex.Message;
                return;
            }
        }

        if (!await _backend.WaitForBackendAsync(25000))
        {
            _status.Text = $"启动超时：未能连上控制服务器 ({_config.Port})。\n请检查 moon-bridge 后端日志。";
            return;
        }

        try
        {
            var udf = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "MoonBridgeSwitcher", "WebView2");
            var env = await CoreWebView2Environment.CreateAsync(null, udf);
            await _web.EnsureCoreWebView2Async(env);
            _web.CoreWebView2.Navigate(_config.PanelUrl);
            _status.Visible = false;
            _web.Visible = true;
        }
        catch (Exception ex)
        {
            _status.Text = "WebView2 初始化失败：\n" + ex.Message;
        }
    }

    void OnClosing(object? sender, FormClosingEventArgs e) => _backend.Stop();
}
