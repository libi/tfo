import Carbon.HIToolbox
import Cocoa

// MARK: - Popover Content View Controller

/// Parsed shortcut representation
private struct ParsedShortcut {
    var ctrl: Bool = false
    var alt: Bool = false
    var shift: Bool = false
    var cmd: Bool = false
    var key: String = ""

    static func parse(_ shortcut: String) -> ParsedShortcut {
        var result = ParsedShortcut()
        let parts = shortcut.components(separatedBy: "+").map {
            $0.trimmingCharacters(in: .whitespaces).lowercased()
        }
        for part in parts {
            switch part {
            case "ctrl", "control": result.ctrl = true
            case "alt", "option": result.alt = true
            case "shift": result.shift = true
            case "cmd", "command", "meta": result.cmd = true
            default: result.key = part
            }
        }
        return result
    }

    func matches(event: NSEvent) -> Bool {
        let mods = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
        guard mods.contains(.control) == ctrl,
            mods.contains(.option) == alt,
            mods.contains(.shift) == shift,
            mods.contains(.command) == cmd
        else { return false }
        guard let chars = event.charactersIgnoringModifiers?.lowercased() else { return false }
        // Map special key names
        let eventKey: String
        switch chars {
        case "\r": eventKey = "enter"
        case " ": eventKey = "space"
        default: eventKey = chars
        }
        return eventKey == key
    }
}

private final class ShortcutTextView: NSTextView {
    var onSubmit: (() -> Void)?
    var submitShortcut: ParsedShortcut = ParsedShortcut.parse("Ctrl+Enter")

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        // Check submit shortcut first
        if submitShortcut.matches(event: event) {
            onSubmit?()
            return true
        }

        let modifiers = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
        let isCommandShortcut =
            modifiers.contains(.command)
            && !modifiers.contains(.control)
            && !modifiers.contains(.option)
        guard isCommandShortcut,
            let characters = event.charactersIgnoringModifiers?.lowercased()
        else {
            return super.performKeyEquivalent(with: event)
        }

        switch characters {
        case "a":
            selectAll(nil)
            return true
        case "c":
            copy(nil)
            return true
        case "v":
            paste(nil)
            return true
        case "x":
            cut(nil)
            return true
        case "z":
            if modifiers.contains(.shift) {
                undoManager?.redo()
            } else {
                undoManager?.undo()
            }
            return true
        default:
            return super.performKeyEquivalent(with: event)
        }
    }
}

class PopoverViewController: NSViewController, NSTextViewDelegate {

    private var textView: NSTextView!
    private var scrollView: NSScrollView!
    private var submitButton: NSButton!
    private var settingsButton: NSButton!
    private var folderButton: NSButton!
    private var quitButton: NSButton!
    private var placeholderLabel: NSTextField!
    private var statusDot: NSView!
    private var bottomBar: NSView!
    private let textHorizontalInset: CGFloat = 6
    private let textVerticalInset: CGFloat = 6

    weak var appDelegate: AppDelegate?

    override func loadView() {
        let container = NSView(frame: NSRect(x: 0, y: 0, width: 360, height: 280))
        container.wantsLayer = true
        container.layer?.backgroundColor = NSColor.white.cgColor
        self.view = container
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        focusTextView()
    }

