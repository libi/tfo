//go:build windows

package main

import (
	_ "embed"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/energye/systray"
)

//go:embed icons/icon.ico
var windowsIcon []byte

// ---------------------------------------------------------------------------
// Platform helpers
// ---------------------------------------------------------------------------

func openBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func detectSystemLang() Lang {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := os.Getenv(key); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				return LangZH
			}
			return LangEN
		}
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	langID, _, _ := proc.Call()

	const langChinese = 0x04
	if langID&0x3FF == langChinese {
		return LangZH
	}

	return LangEN
}

// ---------------------------------------------------------------------------
// System tray
// ---------------------------------------------------------------------------

// serverReady tracks whether the backend is reachable (shared across goroutines).
var serverReady atomic.Bool

// popupActive prevents opening multiple popups simultaneously.
var popupActive atomic.Bool

func runDesktop(state *appState) {
	systray.Run(func() {
		onReady(state)
	}, func() {
		state.stopServer()
	})
}

func onReady(state *appState) {
	systray.SetIcon(windowsIcon)
	systray.SetTitle("TFO")
	systray.SetTooltip("TFO")

	// Left-click tray icon → show native capture popup (matches macOS behaviour).
	systray.SetOnClick(func(menu systray.IMenu) {
		go showNotePopup(state)
	})

	openDashboardWhenReady(state, desktopStartupReadyTimeout, false, func(url string) {
		serverReady.Store(true)
		systray.SetTooltip("TFO — " + state.Addr())
	}, func(err error) {
		systray.SetTooltip("TFO — Error")
	})
}

// ---------------------------------------------------------------------------
// Native WPF popup (PowerShell, no CGO / no WebView)
// ---------------------------------------------------------------------------

// showNotePopup launches a native WPF popup for quick note capture on Windows.
func showNotePopup(state *appState) {
	if !popupActive.CompareAndSwap(false, true) {
		return
	}
	defer popupActive.Store(false)

	statusColor := "Orange"
	if serverReady.Load() {
		statusColor = "#34C759"
	}

	script := strings.NewReplacer(
		"{{STATUS_COLOR}}", statusColor,
		"{{DASHBOARD_URL}}", state.DashboardURL(),
		"{{PLACEHOLDER}}", T("placeholder.capture"),
		"{{BTN_SUBMIT}}", T("button.submit"),
	).Replace(wpfPopupScript)

	tmpFile, err := os.CreateTemp("", "tfo-popup-*.ps1")
	if err != nil {
		slog.Warn("popup: failed to create temp file", "error", err)
		_ = openBrowser(state.DashboardURL())
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// UTF-8 BOM so PowerShell reads CJK characters correctly.
	_, _ = tmpFile.Write([]byte{0xEF, 0xBB, 0xBF})
	_, _ = tmpFile.WriteString(script)
	tmpFile.Close()

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", tmpPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("popup closed without input", "error", err)
		return
	}

	result := strings.TrimSpace(string(output))
	switch {
	case result == "::QUIT::":
		systray.Quit()
	case result == "::SETTINGS::":
		_ = openBrowser(state.DashboardURL())
	case result == "":
		// closed without input
	default:
		if err := submitNote(state, result); err != nil {
			slog.Error("submit note failed", "error", err)
		} else {
			slog.Info("note submitted via popup")
		}
	}
}

