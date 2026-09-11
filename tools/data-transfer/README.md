# Alive 数据交换工具

数据库不进入 Git。程序代码通过 Git 同步，真实数据通过带 SHA-256 校验的 ZIP 交换包在 Mac 和 Windows 之间接力。

## 目录

- `mac/导出 Alive 数据.command`：从 Mac PostgreSQL 导出交换包。
- `mac/导入 Alive 数据.command`：校验、自动备份本机数据库，然后恢复交换包。
- `windows/初始化 Alive 数据环境.ps1`：检测并安装 PostgreSQL 18，创建 `alive` 数据库和本机连接配置。
- `windows/导出 Alive 数据.ps1`：从 Windows PostgreSQL 导出交换包。
- `windows/导入 Alive 数据.ps1`：校验、自动备份本机数据库，然后恢复交换包。
- `CODEX-WINDOWS-HANDOFF.md`：交给 Windows 电脑 Codex 的操作和验收说明。

## 固定工作方式

1. 同一时间只在一台电脑编辑 Alive 数据。
2. 离开当前电脑前先导出。
3. 将 ZIP 放入另一台电脑的 `Documents/Alive数据交换/待导入`。
4. 在另一台电脑执行导入，确认成功后再开始编辑。
5. 导入是整库替换，不会合并两台电脑各自新增的内容。

交换包、`.dump` 和本地数据库连接配置均已排除在 Git 之外。OSS 对象不在数据库备份中；两台电脑需要配置同一套 OSS，或另行迁移对象。

## Mac 首次准备

赋予脚本执行权限（仓库提交会保存该权限）：

```zsh
chmod +x tools/data-transfer/mac/*.command tools/data-transfer/mac/common.zsh
```

脚本优先读取 `~/Library/Application Support/Alive/backend/.env` 的 `DATABASE_URL`，其次读取仓库 `backend/.env`，也可以临时设置 `ALIVE_DATABASE_URL`。

## Windows 首次准备

在 PowerShell 中进入项目目录，执行：

```powershell
Set-ExecutionPolicy -Scope Process Bypass
& '.\tools\data-transfer\windows\初始化 Alive 数据环境.ps1'
```

如果缺少 PostgreSQL，脚本使用 `winget` 启动 PostgreSQL 18 交互安装。安装时保留端口 `5432` 并记住 `postgres` 密码。连接配置保存在 `%APPDATA%\Alive\data-transfer.env`，不会进入 Git。

## 安全恢复

每次导入前都会在 `Documents/Alive数据交换/导入前自动备份` 创建 custom-format PostgreSQL 备份。只有该备份生成且可读取后，导入才会继续。交换包 SHA-256 不一致时不会操作数据库。

Mac 脚本会暂停并恢复当前加载的 `com.alive.backend` LaunchAgent。Windows 初次迁移时通常还没有运行 Alive；以后若将后端注册成 Windows 服务，应先停止服务再导入，完成后再启动。
