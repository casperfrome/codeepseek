using System.Text.Json;
using System.Text.Json.Serialization;

namespace MoonBridgeSwitcher;

/// <summary>
/// Resolves and persists the location of the external codeepseek backend
/// (the folder containing <c>config.yml</c> and <c>mb_config.json</c>) plus the
/// local control-server endpoint the panel is served from.
/// </summary>
sealed class AppConfig
{
    /// <summary>Folder that holds <c>config.yml</c> and <c>mb_config.json</c>.</summary>
    public string BackendDir { get; set; } = "";

    /// <summary>Port the codeepseek control server listens on.</summary>
    public int Port { get; set; } = 38450;

    /// <summary>Full URL of the embedded config panel.</summary>
    [JsonIgnore]
    public string PanelUrl => $"http://127.0.0.1:{Port}/";

    /// <summary>Environment variable that, when set, overrides every other source.</summary>
    public const string EnvVar = "MOON_BRIDGE_DIR";

    static string ConfigPath => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "MoonBridgeSwitcher", "config.json");

    static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };

    /// <summary>A directory is a valid backend if it has the files the Go control server needs.</summary>
    public static bool IsValidBackendDir(string? dir) =>
        !string.IsNullOrWhiteSpace(dir) &&
        File.Exists(Path.Combine(dir, "config.yml")) &&
        File.Exists(Path.Combine(dir, "mb_config.json"));

    /// <summary>
    /// Loads config (creating defaults if absent) and resolves <see cref="BackendDir"/>
    /// in priority order: env var → saved config → auto-detected candidates.
    /// Returns the config; <see cref="BackendDir"/> may still be empty/invalid, in which
    /// case the caller should prompt the user via <see cref="PromptForBackendDir"/>.
    /// </summary>
    public static AppConfig LoadAndResolve()
    {
        var config = Load();

        // 1. Environment variable wins.
        var fromEnv = Environment.GetEnvironmentVariable(EnvVar);
        if (IsValidBackendDir(fromEnv))
        {
            config.BackendDir = fromEnv!;
            return config;
        }

        // 2. Previously saved (and still valid) path.
        if (IsValidBackendDir(config.BackendDir))
            return config;

        // 3. Auto-detect common locations.
        foreach (var candidate in CandidatePaths())
        {
            if (IsValidBackendDir(candidate))
            {
                config.BackendDir = candidate;
                return config;
            }
        }

        return config; // BackendDir not resolved — caller should prompt.
    }

    static IEnumerable<string> CandidatePaths()
    {
        var home = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        var exeDir = AppContext.BaseDirectory;

        // Preferred: backend subfolder inside the codeepseek repo / distribution root.
        yield return @"D:\moon_bridge_swticher\codeepseek\backend";
        // Relative to the launcher exe (works when exe is published into codeepseek\launcher\bin\...).
        yield return Path.Combine(exeDir, "..", "..", "..", "..", "..", "backend");
        // Sibling of the exe (works in single-file publish next to backend\).
        yield return Path.Combine(exeDir, "backend");
        // Legacy paths kept as fallback.
        yield return @"D:\moon_bridge_main\moon-bridge";
        yield return Path.Combine(home, "moon_bridge_main", "moon-bridge");
        yield return Path.Combine(home, "moon-bridge");
    }

    /// <summary>
    /// Shows a folder picker so the user can locate the codeepseek backend on first run.
    /// On a valid selection the path is saved and <c>true</c> is returned.
    /// </summary>
    public bool PromptForBackendDir(IWin32Window? owner = null)
    {
        using var dialog = new FolderBrowserDialog
        {
            Description = "Select the codeepseek backend folder (contains config.yml and mb_config.json).",
            UseDescriptionForTitle = true,
            ShowNewFolderButton = false
        };

        if (dialog.ShowDialog(owner) != DialogResult.OK)
            return false;

        if (!IsValidBackendDir(dialog.SelectedPath))
        {
            MessageBox.Show(owner,
                "The selected folder does not contain config.yml and mb_config.json.",
                "codeepseek", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            return false;
        }

        BackendDir = dialog.SelectedPath;
        Save();
        return true;
    }

    static AppConfig Load()
    {
        try
        {
            if (File.Exists(ConfigPath))
            {
                var json = File.ReadAllText(ConfigPath);
                var loaded = JsonSerializer.Deserialize<AppConfig>(json, JsonOptions);
                if (loaded is not null)
                    return loaded;
            }
        }
        catch { /* fall back to defaults on any read/parse error */ }

        return new AppConfig();
    }

    public void Save()
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(ConfigPath)!);
            File.WriteAllText(ConfigPath, JsonSerializer.Serialize(this, JsonOptions));
        }
        catch { /* persisting config is best-effort */ }
    }
}
