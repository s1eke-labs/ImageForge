# ImageForge

[English](README.md)

AI 图像生成平台，采用任务队列架构。用户提交提示词并可附带参考图片，远程 Runner 领取并执行生成任务。

## 技术栈

| 层级   | 技术                                                          |
| ------ | ------------------------------------------------------------- |
| 后端   | Go 1.24, Echo v4, GORM + SQLite, JWT 认证, Cobra CLI         |
| 前端   | React 19, Vite 6, Zustand, React Router 7, 国际化（中/英）    |
| 部署   | Docker 多阶段构建, docker-compose                             |

## 快速开始

### Docker 部署（推荐）

```bash
cp .env.example .env
# 编辑 .env，设置 IMAGEFORGE_JWT_SECRET（生成方式：openssl rand -hex 32）

docker compose up --build
```

启动后访问 `http://localhost:8020`。

### 本地开发

**后端**（使用 [Air](https://github.com/air-verse/air) 热重载）：

```bash
cd backend
go run ./cmd/server
```

**前端**：

```bash
cd frontend
npm install
npm run dev
```

前端开发服务器会将 API 请求代理到后端。生产环境下，`npm run build` 产出的静态文件会嵌入到 Go 二进制中。

## 项目结构

```
ImageForge
├── backend/           # Go API 服务 + CLI 工具
│   ├── cmd/
│   │   ├── server/    # HTTP 服务入口
│   │   └── cli/       # 命令行工具
│   └── internal/      # Handler、Model、Config、Middleware
├── frontend/          # React 单页应用
│   └── src/
├── data/              # 运行时数据（SQLite 数据库、图片）
├── docs/              # 设计文档
└── docker-compose.yml
```

## 环境变量

| 变量                  | 必填 | 说明                           |
| --------------------- | ---- | ------------------------------ |
| `IMAGEFORGE_JWT_SECRET` | 是  | JWT 签名密钥                   |
| `DATA_DIR`            | 否   | 宿主机数据目录（默认 `./data`）|
| `PORT`                | 否   | 宿主机端口映射（默认 `8020`）  |

完整配置参见 [.env.example](.env.example)。

## API 概览

- **认证** — 基于 JWT 的登录，支持速率限制
- **任务** — 创建、列表、取消图像生成任务；上传参考图片
- **Runner** — 注册、心跳、领取任务、提交结果

## 许可证

[MIT](LICENSE)
