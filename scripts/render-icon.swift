// Merender SVG ke PNG 1024x1024 dengan latar TRANSPARAN (qlmanage selalu memberi latar putih).
// Pemakaian: swift scripts/render-icon.swift masukan.svg keluaran.png
import AppKit

let args = CommandLine.arguments
guard args.count == 3, let img = NSImage(contentsOfFile: args[1]) else {
    FileHandle.standardError.write(Data("gagal membaca SVG\n".utf8)); exit(1)
}
let size = 1024
guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { exit(1) }
rep.size = NSSize(width: size, height: size)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSColor.clear.set()
NSRect(x: 0, y: 0, width: size, height: size).fill(using: .copy)
img.draw(in: NSRect(x: 0, y: 0, width: size, height: size))
NSGraphicsContext.restoreGraphicsState()
guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: args[2]))