    private func setupUI() {
        let width: CGFloat = 360
        let totalHeight: CGFloat = 280
        let padding: CGFloat = 16
        let bottomBarHeight: CGFloat = 44

        // --- Bottom bar (settings + quit on left, submit on right) ---
        bottomBar = NSView(frame: NSRect(x: 0, y: 0, width: width, height: bottomBarHeight))
        bottomBar.wantsLayer = true
        bottomBar.layer?.backgroundColor = NSColor(white: 0.975, alpha: 1.0).cgColor

        // Top separator line
        let separator = NSView(frame: NSRect(x: 0, y: bottomBarHeight - 1, width: width, height: 1))
        separator.wantsLayer = true
        separator.layer?.backgroundColor = NSColor(white: 0.90, alpha: 1.0).cgColor
        bottomBar.addSubview(separator)

        // Status dot
        statusDot = NSView(
            frame: NSRect(x: padding, y: (bottomBarHeight - 8) / 2, width: 8, height: 8))
        statusDot.wantsLayer = true
        statusDot.layer?.cornerRadius = 4
        statusDot.layer?.backgroundColor = NSColor.systemOrange.cgColor
        bottomBar.addSubview(statusDot)

        // Settings button (gear icon)
        settingsButton = makeIconButton(
            symbolName: "gearshape",
            fallbackTitle: "⚙",
            accessibilityLabel: "Settings",
            action: #selector(settingsClicked)
        )
        settingsButton.frame = NSRect(
            x: padding + 16, y: (bottomBarHeight - 28) / 2, width: 28, height: 28)
        settingsButton.toolTip = L10n.isZH ? "打开设置" : "Settings"
        bottomBar.addSubview(settingsButton)

        // Folder button (data directory)
        folderButton = makeIconButton(
            symbolName: "folder",
            fallbackTitle: "📁",
            accessibilityLabel: "Data Directory",
            action: #selector(folderClicked)
        )
        folderButton.frame = NSRect(
            x: padding + 16 + 36, y: (bottomBarHeight - 28) / 2, width: 28, height: 28)
        folderButton.toolTip = L10n.isZH ? "修改数据目录" : "Change Data Directory"
        bottomBar.addSubview(folderButton)

        // Quit button (power icon)
        quitButton = makeIconButton(
            symbolName: "power",
            fallbackTitle: "⏻",
            accessibilityLabel: "Quit",
            action: #selector(quitClicked)
        )
        quitButton.frame = NSRect(
            x: padding + 16 + 36 + 36, y: (bottomBarHeight - 28) / 2, width: 28, height: 28)
        quitButton.toolTip = L10n.isZH ? "退出" : "Quit"
        bottomBar.addSubview(quitButton)

        // Submit button (right side) — paper plane icon
        submitButton = NSButton(
            frame: NSRect(
                x: width - padding - 36, y: (bottomBarHeight - 28) / 2, width: 28, height: 28))
        submitButton.bezelStyle = .inline
        submitButton.isBordered = false
        submitButton.wantsLayer = true
        submitButton.target = self
        submitButton.action = #selector(submitClicked)
        if #available(macOS 11.0, *),
            let img = NSImage(systemSymbolName: "paperplane.fill", accessibilityDescription: "Send")
        {
            let config = NSImage.SymbolConfiguration(pointSize: 15, weight: .medium)
            submitButton.image = img.withSymbolConfiguration(config)
            submitButton.imagePosition = .imageOnly
            submitButton.contentTintColor = NSColor(white: 0.25, alpha: 1.0)
        } else {
            submitButton.title = "➤"
            submitButton.font = NSFont.systemFont(ofSize: 15)
        }
        submitButton.toolTip = L10n.isZH ? "发送" : "Send"
        bottomBar.addSubview(submitButton)

        view.addSubview(bottomBar)

        // --- Text input area ---
        let textAreaHeight = totalHeight - bottomBarHeight - padding * 2

        scrollView = NSScrollView(
            frame: NSRect(
                x: padding, y: bottomBarHeight + padding,
                width: width - padding * 2, height: textAreaHeight
            ))
        scrollView.hasVerticalScroller = true
        scrollView.hasHorizontalScroller = false
        scrollView.autohidesScrollers = true
        scrollView.borderType = .noBorder
        scrollView.drawsBackground = false

        textView = ShortcutTextView(
            frame: NSRect(
                x: 0, y: 0,
                width: scrollView.contentSize.width,
                height: scrollView.contentSize.height
            ))

        // Wire up submit shortcut callback
        (textView as! ShortcutTextView).onSubmit = { [weak self] in
            self?.submitClicked()
        }
        textView.minSize = NSSize(width: 0, height: scrollView.contentSize.height)
        textView.maxSize = NSSize(
            width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = false
        textView.autoresizingMask = [.width]
        textView.textContainer?.containerSize = NSSize(
            width: scrollView.contentSize.width,
            height: CGFloat.greatestFiniteMagnitude
        )
        textView.textContainer?.widthTracksTextView = true
        textView.textContainer?.lineFragmentPadding = 0
        textView.textContainerInset = NSSize(width: textHorizontalInset, height: textVerticalInset)
        textView.font = NSFont.systemFont(ofSize: 14)
        textView.textColor = NSColor(white: 0.15, alpha: 1.0)
        textView.backgroundColor = .clear
        textView.isRichText = false
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.isAutomaticTextReplacementEnabled = false
        textView.allowsUndo = true
        textView.delegate = self
        textView.insertionPointColor = NSColor(white: 0.3, alpha: 1.0)

        scrollView.documentView = textView
        view.addSubview(scrollView)

        // Placeholder
        placeholderLabel = NSTextField(
            labelWithString: L10n.isZH ? "记录一个碎片想法..." : "Capture a fleeting thought...")
        placeholderLabel.font = NSFont.systemFont(ofSize: 14)
        placeholderLabel.textColor = NSColor(white: 0.70, alpha: 1.0)
        placeholderLabel.frame = NSRect(
            x: scrollView.frame.minX + textHorizontalInset,
            y: scrollView.frame.maxY - textVerticalInset
                - placeholderLabel.intrinsicContentSize.height,
            width: scrollView.frame.width - textHorizontalInset * 2,
            height: placeholderLabel.intrinsicContentSize.height
        )
        placeholderLabel.isEditable = false
        placeholderLabel.isBezeled = false
        placeholderLabel.drawsBackground = false
        view.addSubview(placeholderLabel)
    }

    private func makeIconButton(
        symbolName: String, fallbackTitle: String, accessibilityLabel: String, action: Selector
    ) -> NSButton {
        let btn = NSButton(frame: .zero)
        btn.bezelStyle = .inline
        btn.isBordered = false
        btn.target = self
        btn.action = action
        btn.setAccessibilityLabel(accessibilityLabel)

        if #available(macOS 11.0, *),
            let img = NSImage(
                systemSymbolName: symbolName, accessibilityDescription: accessibilityLabel)
        {
            let config = NSImage.SymbolConfiguration(pointSize: 14, weight: .regular)
            btn.image = img.withSymbolConfiguration(config)
            btn.imagePosition = .imageOnly
            btn.contentTintColor = NSColor(white: 0.40, alpha: 1.0)
        } else {
            btn.title = fallbackTitle
            btn.font = NSFont.systemFont(ofSize: 14)
        }

        return btn
    }

    func updateStatusDot(isRunning: Bool, isStarting: Bool) {
        guard statusDot != nil else { return }
        if isStarting {
            statusDot.layer?.backgroundColor = NSColor.systemOrange.cgColor
        } else if isRunning {
            statusDot.layer?.backgroundColor = NSColor.systemGreen.cgColor
        } else {
            statusDot.layer?.backgroundColor = NSColor(white: 0.75, alpha: 1.0).cgColor
        }
    }

    // MARK: - NSTextViewDelegate

    func textDidChange(_ notification: Notification) {
        placeholderLabel.isHidden = !textView.string.isEmpty
    }

    func focusTextView() {
        guard isViewLoaded else { return }
        view.window?.makeFirstResponder(textView)
    }

    /// Update the submit shortcut from config string (e.g. "Ctrl+Enter")
    func updateSubmitShortcut(_ shortcut: String) {
        guard !shortcut.isEmpty else { return }
        (textView as? ShortcutTextView)?.submitShortcut = ParsedShortcut.parse(shortcut)
    }

    // MARK: - Actions

    @objc private func submitClicked() {
        let content = textView.string.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !content.isEmpty else { return }
        guard let delegate = appDelegate, delegate.isServerRunning else {
            NSSound.beep()
            return
        }

        // POST to server API
        let urlString = "http://127.0.0.1:\(delegate.serverPort)/api/notes"
        guard let url = URL(string: urlString) else { return }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let body: [String: Any] = ["content": content]
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)

        submitButton.isEnabled = false
        URLSession.shared.dataTask(with: request) { [weak self] _, response, error in
            DispatchQueue.main.async {
                guard let self = self else { return }
                self.submitButton.isEnabled = true
                if let httpResp = response as? HTTPURLResponse,
                    (200...299).contains(httpResp.statusCode)
                {
                    self.textView.string = ""
                    self.placeholderLabel.isHidden = false
                    self.showToast(success: true, message: L10n.isZH ? "已保存" : "Saved")
                    // Auto-close popover after a short delay
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                        self.appDelegate?.closePopover()
                    }
                } else {
                    let errMsg = error?.localizedDescription ?? "unknown error"
                    NSSound.beep()
                    self.showToast(
                        success: false, message: L10n.isZH ? "保存失败: \(errMsg)" : "Failed: \(errMsg)"
                    )
                    NSLog("Submit failed: %@", errMsg)
                    // Keep user input so they can retry
                }
            }
        }.resume()
    }

    private func showToast(success: Bool, message: String) {
        let toast = NSTextField(labelWithString: message)
        toast.font = NSFont.systemFont(ofSize: 12, weight: .medium)
        toast.textColor = .white
        toast.alignment = .center
        toast.backgroundColor = success ? NSColor.systemGreen : NSColor.systemRed
        toast.drawsBackground = true
        toast.wantsLayer = true
        toast.layer?.cornerRadius = 6
        toast.layer?.masksToBounds = true

        let textSize = toast.intrinsicContentSize
        let toastW = textSize.width + 24
        let toastH: CGFloat = 28
        let viewW = view.bounds.width
        toast.frame = NSRect(
            x: (viewW - toastW) / 2,
            y: view.bounds.height - toastH - 8,
            width: toastW,
            height: toastH
        )
        toast.alphaValue = 0
        view.addSubview(toast)

        NSAnimationContext.runAnimationGroup({ ctx in
            ctx.duration = 0.2
            toast.animator().alphaValue = 1
        })

        DispatchQueue.main.asyncAfter(deadline: .now() + (success ? 0.6 : 3.0)) {
            NSAnimationContext.runAnimationGroup(
                { ctx in
                    ctx.duration = 0.3
                    toast.animator().alphaValue = 0
                },
                completionHandler: {
                    toast.removeFromSuperview()
                })
        }
    }

    @objc private func settingsClicked() {
        appDelegate?.openDashboard()
    }

    @objc private func folderClicked() {
        appDelegate?.chooseDataDirectory()
    }

    @objc private func quitClicked() {
        appDelegate?.quitApp()
    }

    private enum L10n {
        static let isZH: Bool = {
            let lang = Locale.preferredLanguages.first ?? "en"
            return lang.hasPrefix("zh")
        }()
    }
}

