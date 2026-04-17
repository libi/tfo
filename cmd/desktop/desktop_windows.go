//go:build windows

package main

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/libi/tfo/internal/app"
	"github.com/libi/tfo/internal/config"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wopts "github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed icons/icon.ico
var windowsIcon []byte

//go:embed popup.html
var popupHTML []byte

// ---------------------------------------------------------------------------
// Platform helpers
// ---------------------------------------------------------------------------

func openBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func detectSystemLang() Lang {
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := os.Getenv(k); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				return LangZH
			}
			return LangEN
		}
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	langID, _, _ := proc.Call()
	if langID&0x3FF == 0x04 {
		return LangZH
	}
	return LangEN
}

// platformDataDirChooser Windows 下不弹选择器，使用默认目录（可执行文件同级）。
func platformDataDirChooser() app.DataDirChooser {
	return nil
}

// ---------------------------------------------------------------------------
// System tray
// ---------------------------------------------------------------------------

var serverReady atomic.Bool

// popupCtx holds the Wails context, set once during OnStartup.
var popupCtx context.Context

// popupVisible tracks whether the popup is currently shown.
var popupVisible atomic.Bool

// showGuardUntil prevents hidePopup from firing right after showPopup.
var showGuardUntil atomic.Int64

func runDesktop(state *appState) {
	// Pre-load Wails popup in background goroutine (hidden).
	go preloadPopup(state)

	// Register global hotkey Alt+Shift+F
	go registerGlobalHotkey()

	systray.Run(func() {
		onReady(state)
	}, func() {
		unregisterGlobalHotkey()
		if popupCtx != nil {
			wailsRuntime.Quit(popupCtx)
		}
		state.stopServer()
	})
}

func onReady(state *appState) {
	systray.SetIcon(windowsIcon)
	systray.SetTitle("TFO")
	systray.SetTooltip("TFO")

	systray.SetOnClick(func(menu systray.IMenu) {
		go togglePopup()
	})

	openDashboardWhenReady(state, desktopStartupReadyTimeout, false, func(url string) {
		serverReady.Store(true)
		systray.SetTooltip("TFO — " + state.Addr())
		if popupCtx != nil {
			wailsRuntime.EventsEmit(popupCtx, "server-ready")
		}
	}, func(err error) {
		systray.SetTooltip("TFO — Error")
	})
}

// togglePopup shows or hides the popup depending on current state.
func togglePopup() {
	if popupVisible.Load() {
		hidePopup()
		return
	}
	showPopup()
}

// showPopup positions the pre-loaded window near the tray and shows it.
func showPopup() {
	ctx := popupCtx
	if ctx == nil {
		return
	}
	// Set guard: ignore hidePopup calls for 600ms after show.
	showGuardUntil.Store(time.Now().Add(600 * time.Millisecond).UnixMilli())
	popupVisible.Store(true)

	posX, posY := getTrayPosition()
	wailsRuntime.WindowSetPosition(ctx, posX, posY)
	wailsRuntime.WindowShow(ctx)
	wailsRuntime.WindowSetAlwaysOnTop(ctx, true)
	// Reset textarea and focus via JS.
	wailsRuntime.EventsEmit(ctx, "popup-show")
}

// hidePopup hides the window without destroying it.
func hidePopup() {
	ctx := popupCtx
	if ctx == nil {
		return
	}
	// Respect show guard to avoid hiding immediately after showing.
	if time.Now().UnixMilli() < showGuardUntil.Load() {
		return
	}
	if popupVisible.CompareAndSwap(true, false) {
		wailsRuntime.WindowHide(ctx)
	}
}

// ---------------------------------------------------------------------------
// PopupAPI — bound to Wails, called from JS frontend
// ---------------------------------------------------------------------------

// PopupAPI is the Go backend for the capture popup.
type PopupAPI struct {
	state *appState
}

// InitResult is sent to JS on page load.
type InitResult struct {
	Placeholder string `json:"placeholder"`
	Submit      string `json:"submit"`
	Ready       bool   `json:"ready"`
	HotkeySave  string `json:"hotkeySave"`
}

// Init returns i18n strings and server readiness.
func (p *PopupAPI) Init() InitResult {
	hotkeySave := "Ctrl+Enter"
	if p.state.app != nil {
		if cfg := p.state.app.GetConfig(); cfg != nil && cfg.HotkeySave != "" {
			hotkeySave = cfg.HotkeySave
		}
	}
	return InitResult{
		Placeholder: T("placeholder.capture"),
		Submit:      T("button.submit"),
		Ready:       serverReady.Load(),
		HotkeySave:  hotkeySave,
	}
}

// Submit posts the note and hides the popup on success.
func (p *PopupAPI) Submit(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if err := submitNote(p.state, content); err != nil {
		slog.Error("submit note failed", "error", err)
		return err
	}
	slog.Info("note submitted via popup")
	hidePopup()
	return nil
}

// OpenSettings opens the dashboard and hides the popup.
func (p *PopupAPI) OpenSettings() {
	_ = openBrowser(p.state.DashboardURL())
	hidePopup()
}

// HidePopup hides the popup window (called from JS on blur / Escape).
func (p *PopupAPI) HidePopup() {
	hidePopup()
}

// Quit closes the entire app.
func (p *PopupAPI) Quit() {
	hidePopup()
	go systray.Quit()
}

