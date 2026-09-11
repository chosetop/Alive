$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Get-TransferRoot {
    if ($env:ALIVE_TRANSFER_ROOT) { return $env:ALIVE_TRANSFER_ROOT }
    return (Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'Alive数据交换')
}

function Initialize-TransferFolders {
    $root = Get-TransferRoot
    @('待导入', '已导入', '手动备份', '导入前自动备份') | ForEach-Object {
        New-Item -ItemType Directory -Force -Path (Join-Path $root $_) | Out-Null
    }
    return $root
}

function Find-PgTool([string]$Name) {
    $command = Get-Command "$Name.exe" -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    $candidate = Get-ChildItem 'C:\Program Files\PostgreSQL\*\bin' -Filter "$Name.exe" -ErrorAction SilentlyContinue |
        Sort-Object { [version]$_.Directory.Parent.Name } -Descending | Select-Object -First 1
    if ($candidate) { return $candidate.FullName }
    throw "找不到 $Name。请先运行《初始化 Alive 数据环境.ps1》。"
}

function Get-DatabaseUrl {
    if ($env:ALIVE_DATABASE_URL) { return $env:ALIVE_DATABASE_URL }
    $config = Join-Path $env:APPDATA 'Alive\data-transfer.env'
    if (Test-Path $config) {
        $line = Get-Content $config | Where-Object { $_ -like 'DATABASE_URL=*' } | Select-Object -First 1
        if ($line) { return $line.Substring('DATABASE_URL='.Length) }
    }
    throw '没有数据库连接配置。请先运行《初始化 Alive 数据环境.ps1》。'
}

function Invoke-Scalar([string]$Psql, [string]$DatabaseUrl, [string]$Sql) {
    $value = & $Psql $DatabaseUrl -X -qAt -v ON_ERROR_STOP=1 -c $Sql
    if ($LASTEXITCODE -ne 0) { throw '数据库查询失败。' }
    return ($value | Select-Object -First 1).Trim()
}

function Read-RequiredSecret([string]$Prompt) {
    $secure = Read-Host $Prompt -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}
