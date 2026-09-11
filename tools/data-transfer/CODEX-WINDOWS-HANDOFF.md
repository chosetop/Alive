# 交给 Windows 电脑 Codex 的任务说明

请在 Windows 的 Alive 项目根目录执行本任务。不要修改业务代码，不要读取、输出或提交数据库密码及 OSS 密钥。

## 目标

完成 Windows 首次数据库环境初始化，导入 Mac 生成的 Alive 数据交换包，并验证应用能够使用恢复后的真实数据。

## 操作步骤

1. 先运行 `git status --short`，保留用户现有改动。
2. 确认项目包含 `tools/data-transfer/windows` 下的四个 PowerShell 文件。
3. 在当前 PowerShell 进程允许本地脚本：

   ```powershell
   Set-ExecutionPolicy -Scope Process Bypass
   ```

4. 执行初始化：

   ```powershell
   & '.\tools\data-transfer\windows\初始化 Alive 数据环境.ps1'
   ```

5. 如果脚本启动 PostgreSQL 18 安装器，指导用户完成安装：端口使用 `5432`，让用户自己输入并保存 `postgres` 密码。不得要求用户在聊天中发送密码。安装完成后重新执行初始化脚本。
6. 将 Mac 导出的 `Alive-*.zip` 放入 `%USERPROFILE%\Documents\Alive数据交换\待导入`。
7. 执行：

   ```powershell
   & '.\tools\data-transfer\windows\导入 Alive 数据.ps1'
   ```

8. 让用户在脚本窗口内输入 `IMPORT`。不要绕过确认，也不要跳过导入前自动备份。
9. 验证 PostgreSQL 服务正在运行；使用脚本已经保存的本机配置进行只读验证，但不要把连接 URL 输出到终端或回复中。
10. 核对 `schema_migrations` 不是 dirty，`entries` 有数据，并启动 Alive 后检查登录、文章列表、草稿、图片和音乐。

## 验收标准

- `psql`、`pg_dump`、`pg_restore` 来自 PostgreSQL 18，且可以运行。
- `%APPDATA%\Alive\data-transfer.env` 存在，但未被 Git 跟踪。
- `alive` 数据库存在。
- ZIP 的 SHA-256 校验通过。
- `Documents\Alive数据交换\导入前自动备份` 中产生了恢复前备份。
- 恢复后数据库迁移状态干净，关键记录数与交换包 `manifest.json` 一致。
- Alive 可以登录并读取真实内容。

如果任一步失败，停止继续覆盖数据库，保留交换包和导入前备份，报告准确错误。不要创建测试数据，不要运行会清理数据库的集成测试。
