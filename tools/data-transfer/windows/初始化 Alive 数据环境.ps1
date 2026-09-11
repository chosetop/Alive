$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Common.ps1')

Write-Host '== 初始化 Alive Windows 数据环境 =='
$psql = $null
try { $psql = Find-PgTool 'psql' } catch {}
if (-not $psql) {
    if (-not (Get-Command winget.exe -ErrorAction SilentlyContinue)) {
        throw '未找到 winget。请先从 Microsoft Store 安装“应用安装程序”，然后重新运行。'
    }
    Write-Host '未检测到 PostgreSQL，正在启动 PostgreSQL 18 官方安装程序。'
    Write-Host '安装时请记住 postgres 管理员密码，端口保持默认 5432。'
    & winget install --id PostgreSQL.PostgreSQL.18 --exact --interactive --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 安装未完成。' }
    $psql = Find-PgTool 'psql'
}

$password = Read-RequiredSecret '请输入安装 PostgreSQL 时设置的 postgres 密码'
$encodedPassword = [Uri]::EscapeDataString($password)
$adminUrl = "postgresql://postgres:$encodedPassword@127.0.0.1:5432/postgres?sslmode=disable"
$aliveUrl = "postgresql://postgres:$encodedPassword@127.0.0.1:5432/alive?sslmode=disable"

& $psql $adminUrl -X -v ON_ERROR_STOP=1 -qAt -c "select 1" | Out-Null
if ($LASTEXITCODE -ne 0) { throw '无法使用该密码连接 PostgreSQL。' }
$exists = (& $psql $adminUrl -X -qAt -v ON_ERROR_STOP=1 -c "select 1 from pg_database where datname='alive';").Trim()
if ($exists -ne '1') {
    & $psql $adminUrl -X -v ON_ERROR_STOP=1 -c 'create database alive;'
    if ($LASTEXITCODE -ne 0) { throw '创建 alive 数据库失败。' }
}

$configDir = Join-Path $env:APPDATA 'Alive'
New-Item -ItemType Directory -Force -Path $configDir | Out-Null
$configPath = Join-Path $configDir 'data-transfer.env'
Set-Content -Encoding UTF8 -Path $configPath -Value "DATABASE_URL=$aliveUrl"
Initialize-TransferFolders | Out-Null

Write-Host "初始化完成。数据库配置保存在：$configPath"
Write-Host '下一步请运行《导入 Alive 数据.ps1》。'
Read-Host '按回车关闭窗口'
