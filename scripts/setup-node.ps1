#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$NodeVersion = if ($env:NODE_VERSION) { $env:NODE_VERSION } else { '22.14.0' }
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$ToolsDir = Join-Path $Root '.tools'
$NodeDir = Join-Path $ToolsDir 'node'
$ZipPath = Join-Path $ToolsDir 'node.zip'
$ArchiveName = "node-v$NodeVersion-win-x64"

Write-Host "Freight Platform: installing portable Node.js $NodeVersion"

if (Test-Path (Join-Path $NodeDir 'node.exe')) {
  Write-Host "Node.js already installed at $NodeDir"
  & (Join-Path $NodeDir 'node.exe') --version
  & (Join-Path $NodeDir 'npm.cmd') --version
  exit 0
}

New-Item -ItemType Directory -Force -Path $ToolsDir | Out-Null
$Url = "https://nodejs.org/dist/v$NodeVersion/$ArchiveName.zip"

Write-Host "Downloading $Url"
Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing

Write-Host "Extracting..."
Expand-Archive -Path $ZipPath -DestinationPath $ToolsDir -Force
$Extracted = Join-Path $ToolsDir $ArchiveName
if (Test-Path $Extracted) {
  if (Test-Path $NodeDir) { Remove-Item $NodeDir -Recurse -Force }
  Rename-Item $Extracted $NodeDir
}
Remove-Item $ZipPath -Force -ErrorAction SilentlyContinue

& (Join-Path $NodeDir 'node.exe') --version
& (Join-Path $NodeDir 'npm.cmd') --version
Write-Host "Done. Node.js installed to $NodeDir"
Write-Host "Run: make install-web-admin && make run-web-admin"
