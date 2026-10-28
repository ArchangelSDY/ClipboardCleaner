$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$assets = Join-Path $PSScriptRoot '..\assets'

function RoundedPath([float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
    $path = New-Object Drawing.Drawing2D.GraphicsPath
    $d = 2 * $r
    $path.AddArc($x, $y, $d, $d, 180, 90)
    $path.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
    $path.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90)
    $path.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
    $path.CloseFigure()
    return ,$path
}

function FillRounded($g, $brush, $x, $y, $w, $h, $r) {
    $path = RoundedPath $x $y $w $h $r
    try { $g.FillPath($brush, $path) } finally { $path.Dispose() }
}

function DrawIcon([int]$size) {
    $scale = 4
    $canvas = New-Object Drawing.Bitmap ($size * $scale), ($size * $scale)
    $g = [Drawing.Graphics]::FromImage($canvas)
    $colors = @('#163747', '#f5fafb', '#19b5a5', '#7a969f', '#f57955', '#fff4df')
    $brushes = @($colors | ForEach-Object {
        New-Object Drawing.SolidBrush ([Drawing.ColorTranslator]::FromHtml($_))
    })
    $line = New-Object Drawing.Pen $brushes[3], 10
    $outline = New-Object Drawing.Pen $brushes[0], 5
    $divider = New-Object Drawing.Pen $brushes[0], 4
    try {
        $g.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $g.ScaleTransform(($size * $scale / 256.0), ($size * $scale / 256.0))
        FillRounded $g $brushes[0] 40 40 158 192 22
        FillRounded $g $brushes[1] 54 55 130 162 12
        FillRounded $g $brushes[2] 86 22 66 50 13
        FillRounded $g $brushes[0] 106 32 26 10 5
        $line.StartCap = $line.EndCap = [Drawing.Drawing2D.LineCap]::Round
        $g.DrawLine($line, 77, 103, 158, 103)
        $g.DrawLine($line, 77, 129, 138, 129)
        $state = $g.Save()
        $g.TranslateTransform(177, 179)
        $g.RotateTransform(-38)
        $eraser = RoundedPath -49 -24 98 48 9
        try {
            $g.SetClip($eraser)
            $g.FillRectangle($brushes[4], -49, -24, 98, 48)
            $g.FillRectangle($brushes[5], 19, -24, 30, 48)
            $g.DrawLine($divider, 19, -24, 19, 24)
            $g.ResetClip()
            $g.DrawPath($outline, $eraser)
        } finally { $eraser.Dispose(); $g.Restore($state) }
        $sparkle = [Drawing.PointF[]]@(
            (New-Object Drawing.PointF 218,78), (New-Object Drawing.PointF 223,91),
            (New-Object Drawing.PointF 236,96), (New-Object Drawing.PointF 223,101),
            (New-Object Drawing.PointF 218,114), (New-Object Drawing.PointF 213,101),
            (New-Object Drawing.PointF 200,96), (New-Object Drawing.PointF 213,91)
        )
        $g.FillPolygon($brushes[2], $sparkle)
        $g.FillEllipse($brushes[2], 217, 121, 10, 10)
        $bitmap = New-Object Drawing.Bitmap $size, $size
        $output = [Drawing.Graphics]::FromImage($bitmap)
        try {
            $output.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            $output.PixelOffsetMode = [Drawing.Drawing2D.PixelOffsetMode]::HighQuality
            $output.DrawImage($canvas, 0, 0, $size, $size)
        } finally { $output.Dispose() }
        return ,$bitmap
    } finally {
        $g.Dispose()
        $canvas.Dispose()
        $line.Dispose()
        $outline.Dispose()
        $divider.Dispose()
        foreach ($brush in $brushes) { $brush.Dispose() }
    }
}

$sizes = @(16, 20, 24, 32, 48, 64, 128, 256)
$images = @()
foreach ($size in $sizes) {
    $bitmap = DrawIcon $size
    $stream = New-Object IO.MemoryStream
    try {
        $bitmap.Save($stream, [Drawing.Imaging.ImageFormat]::Png)
        $images += ,$stream.ToArray()
        if ($size -eq 256) {
            $bitmap.Save((Join-Path $assets 'clipboard-cleaner.png'), [Drawing.Imaging.ImageFormat]::Png)
        }
    } finally { $bitmap.Dispose(); $stream.Dispose() }
}

$file = [IO.File]::Create((Join-Path $assets 'clipboard-cleaner.ico'))
$writer = New-Object IO.BinaryWriter $file
try {
    $writer.Write([uint16]0)
    $writer.Write([uint16]1)
    $writer.Write([uint16]$sizes.Count)
    $offset = 6 + 16 * $sizes.Count
    for ($i = 0; $i -lt $sizes.Count; $i++) {
        $dimension = $sizes[$i] % 256
        $writer.Write([byte]$dimension)
        $writer.Write([byte]$dimension)
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]32)
        $writer.Write([uint32]$images[$i].Length)
        $writer.Write([uint32]$offset)
        $offset += $images[$i].Length
    }
    foreach ($image in $images) { $writer.Write([byte[]]$image) }
} finally { $writer.Dispose() }
