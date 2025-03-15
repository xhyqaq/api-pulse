# API-Pulse

API-Pulse 是一个 API 变更监控和通知工具，专为 Apifox 设计，用于实时跟踪 API 变更并发送钉钉通知。

## 功能特点

- 自动初始化并存储所有 API 信息
- 接收 Apifox 的 webhook 回调，检测 API 变更
- 对比 API 的变更，包括路径、请求体、参数和响应
- 将变更信息推送到钉钉群聊
- 支持配置化部署

## 技术栈

- 使用 Go 语言开发
- 使用 go-chi 作为 HTTP 路由框架
- 使用 go-diff 进行差异比较
- 直接调用钉钉 webhook URL 发送通知

## 快速开始

### 1. 配置文件

首先，复制示例配置文件并填写您的配置：

```bash
cp config/config.yaml config/config.local.yaml
```

编辑 `config/config.local.yaml` 文件，填写以下配置：

```yaml
server:
  port: 8080  # 服务监听端口

apifox:
  project_id: "你的项目ID"
  branch_id: "你的分支ID"
  authorization: "你的授权token"
  base_url: "https://api.apifox.com/api/v1"

dingtalk:
  webhook_url: "钉钉机器人的 webhook URL"
```

### 2. 启动服务

```bash
go build -o apipulse ./cmd/apipulse
./apipulse --config=config/config.local.yaml
```

### 3. 初始化 API 列表

启动服务后，您需要初始化 API 列表：

```bash
curl -X POST http://localhost:8080/initialize
```

### 4. 配置 Apifox Webhook

在 Apifox 项目设置中配置 webhook：

- Webhook URL: `http://你的服务器地址/webhook`
- 事件类型: 选择 `API 更新`

## API 端点

| 端点 | 方法 | 描述 |
|-----|-----|-----|
| `/health` | GET | 健康检查 |
| `/webhook` | POST | 接收 Apifox webhook 回调 |

## 工作流程

1. 服务启动时，需要调用 `/initialize` 端点初始化所有 API 列表
2. 当 API 变更时，Apifox 会发送 webhook 到 `/webhook` 端点
3. 服务接收到 webhook 后，提取 API 信息，并与存储的 API 信息进行对比
4. 如果检测到变更，服务会将变更信息发送到配置的钉钉群

## 开发指南

### 目录结构

```
├── cmd/
│   └── apipulse/       # 主程序入口
├── config/             # 配置文件
├── internal/
│   ├── apifox/         # Apifox 客户端和模型
│   ├── dingtalk/       # 钉钉通知服务
│   ├── server/         # HTTP 服务器和处理器
│   └── storage/        # API 存储服务
├── pkg/
│   └── utils/          # 通用工具
└── README.md
```

### 构建项目

```bash
go mod tidy  # 整理依赖
go build -o apipulse ./cmd/apipulse
```

## 注意事项

- 确保配置文件中的 Apifox 凭证有效
- 钉钉机器人需要配置关键词过滤（建议使用"API变更通知"）
- 服务重启后需要重新初始化 API 列表

## 许可证

MIT License 