// MARK: - App Delegate

class AppDelegate: NSObject, NSApplicationDelegate {

    // MARK: - Localization

    private enum L10n {
        static let isZH: Bool = {
            let lang = Locale.preferredLanguages.first ?? "en"
            return lang.hasPrefix("zh")
        }()

        static func t(_ en: String, _ zh: String) -> String {
            isZH ? zh : en
        }

        static var startFailed: String { t("Start Failed", "启动失败") }
        static var binaryNotFound: String {
            t(
                "Cannot find the tfo-desktop binary.\n\nSearched:\n1. Bundle Resources\n2. Directory next to Bundle\n3. System PATH",
                "找不到 tfo-desktop 二进制文件。\n\n已搜索:\n1. Bundle Resources\n2. Bundle 同级目录\n3. 系统 PATH"
            )
        }
        static var startError: String {
            t("Failed to start TFO service:", "无法启动 TFO 服务:")
        }
        static var ok: String { t("OK", "确定") }
    }

    // MARK: - Properties
    private var statusItem: NSStatusItem!
    private var popover: NSPopover!
    private var popoverVC: PopoverViewController!
    private var serverProcess: Process?
    fileprivate(set) var serverPort: Int = 8080
    fileprivate(set) var isServerRunning = false
    private var isServerStarting = false
    private var startupProbeID = UUID()
    private var localEventMonitor: Any?
    private var hotkeyRef: EventHotKeyRef?

