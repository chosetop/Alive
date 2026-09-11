$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Common.ps1')
Add-Type -AssemblyName System.Windows.Forms

Write-Host '== 导入 Alive 数据 =='
$root = Initialize-TransferFolders
$databaseUrl = Get-DatabaseUrl
$pgDump = Find-PgTool 'pg_dump'
$pgRestore = Find-PgTool 'pg_restore'
$psql = Find-PgTool 'psql'

$dialog = New-Object System.Windows.Forms.OpenFileDialog
$dialog.Title = '选择 Alive 数据交换包'
$dialog.Filter = 'Alive 数据交换包 (*.zip)|*.zip'
$dialog.InitialDirectory = Join-Path $root '待导入'
if ($dialog.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 0 }
$package = $dialog.FileName
$workRoot = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())

try {
    Expand-Archive -Path $package -DestinationPath $workRoot
    $dump = Get-ChildItem $workRoot -Recurse -Filter alive.dump | Select-Object -First 1
    if (-not $dump) { throw '交换包中没有 alive.dump。' }
    $packageDir = $dump.Directory.FullName
    $manifestPath = Join-Path $packageDir 'manifest.json'
    $shaPath = Join-Path $packageDir 'SHA256.txt'
    if (-not (Test-Path $manifestPath) -or -not (Test-Path $shaPath)) { throw '交换包不完整。' }
    $expected = ((Get-Content $shaPath | Select-Object -First 1) -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $dump.FullName).Hash.ToLowerInvariant()
    if ($expected -ne $actual) { throw '备份校验失败，已停止导入。' }
    & $pgRestore --list $dump.FullName | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '无法读取数据库备份。' }

    Get-Content $manifestPath | Write-Host
    $confirm = Read-Host '输入 IMPORT 确认覆盖本机 Alive 数据库'
    if ($confirm -cne 'IMPORT') { throw '已取消，数据库未修改。' }

    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $safety = Join-Path (Join-Path $root '导入前自动备份') "alive-before-import-$stamp.dump"
    & $pgDump --format=custom --no-owner --no-privileges --file=$safety $databaseUrl
    if ($LASTEXITCODE -ne 0) { throw '导入前安全备份失败，已停止导入。' }
    & $pgRestore --list $safety | Out-Null
    if ($LASTEXITCODE -ne 0) { throw '安全备份验证失败，已停止导入。' }

    & $psql $databaseUrl -X -v ON_ERROR_STOP=1 -c 'select pg_terminate_backend(pid) from pg_stat_activity where datname=current_database() and pid<>pg_backend_pid();' | Out-Null
    & $pgRestore --clean --if-exists --no-owner --no-privileges --exit-on-error --dbname=$databaseUrl $dump.FullName
    if ($LASTEXITCODE -ne 0) { throw "恢复失败。本机原数据仍保存在：$safety" }

    $schemaVersion = Invoke-Scalar $psql $databaseUrl "select version::text || case when dirty then '-dirty' else '' end from schema_migrations limit 1;"
    $entryCount = Invoke-Scalar $psql $databaseUrl 'select count(*) from entries where deleted_at is null;'
    Copy-Item -Force $package (Join-Path (Join-Path $root '已导入') (Split-Path $package -Leaf))
    Write-Host "导入成功。有效内容数：$entryCount；数据库迁移版本：$schemaVersion"
    Write-Host "导入前备份：$safety"
} finally {
    Remove-Item -Recurse -Force $workRoot -ErrorAction SilentlyContinue
}
Read-Host '按回车关闭窗口'
