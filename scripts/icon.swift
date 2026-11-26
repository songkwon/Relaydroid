import AppKit
import CoreGraphics

let size = 1024
let colors = CGColorSpace(name: CGColorSpace.sRGB)!
let image = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8, bytesPerRow: size * 4,
                      space: colors, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
func color(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat) -> CGColor {
    CGColor(srgbRed: r, green: g, blue: b, alpha: 1)
}
func round(_ rect: CGRect, _ radius: CGFloat) -> CGPath {
    CGPath(roundedRect: rect, cornerWidth: radius, cornerHeight: radius, transform: nil)
}

let tile = round(CGRect(x: 80, y: 80, width: 864, height: 864), 191)
let gradient = CGGradient(colorsSpace: colors, colors: [
    color(0.20, 0.39, 0.28), color(0.09, 0.17, 0.13)
] as CFArray, locations: [0, 1])!
image.saveGState()
image.setShadow(offset: CGSize(width: 0, height: -18), blur: 32, color: CGColor(gray: 0.12, alpha: 0.25))
image.addPath(tile)
image.setFillColor(color(0.12, 0.25, 0.17))
image.fillPath()
image.restoreGState()
image.saveGState()
image.addPath(tile)
image.clip()
image.drawLinearGradient(gradient, start: CGPoint(x: 125, y: 950), end: CGPoint(x: 885, y: 80), options: [])
image.restoreGState()

image.setStrokeColor(CGColor(srgbRed: 0.66, green: 0.85, blue: 0.6, alpha: 0.21))
image.setLineWidth(3)
image.addPath(round(CGRect(x: 91, y: 91, width: 842, height: 842), 183))
image.strokePath()

func arrow(from start: CGPoint, to end: CGPoint, displacement: CGFloat, tint: CGColor) {
    image.setStrokeColor(tint)
    image.setLineWidth(55)
    image.setLineCap(.round)
    image.setLineJoin(.round)
    image.move(to: start)
    image.addLine(to: end)
    image.move(to: CGPoint(x: end.x - displacement, y: end.y + 94))
    image.addLine(to: end)
    image.addLine(to: CGPoint(x: end.x - displacement, y: end.y - 94))
    image.strokePath()
}

arrow(from: CGPoint(x: 315, y: 610), to: CGPoint(x: 708, y: 610), displacement: 94,
      tint: color(0.83, 0.96, 0.68))
arrow(from: CGPoint(x: 708, y: 414), to: CGPoint(x: 315, y: 414), displacement: -94,
      tint: color(0.98, 0.99, 0.96))

let bitmap = NSBitmapImageRep(cgImage: image.makeImage()!)
try bitmap.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
