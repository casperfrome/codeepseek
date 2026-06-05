using System.Diagnostics;
using System.Net.Http;

namespace MoonBridgeSwitcher;

/// <summary>
/// Owns the lifecycle of the external moon-bridge backend: probing whether it is
/// already running, starting it as a hidden PowerShell process, and tearing it down
/// on exit (only if this app was the one that started it).
/// </summary>
sealed class BackendController
{
    readonly AppConfig _config;
    Process? _backend;
    bool _startedBackend;

    public BackendController(AppConfig config) => _config = config;

    /// <summary>True if a backend instance was started by this app and should be stopped on exit.</summary>
    public bool StartedBackend => _startedBackend;

    /// <summary>Checks whether the control server already answers on the configured port.</summary>
    public async Task<bool> PingAsync()
    {
        using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(2) };
        try
        {
            return (await http.GetAsync(_config.PanelUrl + "api/status")).IsSuccessStatusCode;
        }
        catch
        {
            return false;
        }
    }

    /// <summary>Polls <see cref="PingAsync"/> until the backend responds or the timeout elapses.</summary>
    public async Task<bool> WaitForBackendAsync(int timeoutMs)
    {
        var sw = Stopwatch.StartNew();
        while (sw.ElapsedMilliseconds < timeoutMs)
        {
            if (await PingAsync()) return true;
            await Task.Delay(400);
        }
        return false;
    }

    /// <summary>
    /// Starts the backend via a hidden <c>powershell.exe</c> running <c>mb_control.ps1</c>.
    /// Throws on failure so the caller can surface the error.
    /// </summary>
    public void Start()
    {
        var psi = new ProcessStartInfo("powershell.exe",
            "-NoProfile -ExecutionPolicy Bypass -File \"mb_control.ps1\"")
        {
            WorkingDirectory = _config.BackendDir,
            UseShellExecute = false,
            CreateNoWindow = true,
            WindowStyle = ProcessWindowStyle.Hidden
        };
        psi.EnvironmentVariables["MB_NO_BROWSER"] = "1"; // stop the ps1 from opening a browser
        _backend = Process.Start(psi);
        _startedBackend = true;
    }

    /// <summary>
    /// Stops the backend we started: kills the hidden PowerShell process tree (which
    /// includes the moonbridge.exe it launched), then sweeps up any leftover
    /// moonbridge.exe still rooted in the backend directory. No-op if we reused an
    /// instance we didn't start.
    /// </summary>
    public void Stop()
    {
        if (!_startedBackend) return; // reused an existing instance — leave it alone

        try
        {
            if (_backend is { HasExited: false })
            {
                Process.Start(new ProcessStartInfo("taskkill", $"/PID {_backend.Id} /T /F")
                {
                    CreateNoWindow = true,
                    UseShellExecute = false
                })?.WaitForExit(3000);
            }
        }
        catch { /* ignore */ }

        // Fallback: clean up any moonbridge.exe still running from the backend dir,
        // which would otherwise keep the control ports occupied.
        foreach (var p in Process.GetProcessesByName("moonbridge"))
        {
            try
            {
                if (p.MainModule?.FileName?.StartsWith(_config.BackendDir, StringComparison.OrdinalIgnoreCase) == true)
                    p.Kill(true);
            }
            catch { /* ignore */ }
        }
    }
}
