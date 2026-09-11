$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Common.ps1')

Write-Host '== 导出 Alive 数据 =='
$root = Initialize-TransferFolders
$databaseUrl = Get-DatabaseUrl
$pgDump = Find-PgTool 'pg_dump'
$pgRestore = Find-PgTool 'pg_restore'
$psql = Find-PgTool 'psql'
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$packageName = "Alive-$stamp"
$workRoot = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
$packageDir = Join-Path $workRoot $packageName
$output = Join-Path (Join-Path $root '手动备份') "$packageName.zip"
New-Item -ItemType Directory -Force -Path $packageDir | Out-Null

try {
    $dump = Join-Path $packageDir 'alive.dump'
    & $pgDump --format=custom --no-owner --no-privileges --file=$dump $databaseUrl
    if ($LASTEXITCODE -ne 0) { throw '数据库导出失败。' }
    & $pgRestore --list $dump | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '导出文件验证失败。' }

    $dbVersion = Invoke-Scalar $psql $databaseUrl 'show server_version;'
    $schemaVersion = Invoke-Scalar $psql $databaseUrl "select version::text || case when dirty then '-dirty' else '' end from schema_migrations limit 1;"
    $entryCount = [int](Invoke-Scalar $psql $databaseUrl 'select count(*) from entries where deleted_at is null;')
    $checksum = (Get-FileHash -Algorithm SHA256 $dump).Hash.ToLowerInvariant()
    $manifest = [ordered]@{
        format = 1; source_os = 'Windows'; source_computer = $env:COMPUTERNAME
        exported_at = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
        postgres_version = $dbVersion; schema_version = $schemaVersion
        active_entries = $entryCount; dump_sha256 = $checksum
    }
    $manifest | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $packageDir 'manifest.json')
    "$checksum  alive.dump" | Set-Content -Encoding ASCII (Join-Path $packageDir 'SHA256.txt')
    '请使用 Alive 项目内的一键导入脚本恢复。' | Set-Content -Encoding UTF8 (Join-Path $packageDir 'README.txt')
    if (Test-Path $output) { Remove-Item -Force $output }
    Compress-Archive -Path $packageDir -DestinationPath $output -CompressionLevel Optimal
    Write-Host "导出成功：$output"
    Write-Host "有效内容数：$entryCount；数据库迁移版本：$schemaVersion"
    Start-Process explorer.exe "/select,`"$output`""
} finally {
    Remove-Item -Recurse -Force $workRoot -ErrorAction SilentlyContinue
}
Read-Host '按回车关闭窗口'