    // MARK: - Lifecycle

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
        setupStatusItem()
        setupPopover()
        registerGlobalHotkey()
        startGoServer(autoOpenDashboard: false)
    }

    func applicationWillTerminate(_ notification: Notification) {
        stopGoServer()
        if let monitor = localEventMonitor {
            NSEvent.removeMonitor(monitor)
        }
        if let ref = hotkeyRef {
            UnregisterEventHotKey(ref)
        }
    }

    func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
        return true
    }

    // MARK: - Global Hotkey (Option+Shift+F) via Carbon RegisterEventHotKey
    // Carbon hotkeys work in sandboxed apps without Accessibility permission.

    private func registerGlobalHotkey() {
        // Install a Carbon event handler for hotkey events.
        let hotKeyID = EventHotKeyID(
            signature: OSType(0x5446_4F48),  // 'TFOH'
            id: 1
        )

        var eventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard),
            eventKind: UInt32(kEventHotKeyPressed)
        )

        // The handler callback — calls togglePopover on the AppDelegate.
        let handler: EventHandlerUPP = { _, event, userData -> OSStatus in
            guard let userData = userData else { return OSStatus(eventNotHandledErr) }
            let appDelegate = Unmanaged<AppDelegate>.fromOpaque(userData).takeUnretainedValue()
            DispatchQueue.main.async {
                appDelegate.togglePopover()
            }
            return noErr
        }

        let selfPtr = Unmanaged.passUnretained(self).toOpaque()
        InstallEventHandler(
            GetApplicationEventTarget(),
            handler,
            1,
            &eventType,
            selfPtr,
            nil
        )

        // Register Option+Shift+F (keycode 3 = 'F')
        let modifiers: UInt32 = UInt32(optionKey | shiftKey)
        var hotKeyRefTemp: EventHotKeyRef?
        let status = RegisterEventHotKey(
            UInt32(kVK_ANSI_F),
            modifiers,
            hotKeyID,
            GetApplicationEventTarget(),
            0,
            &hotKeyRefTemp
        )

        if status == noErr {
            hotkeyRef = hotKeyRefTemp
            NSLog("Global hotkey Option+Shift+F registered successfully")
        } else {
            NSLog("Failed to register global hotkey, status: %d", status)
        }

        // Local monitor for when the app is focused (Carbon hotkey also fires, but
        // this lets us swallow the event from the responder chain).
        localEventMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) {
            [weak self] event in
            let flags = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
            let required: NSEvent.ModifierFlags = [.option, .shift]
            if flags == required,
                event.charactersIgnoringModifiers?.lowercased() == "f"
            {
                self?.togglePopover()
                return nil  // swallow
            }
            return event
        }
    }

    // MARK: - Status Bar & Popover Setup

    private func setupStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)

        if let button = statusItem.button {
            button.image = loadStatusBarImage()
            if button.image == nil {
                button.title = "📝"
            }
            button.toolTip = "TFO"
            button.action = #selector(togglePopover)
            button.target = self
        }
    }

    private func setupPopover() {
        popoverVC = PopoverViewController()
        popoverVC.appDelegate = self

        popover = NSPopover()
        popover.contentSize = NSSize(width: 360, height: 280)
        popover.behavior = .transient
        popover.animates = true
        popover.contentViewController = popoverVC
    }

    private func loadStatusBarImage() -> NSImage? {
        let bundle = Bundle.main

        for name in ["tray", "AppIcon", "icon"] {
            if let image = bundle.image(forResource: name) {
                image.size = NSSize(width: 18, height: 18)
                image.isTemplate = false
                return image
            }
            for ext in ["png", "pdf", "icns"] {
                if let path = bundle.path(forResource: name, ofType: ext),
                    let image = NSImage(contentsOfFile: path)
                {
                    image.size = NSSize(width: 18, height: 18)
                    image.isTemplate = false
                    return image
                }
            }
        }

        if #available(macOS 11.0, *) {
            if let image = NSImage(
                systemSymbolName: "note.text", accessibilityDescription: "TFO")
            {
                image.size = NSSize(width: 18, height: 18)
                return image
            }
        }

        return nil
    }

    @objc private func togglePopover() {
        guard let button = statusItem.button else { return }
        if popover.isShown {
            popover.performClose(nil)
        } else {
            popoverVC.updateStatusDot(isRunning: isServerRunning, isStarting: isServerStarting)
            popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
            NSApp.activate(ignoringOtherApps: true)
            DispatchQueue.main.async { [weak self] in
                self?.popoverVC.focusTextView()
            }
        }
    }

    func closePopover() {
        if popover.isShown {
            popover.performClose(nil)
        }
    }

    private func updatePopoverStatus() {
        popoverVC?.updateStatusDot(isRunning: isServerRunning, isStarting: isServerStarting)
    }

    // MARK: - Go Server Management

    private func startGoServer(autoOpenDashboard: Bool) {
        guard let binaryPath = serverBinaryPath() else {
            NSLog("ERROR: Cannot find tfo-desktop binary in bundle or PATH")
            showAlert(
                title: L10n.startFailed,
                message: L10n.binaryNotFound)
            return
        }

        NSLog("Found tfo-desktop binary at: %@", binaryPath)

        startupProbeID = UUID()
        isServerStarting = true
        isServerRunning = false
        updatePopoverStatus()

        let dataDir = tfoDataDir()
        ensureDirectory(at: dataDir)

        let process = Process()
        process.executableURL = URL(fileURLWithPath: binaryPath)

        var env = ProcessInfo.processInfo.environment
        env["TFO_DATA_DIR"] = dataDir
        if let portStr = env["PORT"] {
            serverPort = Int(portStr) ?? 8080
        }
        process.environment = env

        let logFile = dataDir + "/tfo-server.log"
        FileManager.default.createFile(atPath: logFile, contents: nil)
        if let fileHandle = FileHandle(forWritingAtPath: logFile) {
            fileHandle.seekToEndOfFile()
            process.standardOutput = fileHandle
            process.standardError = fileHandle
        }

        process.terminationHandler = { [weak self] proc in
            DispatchQueue.main.async {
                guard let self = self else { return }
                if self.isServerRunning || self.isServerStarting {
                    self.isServerStarting = false
                    self.isServerRunning = false
                    self.updatePopoverStatus()
                    NSLog(
                        "TFO server terminated unexpectedly with code %d",
                        proc.terminationStatus)
                }
            }
        }

        do {
            try process.run()
            serverProcess = process
            scheduleServerReadinessCheck(autoOpenDashboard: autoOpenDashboard)
            NSLog(
                "TFO server started (PID: %d, port: %d)", process.processIdentifier, serverPort
            )
        } catch {
            isServerStarting = false
            NSLog("ERROR starting TFO: %@", error.localizedDescription)
            updatePopoverStatus()
            showAlert(
                title: L10n.startFailed,
                message: "\(L10n.startError)\n\(error.localizedDescription)")
        }
    }

    private func stopGoServer() {
        startupProbeID = UUID()
        guard let process = serverProcess, process.isRunning else { return }
        isServerStarting = false
        isServerRunning = false
        updatePopoverStatus()
        process.terminate()
        DispatchQueue.global().async {
            process.waitUntilExit()
        }
        serverProcess = nil
        NSLog("TFO server stopped")
    }

    // MARK: - Config Sync

    private func fetchAndApplyConfig() {
        guard let url = URL(string: "http://127.0.0.1:\(serverPort)/api/config") else { return }
        URLSession.shared.dataTask(with: url) { [weak self] data, _, error in
            guard let data = data, error == nil else { return }
            guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                let hotkeySave = json["hotkeySave"] as? String, !hotkeySave.isEmpty
            else { return }
            DispatchQueue.main.async {
                self?.popoverVC.updateSubmitShortcut(hotkeySave)
                NSLog("Submit shortcut updated to: %@", hotkeySave)
            }
        }.resume()
    }

    // MARK: - Actions (exposed for PopoverVC)

    @objc func openDashboard() {
        let url = URL(string: "http://127.0.0.1:\(serverPort)")!
        NSWorkspace.shared.open(url)
    }

    @objc func quitApp() {
        stopGoServer()
        NSApp.terminate(nil)
    }

    func chooseDataDirectory() {
        closePopover()

        let alert = NSAlert()
        alert.messageText = L10n.isZH ? "修改数据目录" : "Change Data Directory"
        alert.informativeText =
            L10n.isZH
            ? "您正在修改数据目录，选择新目录后将重启服务。\n当前目录: \(tfoDataDir())"
            : "You are changing the data directory. The service will restart after selection.\nCurrent: \(tfoDataDir())"
        alert.alertStyle = .informational
        alert.addButton(withTitle: L10n.isZH ? "选择目录" : "Choose Folder")
        alert.addButton(withTitle: L10n.isZH ? "取消" : "Cancel")

        NSApp.activate(ignoringOtherApps: true)
        let response = alert.runModal()
        guard response == .alertFirstButtonReturn else { return }

        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.canCreateDirectories = true
        panel.allowsMultipleSelection = false
        panel.prompt = L10n.isZH ? "选择" : "Choose"
        panel.message = L10n.isZH ? "请选择 TFO 数据保存目录" : "Choose a folder to store TFO data"

        let result = panel.runModal()
        guard result == .OK, let url = panel.url else { return }

        let newDir = url.path
        NSLog("User chose new data directory: %@", newDir)

        // Persist via bookmark for sandbox access
        saveSecurityBookmark(for: url)
        customDataDir = newDir
        UserDefaults.standard.set(newDir, forKey: "TFOCustomDataDir")

        // Restart server with new directory
        stopGoServer()
        startGoServer(autoOpenDashboard: false)
    }

    // MARK: - Helpers

    private func serverBinaryPath() -> String? {
        if let bundledPath = Bundle.main.path(forResource: "tfo-desktop", ofType: nil) {
            if FileManager.default.isExecutableFile(atPath: bundledPath) {
                return bundledPath
            }
        }
        let appDir = Bundle.main.bundlePath
        let devPath = (appDir as NSString).deletingLastPathComponent + "/tfo-desktop"
        if FileManager.default.isExecutableFile(atPath: devPath) {
            return devPath
        }
        let sameDirPaths = [
            (appDir as NSString).deletingLastPathComponent + "/macos_arm64/tfo-desktop",
            (appDir as NSString).deletingLastPathComponent + "/macos_amd64/tfo-desktop",
            (appDir as NSString).deletingLastPathComponent + "/macos_universal/tfo-desktop",
        ]
        for p in sameDirPaths {
            if FileManager.default.isExecutableFile(atPath: p) {
                return p
            }
        }
        let whichProcess = Process()
        whichProcess.executableURL = URL(fileURLWithPath: "/usr/bin/which")
        whichProcess.arguments = ["tfo-desktop"]
        let pipe = Pipe()
        whichProcess.standardOutput = pipe
        whichProcess.standardError = FileHandle.nullDevice
        try? whichProcess.run()
        whichProcess.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        if let path = String(data: data, encoding: .utf8)?.trimmingCharacters(
            in: .whitespacesAndNewlines),
            !path.isEmpty
        {
            return path
        }
        return nil
    }

    private var customDataDir: String? = UserDefaults.standard.string(forKey: "TFOCustomDataDir")

    private var isSandboxed: Bool {
        ProcessInfo.processInfo.environment["APP_SANDBOX_CONTAINER_ID"] != nil
    }

    fileprivate func tfoDataDir() -> String {
        // If user has chosen a custom directory, restore bookmark and use it
        if let custom = customDataDir, !custom.isEmpty {
            restoreSecurityBookmark()
            return custom
        }
        let home = FileManager.default.homeDirectoryForCurrentUser
        if isSandboxed {
            let appSupport = home.appendingPathComponent("Library/Application Support/TFO")
            return appSupport.path
        }
        return home.appendingPathComponent(".tfo").path
    }

    private func saveSecurityBookmark(for url: URL) {
        do {
            let bookmarkData = try url.bookmarkData(
                options: .withSecurityScope,
                includingResourceValuesForKeys: nil,
                relativeTo: nil
            )
            UserDefaults.standard.set(bookmarkData, forKey: "TFODataDirBookmark")
        } catch {
            NSLog("Failed to save security bookmark: %@", error.localizedDescription)
        }
    }

    private func restoreSecurityBookmark() {
        guard let bookmarkData = UserDefaults.standard.data(forKey: "TFODataDirBookmark") else {
            return
        }
        do {
            var isStale = false
            let url = try URL(
                resolvingBookmarkData: bookmarkData,
                options: .withSecurityScope,
                relativeTo: nil,
                bookmarkDataIsStale: &isStale
            )
            if isStale {
                saveSecurityBookmark(for: url)
            }
            _ = url.startAccessingSecurityScopedResource()
        } catch {
            NSLog("Failed to restore security bookmark: %@", error.localizedDescription)
        }
    }

    private func ensureDirectory(at path: String) {
        try? FileManager.default.createDirectory(
            atPath: path,
            withIntermediateDirectories: true,
            attributes: nil)
    }

    private func scheduleServerReadinessCheck(autoOpenDashboard: Bool) {
        let probeID = startupProbeID
        let deadline = Date().addingTimeInterval(20)

        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.pollServerReadiness(
                probeID: probeID,
                deadline: deadline,
                autoOpenDashboard: autoOpenDashboard)
        }
    }

    private func pollServerReadiness(probeID: UUID, deadline: Date, autoOpenDashboard: Bool) {
        guard probeID == startupProbeID else { return }
        guard let process = serverProcess, process.isRunning else { return }

        if isDashboardReachable() {
            DispatchQueue.main.async { [weak self] in
                guard let self = self, probeID == self.startupProbeID else { return }
                self.isServerStarting = false
                self.isServerRunning = true
                self.updatePopoverStatus()
                self.fetchAndApplyConfig()
                if autoOpenDashboard {
                    self.openDashboard()
                }
            }
            return
        }

        if Date() >= deadline {
            // Deadline passed — stop showing "starting" but keep polling in background
            // so the status can recover if the server becomes ready later.
            DispatchQueue.main.async { [weak self] in
                guard let self = self, probeID == self.startupProbeID else { return }
                if self.isServerStarting {
                    self.isServerStarting = false
                    self.updatePopoverStatus()
                    NSLog(
                        "WARN: TFO server readiness initial check timed out, continuing background polling"
                    )
                }
            }
        }

        // Keep polling as long as the process is alive (slower interval after deadline).
        let interval: TimeInterval = Date() >= deadline ? 2.0 : 0.25
        Thread.sleep(forTimeInterval: interval)
        pollServerReadiness(
            probeID: probeID, deadline: deadline, autoOpenDashboard: autoOpenDashboard)
    }

    private func isDashboardReachable() -> Bool {
        guard let url = URL(string: "http://127.0.0.1:\(serverPort)/") else {
            return false
        }

        var request = URLRequest(url: url)
        request.timeoutInterval = 2.0
        request.cachePolicy = .reloadIgnoringLocalCacheData

        let semaphore = DispatchSemaphore(value: 0)
        var reachable = false

        let task = URLSession.shared.dataTask(with: request) { _, response, error in
            if error == nil, response is HTTPURLResponse {
                reachable = true
            }
            semaphore.signal()
        }
        task.resume()

        _ = semaphore.wait(timeout: .now() + 1.0)
        return reachable
    }

    private func showAlert(title: String, message: String) {
        NSApp.activate(ignoringOtherApps: true)
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = message
        alert.alertStyle = .warning
        alert.addButton(withTitle: L10n.ok)
        alert.runModal()
    }
}
