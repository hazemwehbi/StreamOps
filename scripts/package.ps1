$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
$root = (Get-Location).Path
New-Item -ItemType Directory -Force 'dist' | Out-Null
$outputPath = Join-Path $root 'dist/StreamOps-FR.zip'
$stream = [IO.File]::Open($outputPath, [IO.FileMode]::Create)
$zip = [IO.Compression.ZipArchive]::new($stream, [IO.Compression.ZipArchiveMode]::Create)
try {
    $entries = @('cmd','internal','migrations','tests','docs','deploy','scripts','.github','go.mod','go.sum','README.md','LICENSE','Dockerfile','compose.yaml','Makefile','.gitignore','.dockerignore','.env.example')
    foreach ($entry in $entries) {
        $item = Get-Item -LiteralPath (Join-Path $root $entry) -Force
        $files = if ($item.PSIsContainer) { Get-ChildItem -LiteralPath $item.FullName -Recurse -File -Force } else { @($item) }
        foreach ($file in $files) {
            $relative = $file.FullName.Substring($root.Length + 1).Replace('\','/')
            [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $file.FullName, "StreamOps-FR/$relative") | Out-Null
        }
    }
} finally { $zip.Dispose(); $stream.Dispose() }
Write-Host "Archive du projet créée : $outputPath"