// ChooseDataDir shows a native folder picker and updates the data directory.
func (p *PopupAPI) ChooseDataDir() {
	hidePopup()
	// Determine current data directory to pre-select in the picker.
	currentDir := config.DefaultDataDir()
	if bc, _, err := config.LoadBootstrap(); err == nil && bc.DataDir != "" {
		currentDir = bc.DataDir
	}
	go func() {
		dir := chooseDataDirWindows(currentDir)
		if dir == "" {
			return
		}
		slog.Info("user chose new data directory", "dir", dir)
		bootstrapDir := config.DefaultBootstrapDir()
		bc := &config.BootstrapConfig{DataDir: dir}
		if err := config.SaveBootstrap(bootstrapDir, bc); err != nil {
			slog.Error("failed to save bootstrap config", "error", err)
			return
		}
		// Restart server with new data directory
		serverReady.Store(false)
		if err := p.state.restartServer(); err != nil {
			slog.Error("failed to restart server", "error", err)
			return
		}
		openDashboardWhenReady(p.state, desktopStartupReadyTimeout, false, func(url string) {
			serverReady.Store(true)
			if popupCtx != nil {
				wailsRuntime.EventsEmit(popupCtx, "server-ready")
			}
		}, nil)
	}()
}

// ---------------------------------------------------------------------------
// Native folder picker (Windows)
// ---------------------------------------------------------------------------

func chooseDataDirWindows(defaultDir string) string {
	ole32 := syscall.NewLazyDLL("ole32.dll")
	coInitialize := ole32.NewProc("CoInitializeEx")
	coUninitialize := ole32.NewProc("CoUninitialize")

	coInitialize.Call(0, 0)
	defer coUninitialize.Call()

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shBrowseForFolder := shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDList := shell32.NewProc("SHGetPathFromIDListW")

	type browseInfo struct {
		HwndOwner    uintptr
		PidlRoot     uintptr
		DisplayName  *uint16
		Title        *uint16
		Flags        uint32
		CallbackFunc uintptr
		LParam       uintptr
		Image        int32
	}

	title := "Choose a folder to store TFO data"
	if currentLang == LangZH {
		title = "请选择 TFO 数据保存目录"
	}
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	displayName := make([]uint16, syscall.MAX_PATH)

	// Prepare default directory as UTF-16 for the callback
	defaultDirPtr, _ := syscall.UTF16PtrFromString(defaultDir)

	// BFFM_INITIALIZED = 1, BFFM_SETSELECTIONW = 0x0467
	callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, lp uintptr, data uintptr) uintptr {
		const bffmInitialized = 1
		const bffmSetSelectionW = 0x0467
		if msg == bffmInitialized {
			user32 := syscall.NewLazyDLL("user32.dll")
			sendMessage := user32.NewProc("SendMessageW")
			sendMessage.Call(hwnd, bffmSetSelectionW, 1, data)
		}
		return 0
	})

	bi := browseInfo{
		DisplayName:  &displayName[0],
		Title:        titlePtr,
		Flags:        0x00000001 | 0x00000010 | 0x00000040, // BIF_RETURNONLYFSDIRS | BIF_EDITBOX | BIF_NEWDIALOGSTYLE
		CallbackFunc: callback,
		LParam:       uintptr(unsafe.Pointer(defaultDirPtr)),
	}

	pidl, _, _ := shBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}

	buf := make([]uint16, syscall.MAX_PATH)
	shGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))

	return syscall.UTF16ToString(buf)
}

// ---------------------------------------------------------------------------
// Global Hotkey (Alt+Shift+F)
// ---------------------------------------------------------------------------

const hotkeyID = 1

func registerGlobalHotkey() {
	user32 := syscall.NewLazyDLL("user32.dll")
	registerHotKey := user32.NewProc("RegisterHotKey")
	getMessage := user32.NewProc("GetMessageW")

	// MOD_ALT=0x0001, MOD_SHIFT=0x0004, VK_F=0x46
	ret, _, err := registerHotKey.Call(0, uintptr(hotkeyID), 0x0001|0x0004, 0x46)
	if ret == 0 {
		slog.Warn("failed to register global hotkey Alt+Shift+F", "error", err)
		return
	}
	slog.Info("global hotkey Alt+Shift+F registered")

	// Message loop for hotkey events
	type msg struct {
		HWnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      struct{ X, Y int32 }
	}
	var m msg
	for {
		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		if m.Message == 0x0312 { // WM_HOTKEY
			go togglePopup()
		}
	}
}

func unregisterGlobalHotkey() {
	user32 := syscall.NewLazyDLL("user32.dll")
	unregisterHotKey := user32.NewProc("UnregisterHotKey")
	unregisterHotKey.Call(0, uintptr(hotkeyID))
}

// ---------------------------------------------------------------------------
// Wails popup window (pre-loaded, hidden)
// ---------------------------------------------------------------------------

func getTrayPosition() (x, y int) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("SystemParametersInfoW")
	type sRECT struct{ Left, Top, Right, Bottom int32 }
	var wa sRECT
	proc.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0)
	const winW, winH, margin = 360, 280, 12
	return int(wa.Right) - winW - margin, int(wa.Bottom) - winH - margin
}

func preloadPopup(state *appState) {
	api := &PopupAPI{state: state}

	err := wails.Run(&options.App{
		Title:             "TFO",
		Width:             360,
		Height:            280,
		MinWidth:          360,
		MinHeight:         280,
		MaxWidth:          360,
		MaxHeight:         280,
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       true,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Handler: popupHandler{},
		},
		OnStartup: func(ctx context.Context) {
			popupCtx = ctx
		},
		OnDomReady: func(ctx context.Context) {
			slog.Info("popup webview preloaded")
		},
		Bind: []interface{}{api},
		Windows: &wopts.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    true,
		},
	})
	if err != nil {
		slog.Error("wails popup error", "error", err)
	}
}

// popupHandler serves the embedded popup.html for all requests.
type popupHandler struct{}

func (popupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(popupHTML)
}
