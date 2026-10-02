param(
    [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $projectDir "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

$targets = @(
    @{ Name = "windows-amd64.exe"; OS = "windows"; Arch = "amd64" },
    @{ Name = "linux-amd64"; OS = "linux"; Arch = "amd64" },
    @{ Name = "linux-arm64"; OS = "linux"; Arch = "arm64" },
    @{ Name = "linux-armv7"; OS = "linux"; Arch = "arm"; Arm = "7" },
    @{ Name = "linux-mipsle-softfloat"; OS = "linux"; Arch = "mipsle"; Mips = "softfloat" }
)

Push-Location $projectDir
try {
    foreach ($target in $targets) {
        $env:CGO_ENABLED = "0"
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:GOARM = if ($target.Arm) { $target.Arm } else { "" }
        $env:GOMIPS = if ($target.Mips) { $target.Mips } else { "" }
        $output = Join-Path $distDir ("santak-nut-bridge-" + $target.Name)
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $output .
        Write-Host "Built $output"
    }

    $checksumLines = Get-ChildItem -Path $distDir -File -Filter "santak-nut-bridge-*" |
        Sort-Object Name |
        ForEach-Object {
            $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
            "$hash  $($_.Name)"
        }
    [System.IO.File]::WriteAllLines(
        (Join-Path $distDir "SHA256SUMS"),
        [string[]]$checksumLines,
        [System.Text.UTF8Encoding]::new($false)
    )
} finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED,Env:GOOS,Env:GOARCH,Env:GOARM,Env:GOMIPS -ErrorAction SilentlyContinue
}
