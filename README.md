# 企微对接网关 (WeCom Gateway)

企业微信 IM 机器人对接网关，将企业微信用户消息桥接到 DATRIX 智能体平台。

## 架构概览

```
┌──────────────────────────────────────────────────────────┐
│                      企微对接网关                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │  BotManager (生命周期管理)                         │   │
│  │  ┌────────────────┐  ┌────────────────┐          │   │
│  │  │  WeComAdapter   │  │  (DingTalk预留) │          │   │
│  │  │  - Webhook接收  │  │                │          │   │
│  │  │  - AES加解密    │  │                │          │   │
│  │  │  - 消息收发     │  │                │          │   │
│  │  └───────┬────────┘  └────────────────┘          │   │
│  │          │                                        │   │
│  │  ┌───────▼────────────────────────────────┐      │   │
│  │  │  Pipeline (消息处理责任链)              │      │   │
│  │  │  Dedup → Identity → Token → Session    │      │   │
│  │  │  → Chat → Reply                        │      │   │
│  │  └────────────────┬───────────────────────┘      │   │
│  │                   │                               │   │
│  │  ┌────────────────▼───────────────────────┐      │   │
│  │  │  DatrixBridge (DATRIX后端对接)          │      │   │
│  │  │  用户查询 / 免密登录 / 智能体 / WebSocket│      │   │
│  │  └────────────────────────────────────────┘      │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  基础设施: PostgreSQL + Redis + Gin HTTP         │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

## 设计思想

本项目参照 **LangBot 框架** 的核心设计理念：

| LangBot 概念 | 本项目对应 |
|-------------|-----------|
| `Pipeline / Stage` 责任链 | `internal/pipeline/` — Dedup → Identity → Token → Session → Chat → Reply |
| `AbstractMessagePlatformAdapter` | `internal/adapter/AbstractIMAdapter` 接口 |
| `PlatformManager / RuntimeBot` | `internal/botmgr/BotManager` 单例管理器 |
| `ProviderAPIRequester` | `internal/bridge/DatrixBridge` 接口 |
| `ConfigFile / ConfigManager` | `config/config.go` viper+pflag+环境变量 |
| `EventContext` | `pipeline.Context` 流水线上下文 |

## 目录结构

```
企微对接/
├── cmd/server/main.go          # 启动入口
├── config/
│   ├── config.go               # 配置加载器
│   └── config.example.yaml     # 配置示例
├── internal/
│   ├── adapter/                # IM 平台适配器
│   │   ├── interface.go        # AbstractIMAdapter 抽象接口
│   │   ├── types.go            # 适配器类型定义
│   │   ├── wecom.go            # 企微适配器实现
│   │   └── wecom_crypto.go     # 企微加解密工具
│   ├── auth/                   # 扫码登录
│   │   └── auth_service.go     # OAuth2 扫码登录服务
│   ├── botmgr/                 # 机器人生命周期管理
│   │   └── bot_manager.go      # BotManager 单例
│   ├── bridge/                 # DATRIX 后端对接
│   │   ├── interface.go        # DatrixBridge 接口
│   │   ├── types.go            # Bridge 数据类型
│   │   └── datrix_bridge.go    # Bridge 实现
│   ├── common/                 # 全局常量/类型
│   │   ├── define.go           # 全局变量
│   │   ├── wecom_config.go     # 企微配置解析
│   │   └── datrix.go           # DATRIX 数据结构
│   ├── db/                     # 数据库/缓存
│   │   ├── database.go         # PostgreSQL + GORM
│   │   └── redis.go            # Redis + Key 前缀自动注入
│   ├── handler/                # HTTP 处理器
│   │   └── router.go           # 路由 + Webhook + 管理API + 扫码登录
│   ├── model/                  # 数据模型
│   │   ├── base.go             # 基础模型 + 全局DB
│   │   ├── wecom_app.go        # IM应用表
│   │   ├── wecom_app_assistant.go # 应用-智能体绑定表
│   │   ├── wecom_user_info.go  # IM用户信息表
│   │   └── wecom_session.go    # 会话表
│   ├── pipeline/               # 消息处理流水线
│   │   ├── interface.go        # Pipeline / Stage / Context 接口
│   │   ├── manager.go          # Pipeline 管理器
│   │   └── stages/             # 各阶段实现
│   │       ├── dedup.go        # 消息去重
│   │       ├── identity.go     # 身份映射
│   │       ├── token.go        # Token 管理
│   │       ├── session.go      # 会话管理
│   │       ├── chat.go         # AI 对话
│   │       └── reply.go        # 消息回复
│   ├── processor/              # 消息处理器
│   │   ├── message_processor.go # 主处理器
│   │   └── media_links.go      # 媒体链接转换
│   └── utils/                  # 工具函数
│       ├── crypt.go            # 加解密
│       ├── log.go              # Zap 日志
│       └── util.go             # 通用工具
├── Makefile
├── go.mod
└── README.md
```

## 快速开始

### 前置要求

- Go 1.23+
- PostgreSQL 或 MySQL
- Redis
- DATRIX 平台（Asset + Assistant 服务）

### 配置

```bash
# 复制示例配置
cp config/config.example.yaml config/config.yaml
# 编辑配置文件
vim config/config.yaml
```

### 运行

```bash
# 开发模式
make run

# 或直接使用 go run
go run cmd/server/main.go --server.mode=debug --config=config/config.yaml
```

### 编译

```bash
make build        # 编译当前平台
make build-linux  # 交叉编译 Linux
make build-windows # 交叉编译 Windows
```

## API 接口

### 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/im/api/apps` | 查询应用列表 |
| POST | `/im/api/apps` | 创建新应用 |
| PUT | `/im/api/apps/:app_id` | 更新应用配置 |
| DELETE | `/im/api/apps/:app_id` | 删除应用 |
| GET | `/im/api/apps/:app_id/assistants` | 查询智能体绑定 |
| POST | `/im/api/apps/:app_id/assistants` | 创建智能体绑定 |

### 企微 Webhook

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/im/webhook/wecom/:app_id` | URL 验证 |
| POST | `/im/webhook/wecom/:app_id` | 接收消息 |

### 扫码登录

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/im/auth/{platform}/url` | 获取授权链接 |
| GET | `/im/auth/callback` | OAuth2 回调 |
| GET | `/im/auth/status` | 轮询扫码状态 |

## 核心设计模式

### 适配器模式
所有 IM 平台适配器统一实现 `AbstractIMAdapter` 接口，上层业务无需关心平台差异。

### 责任链模式 (Pipeline)
消息处理按固定顺序经过各阶段，每个阶段处理特定职责，可中断或继续。

### 依赖反转
Pipeline 各阶段通过接口依赖外部服务，而非直接依赖具体实现，便于测试和替换。

## 参考

- [LangBot 框架](https://github.com/langbot-app/LangBot)
- [DATRIX IM 机器人接入设计文档](./docs/)
- [企微开发文档](https://developer.work.weixin.qq.com/)
