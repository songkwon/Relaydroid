param(
    [ValidateSet("amd64", "arm64")]
    [string]$Architecture = $(switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { throw "Specify -Architecture amd64 or arm64." }
    }),
    [string]$Output = "build"
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
Set-Location (Join-Path $PSScriptRoot "..")

npm ci --prefix frontend --no-audit --no-fund
npm run build --prefix frontend

$dir = Join-Path $Output "Relaydroid-windows-$Architecture"
$licenses = Join-Path $dir "Licenses"
New-Item -ItemType Directory -Force -Path $licenses | Out-Null
$binary = Join-Path $dir "Relaydroid.exe"

$version = "0.4.0"
if ($env:GITHUB_REF_NAME -match '^v([0-9]+\.[0-9]+\.[0-9]+)$') { $version = $Matches[1] }
$resource = "relaydroidicon_windows_$Architecture.syso"
if (Test-Path $resource) { throw "Remove the existing generated resource before building: $resource" }
try {
    go run github.com/tc-hib/go-winres@v0.3.3 simply --arch $Architecture --out relaydroidicon `
        --manifest gui --icon assets/Relaydroid-icon.png --product-name Relaydroid --original-filename Relaydroid.exe `
        --file-version $version --product-version $version

    $env:GOOS = "windows"
    $env:GOARCH = $Architecture
    $env:CGO_ENABLED = "0"
    $env:GOFLAGS = "-mod=readonly"
    go test ./...
    go build -trimpath -ldflags "-s -w -H=windowsgui" -o $binary .
} finally {
    if (Test-Path $resource) { Remove-Item -LiteralPath $resource }
}

$pe = [IO.File]::ReadAllBytes((Resolve-Path $binary))
$header = [BitConverter]::ToInt32($pe, 60)
$machine = [BitConverter]::ToUInt16($pe, $header + 4)
$expected = if ($Architecture -eq "amd64") { 0x8664 } else { 0xAA64 }
if ($machine -ne $expected) { throw "Unexpected PE machine type: $machine" }

Copy-Item README.md $dir
Copy-Item LICENSE (Join-Path $licenses "Relaydroid.txt")
Copy-Item assets/Android-icon-LICENSE.txt (Join-Path $licenses "Android-icon.txt")
Copy-Item (Join-Path (go env GOROOT) "LICENSE") (Join-Path $licenses "Go.txt")

go list -deps -f '{{with .Module}}{{if and (not .Main) .Dir}}{{.Path}}|{{.Dir}}{{end}}{{end}}' . |
    Sort-Object -Unique | ForEach-Object {
        if ($_ -match '^(.+)\|(.+)$') {
            $name = $Matches[1] -replace '/', '_'
            $module = $Matches[2]
            foreach ($notice in @("LICENSE", "NOTICE")) {
                $file = Join-Path $module $notice
                if (Test-Path $file) { Copy-Item $file (Join-Path $licenses "$name-$notice.txt") }
            }
        }
    }

npm ls --prefix frontend --omit=dev --all --parseable | ForEach-Object {
    if ($_ -ne (Resolve-Path frontend).Path) {
        $name = Split-Path -Leaf $_
        foreach ($notice in @("LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "NOTICE")) {
            $file = Join-Path $_ $notice
            if (Test-Path $file) { Copy-Item $file (Join-Path $licenses "npm-$name-$notice.txt") }
        }
    }
}

Compress-Archive -Path (Join-Path $dir "*") -DestinationPath "$dir.zip" -Force
Get-Item "$dir.zip"
