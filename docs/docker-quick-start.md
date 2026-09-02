# 在其他电脑上启动 Alive

这份指南用于在一台新电脑上运行 Alive。目标电脑只需要安装 Docker，不需要另外安装 Go、Node.js、Nginx 或 PostgreSQL。

## 1. 安装 Docker

推荐安装 Docker Desktop：

- macOS：[Docker Desktop for Mac](https://docs.docker.com/desktop/setup/install/mac-install/)
- Windows：[Docker Desktop for Windows](https://docs.docker.com/desktop/setup/install/windows-install/)
- Linux：安装 [Docker Engine](https://docs.docker.com/engine/install/) 和 Docker Compose 插件

安装完成后启动 Docker，然后在终端执行：

```bash
docker version
docker compose version
```

两个命令都能正常输出版本信息后，再继续下面的操作。

## 2. 获取项目

可以从 GitHub 克隆：

```bash
git clone https://github.com/chosetop/Alive.git
cd Alive
```

也可以把整个 `Alive` 项目目录复制到新电脑，然后在终端进入该目录。后续命令都要在项目根目录执行，也就是能看到 `compose.yaml` 的目录。

## 3. 创建本地配置

macOS 或 Linux：

```bash
cp .env.docker.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
```

打开新建的 `.env`，把下面的占位密码换成长随机值：

```dotenv
ALIVE_DB_PASSWORD=replace-with-a-long-random-value
```

密码只使用字母、数字、连字符或下划线。不要把 `.env` 发给别人，也不要提交到 Git。

如果本机的 `8081` 端口已经被占用，可以同时修改：

```dotenv
ALIVE_PORT=18081
```

修改后，本文中的访问地址也要相应改成 `http://localhost:18081`。

## 4. 构建并启动

```bash
docker compose up -d --build
```

第一次启动会下载基础镜像并编译前后端，所需时间取决于网络和电脑性能。命令结束后查看服务状态：

```bash
docker compose ps
```

`db`、`backend`、`frontend` 和 `gateway` 应显示为 `Up` 或 `healthy`，`migrate` 显示为 `Exited (0)` 属于正常情况，它只负责执行一次数据库迁移。

macOS 或 Linux 可以继续检查接口：

```bash
curl -fsS http://localhost:8081/health/ready
```

Windows PowerShell：

```powershell
Invoke-RestMethod http://localhost:8081/health/ready
```

如果修改过 `ALIVE_PORT`，请在检查地址中使用修改后的端口。

## 5. 创建站主账号

首次安装需要创建一个后台账号：

```bash
docker compose exec backend /app/cli user:create --username lzx
```

根据终端提示输入密码。密码不会显示在屏幕上，也不会写入命令历史。

创建完成后打开：

- 前台：<http://localhost:8081/>
- 后台：<http://localhost:8081/admin/login>

## 6. 日常使用

停止项目，但保留数据库：

```bash
docker compose down
```

再次启动：

```bash
docker compose up -d
```

查看运行状态和最近日志：

```bash
docker compose ps
docker compose logs --tail=200
```

持续查看日志时使用：

```bash
docker compose logs -f --tail=200
```

拉取新代码后重新构建：

```bash
git pull
docker compose up -d --build
```

## 7. 数据说明

文章、账号和站点设置保存在 Docker 命名卷 `alive-postgres` 中。执行普通的 `docker compose down` 不会删除这些数据。

不要执行下面的命令：

```bash
docker compose down -v
```

其中的 `-v` 会删除数据库卷，文章、账号和站点设置也会一起被删除。

复制项目目录或重新克隆代码不会自动带走数据库。如果需要把旧电脑中的文章迁移到新电脑，请按照 [部署文档中的备份与恢复说明](./deployment.md#备份与恢复) 操作。

## 8. 常见问题

### 提示无法连接 Docker

确认 Docker Desktop 已经启动，再执行：

```bash
docker info
```

### 页面无法打开

先检查容器和健康状态：

```bash
docker compose ps
docker compose logs --tail=200 gateway frontend backend
```

如果 `8081` 端口被其他程序占用，在 `.env` 中修改 `ALIVE_PORT`，然后重新运行 `docker compose up -d`。

### 构建时下载失败

这通常是 Docker Hub 或 npm 网络连接中断。保持 Docker 运行，网络恢复后重新执行：

```bash
docker compose up -d --build
```

Docker 会复用已经完成的构建缓存。

### 其他电脑无法通过局域网访问

默认配置只允许当前电脑访问，这是为了避免在没有 HTTPS 的情况下暴露后台和登录 Cookie。不要直接把 `ALIVE_BIND_ADDRESS` 改成 `0.0.0.0`。需要局域网或公网访问时，请使用 [完整部署说明](./deployment.md) 中的 HTTPS 生产部署方案。