// wpfPopupScript is a PowerShell script that shows a WPF popup matching the
// macOS native popover: borderless rounded window with a text area, status dot,
// settings/quit/submit buttons. Positioned near the taskbar tray area.
const wpfPopupScript = `
Add-Type -AssemblyName PresentationFramework,PresentationCore,WindowsBase
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

[xml]$xaml = @"
<Window
    xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
    xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
    Width="376" Height="300"
    WindowStartupLocation="Manual"
    ResizeMode="NoResize"
    ShowInTaskbar="False"
    Topmost="True"
    WindowStyle="None"
    AllowsTransparency="True"
    Background="Transparent">
  <Border Background="White" BorderBrush="#D0D0D0" BorderThickness="1"
          CornerRadius="8" ClipToBounds="True">
    <Grid>
      <Grid.RowDefinitions>
        <RowDefinition Height="*"/>
        <RowDefinition Height="Auto"/>
        <RowDefinition Height="44"/>
      </Grid.RowDefinitions>

      <!-- Text area with placeholder overlay -->
      <Grid Grid.Row="0" Margin="16,16,16,8">
        <TextBox x:Name="tb" AcceptsReturn="True" TextWrapping="Wrap"
                 BorderThickness="0" FontSize="14" FontFamily="Segoe UI"
                 VerticalScrollBarVisibility="Auto" Background="Transparent"
                 Padding="2"/>
        <TextBlock x:Name="ph" Text="{{PLACEHOLDER}}"
                   FontSize="14" FontFamily="Segoe UI"
                   Foreground="#AAAAAA" IsHitTestVisible="False"
                   Margin="4,6,0,0"/>
      </Grid>

      <!-- Separator -->
      <Rectangle Grid.Row="1" Fill="#EBEBEB" Height="1"/>

      <!-- Bottom bar -->
      <Grid Grid.Row="2" Margin="16,0">
        <StackPanel Orientation="Horizontal" VerticalAlignment="Center">
          <Ellipse Width="8" Height="8" Fill="{{STATUS_COLOR}}"/>
          <Button x:Name="gear" Content="&#x2699;" Margin="12,0,0,0"
                  Width="28" Height="28" BorderThickness="0"
                  Background="Transparent" FontSize="15"
                  Foreground="#666666" Cursor="Hand"
                  ToolTip="Settings"/>
          <Button x:Name="pwr" Content="&#x23FB;" Margin="6,0,0,0"
                  Width="28" Height="28" BorderThickness="0"
                  Background="Transparent" FontSize="14"
                  Foreground="#666666" Cursor="Hand"
                  ToolTip="Quit"/>
        </StackPanel>
        <Button x:Name="send" Content="&#x27A4;"
                HorizontalAlignment="Right" VerticalAlignment="Center"
                Width="28" Height="28" BorderThickness="0"
                Background="Transparent" FontSize="16"
                Foreground="#444444" Cursor="Hand"
                ToolTip="{{BTN_SUBMIT}}"/>
      </Grid>
    </Grid>
  </Border>
</Window>
"@

$reader = [System.Xml.XmlNodeReader]::new($xaml)
$window = [System.Windows.Markup.XamlReader]::Load($reader)

$tb = $window.FindName("tb")
$ph = $window.FindName("ph")

# Position near the system tray (bottom-right of work area).
$wa = [System.Windows.SystemParameters]::WorkArea
$window.Left = $wa.Right  - $window.Width  - 12
$window.Top  = $wa.Bottom - $window.Height - 12

# Drag the borderless window by its background.
$window.Add_MouseLeftButtonDown({
    param($sender, $e)
    if ($e.ChangedButton -eq "Left") { $window.DragMove() }
})

# Close on deactivate (click outside), mirroring macOS popover behaviour.
$window.Add_Deactivated({ $window.Close() })

# Placeholder visibility.
$tb.Add_TextChanged({
    if ($tb.Text.Length -gt 0) { $ph.Visibility = "Collapsed" }
    else                       { $ph.Visibility = "Visible"   }
})

$script:result = ""

$window.FindName("send").Add_Click({
    $script:result = $tb.Text
    $window.Close()
})

$window.FindName("gear").Add_Click({
    $script:result = "::SETTINGS::"
    $window.Close()
})

$window.FindName("pwr").Add_Click({
    $script:result = "::QUIT::"
    $window.Close()
})

# Focus the text box on open.
$window.Add_ContentRendered({ $tb.Focus() | Out-Null })

$window.ShowDialog() | Out-Null
Write-Output $script:result
`
