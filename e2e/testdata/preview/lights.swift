import AppKit

let renderScale: CGFloat = 4

final class KeyWindow: NSWindow {
    override var isKeyWindow: Bool { true }
    override var isMainWindow: Bool { true }
    override var backingScaleFactor: CGFloat { renderScale }
    @objc func _hasActiveAppearance() -> Bool { true }
    @objc func _hasActiveAppearanceIgnoringKeyFocus() -> Bool { true }
    @objc func _hasKeyAppearance() -> Bool { true }
    @objc func _hasMainAppearance() -> Bool { true }
    @objc var hasKeyAppearance: Bool { true }
    @objc var hasMainAppearance: Bool { true }
}

func bitmap(_ size: NSSize, _ scale: CGFloat) -> NSBitmapImageRep {
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int((size.width * scale).rounded(.up)), pixelsHigh: Int((size.height * scale).rounded(.up)), bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    rep.size = size
    return rep
}

let args = CommandLine.arguments
let dark = args[1] == "dark"
let scale = CGFloat(Double(args[2])!)
let out = args[3]
let app = NSApplication.shared
app.setActivationPolicy(.prohibited)
let w = KeyWindow(contentRect: NSRect(x: -20000, y: -20000, width: 400, height: 200), styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView], backing: .buffered, defer: false)
w.titlebarAppearsTransparent = true
w.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
let buttons = [NSWindow.ButtonType.closeButton, .miniaturizeButton, .zoomButton].compactMap { w.standardWindowButton($0) }
let union = buttons.reduce(NSRect.null) { $0.union($1.frame) }
let large = bitmap(union.size, renderScale)
let ctx = NSGraphicsContext(bitmapImageRep: large)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = ctx
w.appearance!.performAsCurrentDrawingAppearance {
    for b in buttons {
        ctx.cgContext.saveGState()
        ctx.cgContext.translateBy(x: b.frame.minX - union.minX, y: b.frame.minY - union.minY)
        b.displayIgnoringOpacity(b.bounds, in: ctx)
        ctx.cgContext.restoreGState()
    }
}
let final = bitmap(union.size, scale)
let finalCtx = NSGraphicsContext(bitmapImageRep: final)!
NSGraphicsContext.current = finalCtx
finalCtx.imageInterpolation = .high
large.draw(in: NSRect(origin: .zero, size: union.size), from: .zero, operation: .copy, fraction: 1, respectFlipped: false, hints: [.interpolation: NSImageInterpolation.high.rawValue])
NSGraphicsContext.restoreGraphicsState()
try! final.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: out))
