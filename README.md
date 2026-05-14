# Ingress 入群验证 Bot 使用文档

## 目录

1. [概述](#概述)
2. [快速开始](#快速开始)
3. [配置说明](#配置说明)
4. [权限管理](#权限管理)
5. [命令参考](#命令参考)
6. [图片集管理](#图片集管理)
7. [验证流程](#验证流程)
8. [部署指南](#部署指南)
9. [故障排除](#故障排除)

---

## 概述

这是一个用于 Ingress 主题 Telegram 群组的入群验证 Bot。新成员加入时需要通过图片识别验证才能获得发言权限。

### 主要功能

- 新成员自动触发验证
- 图片识别验证（从预设图片集中随机选择）
- AI 智能评估用户风险，动态调整验证难度（3–7 题）
- 防 Hash 扫描的图片合成
- 可配置的验证参数
- 防刷机制（重复加入冷却）
- 管理员权限管理

---

## 快速开始

### 1. 创建 Telegram Bot

1. 在 Telegram 中找到 [@BotFather](https://t.me/BotFather)
2. 发送 `/newbot` 创建新 Bot
3. 按提示设置 Bot 名称和用户名
4. 保存获得的 Bot Token

### 2. 获取你的 User ID

1. 启动 Bot 后，私聊发送 `/myid`
2. 记录返回的数字 ID

### 3. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
TELEGRAM_BOT_TOKEN=你的Bot_Token
BOT_ADMIN_IDS=你的User_ID
```

### 4. 启动 Bot

```bash
# 使用 Docker Compose
docker-compose up -d --build

# 或本地运行
go build -o bot ./cmd/bot
./bot
```

### 5. 配置 Bot 权限

将 Bot 添加到群组并设为管理员，需要以下权限：
- ✅ 删除消息
- ✅ 封禁用户
- ✅ 限制成员

### 6. 添加图片集

至少需要 **9 个不同的图片集**，每个集合至少 1 张图片。

```
/addset Portal
/addset Resonator
/addset XMP
/addset UltraStrike
/addset PowerCube
/addset Shield
/addset LinkAmp
/addset HeatSink
/addset MultiHack
```

然后为每个集合添加图片（回复图片发送命令）：

```
/addimage Portal
```

---

## 配置说明

### 环境变量

| 变量名 | 必填 | 默认值 | 说明 |
|--------|------|--------|------|
| `TELEGRAM_BOT_TOKEN` | ✅ | - | Telegram Bot Token |
| `BOT_ADMIN_IDS` | 推荐 | - | Bot 管理员 ID，逗号分隔 |
| `DATABASE_PATH` | ❌ | `/data/bot.db` | SQLite 数据库路径 |
| `IMAGE_CACHE_PATH` | ❌ | `/images` | 图片缓存目录 |
| `VERIFY_IMAGE_COUNT` | ❌ | `3` | 验证时显示的图片数量（未启用 AI 时的默认值） |
| `VERIFY_REQUIRED_CORRECT` | ❌ | `2` | 需要答对的数量 |
| `VERIFY_TIMEOUT_SECONDS` | ❌ | `120` | 验证超时时间（秒） |
| `VERIFY_MAX_RETRY` | ❌ | `1` | 最大重试次数 |
| `REJOIN_COOLDOWN_SECONDS` | ❌ | `300` | 重新加入冷却时间（秒） |
| `MODEL_BASE_URL` | ❌ | - | AI 模型 API 地址（启用 AI 时必填） |
| `MODEL_API_KEY` | ❌ | - | AI 模型 API Key（启用 AI 时必填） |
| `MODEL_NAME` | ❌ | - | AI 模型名称（启用 AI 时必填） |
| `MODEL_TIMEOUT_SECONDS` | ❌ | `5` | AI 请求超时时间（秒） |

### 验证参数说明

- **图片数量 (n=3)**：验证时显示 3 张图片
- **通过条件 (y=2)**：答对 2 个即可通过
- **选项数量**：9 个选项（3 个正确 + 6 个干扰）
- **超时时间**：120 秒内未完成验证将被踢出
- **重试次数**：失败后可重试 1 次

### AI 动态验证难度（可选）

启用 AI 后，Bot 会根据用户风险评估自动调整验证题目数量（3–7 题）。评估依据包括用户名模式、语言代码、是否为 Bot 等因素。

**启用方式**：配置 `MODEL_BASE_URL`、`MODEL_API_KEY`、`MODEL_NAME` 三个环境变量即可。兼容所有 OpenAI 格式的 API（OpenAI、DeepSeek、Ollama 等）。

**降级策略**：
- 未配置 AI 相关环境变量时，使用固定的 `VERIFY_IMAGE_COUNT`
- AI 服务不可达时（启动健康检查失败），自动禁用 AI 并降级
- 运行时 AI 请求失败，回退到 `VERIFY_IMAGE_COUNT`

```env
# 示例：使用 DeepSeek API
MODEL_BASE_URL=https://api.deepseek.com
MODEL_API_KEY=sk-xxxxxxxx
MODEL_NAME=deepseek-chat
MODEL_TIMEOUT_SECONDS=5

# 示例：使用本地 Ollama
MODEL_BASE_URL=http://localhost:11434
MODEL_API_KEY=ollama
MODEL_NAME=llama3
MODEL_TIMEOUT_SECONDS=10
```

---

## 权限管理

### 权限层级

Bot 有两种权限检查机制：

#### 1. Bot 管理员（全局）

- 通过 `BOT_ADMIN_IDS` 环境变量配置
- 通过 `/addadmin` 命令动态添加
- 可以在**私聊**中管理图片集
- 可以管理其他 Bot 管理员

#### 2. 群组管理员（群组内）

- Telegram 群组的管理员/群主
- 可以在**群组内**管理图片集
- 不能管理 Bot 管理员

### 配置初始管理员

**方法一：环境变量（推荐）**

在 `.env` 文件中设置：

```env
BOT_ADMIN_IDS=123456789,987654321
```

多个管理员用逗号分隔。

**方法二：命令添加**

首先需要有一个初始管理员（通过环境变量设置），然后可以用命令添加更多：

```
/addadmin 123456789
```

### 获取 User ID

1. 私聊 Bot 发送 `/myid`
2. 或使用 [@userinfobot](https://t.me/userinfobot)

### 管理员命令

| 命令 | 说明 | 权限要求 |
|------|------|----------|
| `/addadmin <user_id>` | 添加 Bot 管理员 | Bot 管理员 |
| `/removeadmin <user_id>` | 移除 Bot 管理员 | Bot 管理员 |
| `/listadmins` | 列出所有 Bot 管理员 | Bot 管理员 |

---

## 命令参考

### 通用命令

| 命令 | 说明 |
|------|------|
| `/start` | 显示欢迎信息 |
| `/help` | 显示帮助信息 |
| `/myid` | 获取你的 Telegram User ID |

### 图片集管理命令

| 命令 | 说明 | 权限要求 |
|------|------|----------|
| `/addset <label>` | 创建新图片集 | 管理员 |
| `/addimage <label>` | 添加图片到指定集合 | 管理员 |
| `/listsets` | 列出所有图片集 | 管理员 |
| `/delset <label>` | 删除图片集 | 管理员 |
| `/setstats` | 查看各集合图片数量 | 管理员 |
| `/test` | 触发测试验证 | 管理员 |

### Bot 管理员命令

| 命令 | 说明 | 权限要求 |
|------|------|----------|
| `/addadmin <user_id>` | 添加 Bot 管理员 | Bot 管理员 |
| `/removeadmin <user_id>` | 移除 Bot 管理员 | Bot 管理员 |
| `/listadmins` | 列出所有 Bot 管理员 | Bot 管理员 |

---

## 图片集管理

### 创建图片集

```
/addset Portal
```

### 添加图片

1. 在群组或私聊中发送一张图片
2. **回复**该图片，发送命令：

```
/addimage Portal
```

### 查看统计

```
/setstats
```

输出示例：
```
📊 图片集统计

• Portal: 5 张
• Resonator: 3 张
• XMP: 4 张

共 3 个图片集，12 张图片

⚠️ 需要至少 9 个图片集才能正常验证
```

### 删除图片集

```
/delset Portal
```

⚠️ 这会删除该集合下的所有图片。

### 最低要求

- 至少 **9 个不同的图片集**
- 每个集合至少 **1 张图片**
- 建议每个集合 **3-5 张图片**以增加随机性

---

## 验证流程

### 用户视角

1. 用户加入群组
2. 立即被限制发言权限
3. Bot 通过 AI 评估用户风险，确定验证题目数量（3–7 题）
4. 收到验证消息，显示合成图片
5. 依次为每张图片选择正确的标签
6. 答对 2 个以上：验证通过，恢复权限
7. 答错：
   - 首次失败：重新生成验证
   - 再次失败：被踢出群组
8. 超时未完成：被踢出群组

### 验证消息示例

```
👋 欢迎 张三！

请在 120 秒内完成验证。
图片中显示了 3 张图片（标注 1、2、3）。

请依次为每张图片选择正确的标签。

当前：请选择图片 1 的标签

[Portal] [Resonator] [XMP]
[Shield] [LinkAmp] [HeatSink]
[PowerCube] [UltraStrike] [MultiHack]
```

---

## 部署指南

### Docker 部署（推荐）

```bash
# 1. 克隆项目
git clone <repo_url>
cd ING-reCAPTCHA

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件

# 3. 启动
docker-compose up -d --build

# 4. 查看日志
docker-compose logs -f
```

### 本地运行

```bash
# 1. 安装 Go 1.21+
# 2. 安装依赖
go mod download

# 3. 配置环境变量
export TELEGRAM_BOT_TOKEN=your_token
export BOT_ADMIN_IDS=your_user_id

# 4. 运行
go run ./cmd/bot
```

### 数据持久化

Docker 部署时，以下目录会被挂载：

- `./data/` - SQLite 数据库
- `./images/` - 图片缓存

---

## 故障排除

### Bot 无响应

1. 检查 Bot Token 是否正确
2. 检查网络连接
3. 查看日志：`docker-compose logs -f`

### 验证不触发

1. 确保 Bot 是群组管理员
2. 确保 Bot 有"限制成员"权限
3. 检查是否有足够的图片集（至少 9 个）

### 权限不足

1. 群组内：确保你是群组管理员
2. 私聊中：确保你的 User ID 在 `BOT_ADMIN_IDS` 中

### 图片无法添加

1. 确保回复的是图片消息
2. 确保图片集已创建
3. 检查磁盘空间

### 获取 User ID

```
/myid
```

或使用 [@userinfobot](https://t.me/userinfobot)

### AI 功能相关

**日志中出现 "AI health check failed"**
- 检查 `MODEL_BASE_URL` 是否可访问
- 检查 `MODEL_API_KEY` 是否有效
- Bot 会自动禁用 AI 并继续运行（使用固定验证数量）

**日志中出现 "AI risk assessment failed"**
- AI 请求超时或返回错误，Bot 自动降级为固定数量
- 可尝试增大 `MODEL_TIMEOUT_SECONDS`

---

## 技术支持

如有问题，请提交 Issue 或联系管理员。
