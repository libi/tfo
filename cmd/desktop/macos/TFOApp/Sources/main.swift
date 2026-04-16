import Cocoa

// Explicit entry point — required for SwiftPM without XIB/Storyboard.
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
