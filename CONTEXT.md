# BiliQueue 开发上下文

## 项目概述

B站直播弹幕排队助手。桌面悬浮窗，TCP 直连 B站弹幕服务器，识别弹幕排队意图并管理队列。

- 仓库: https://github.com/Netsysn/bili-queue
- 技术栈: Go + Wails v3 + WebView2
- 入口: `main.go` → `go build -o BiliQueue.exe .`

## 架构

```
B站 TCP 2243 端口 (自研 ws.go)
    │ DANMU_MSG + 系统消息
    ▼
client.go ──→ s.msgCh ──→ processMsg ──→ queue.Manager
                                       │
HTTP gethistory (history.go) ──────────┤
HTTP 礼物轮询 (gift_poller.go) ────────┤  ← TODO: 礼物解析
                                       ▼
                              frontend (Wails 事件)
```

## 关键文件

| 文件 | 职责 |
|---|---|
| `main.go` | Wails 窗口、单实例、图标 |
| `app.go` | 服务层：消息路由、队列操作、前后端桥接 |
| `config.go` | 配置文件读写 |
| `win32.go` | Windows DWM API（专注模式去阴影） |
| `internal/danmaku/client.go` | 消息体定义 + WS 连接入口 |
| `internal/danmaku/ws.go` | **TCP 客户端**：getDanmuInfo/getConf → TCP → auth → 流式 brotli 解压 → parseMessages |
| `internal/danmaku/history.go` | HTTP gethistory 轮询 |
| `internal/danmaku/gift_poller.go` | HTTP 礼物 API 轮询（需要主播 Cookie） |
| `internal/danmaku/wbi.go` | WBI 签名（getDanmuInfo 用） |
| `internal/danmaku/avatar.go` | 头像缓存 |
| `internal/danmaku/live.go` | 开播状态 |
| `internal/intent/intent.go` | 排队意图识别（帮类型+服务器） |
| `internal/queue/manager.go` | 队列管理（四种状态、超时、去重） |
| `frontend/index.html` | 悬浮窗 UI |
| `frontend/src/main.js` | 前端逻辑 |

## TCP 连接流程 (ws.go)

1. `ResolveRoomID` → 真实房间号
2. `finger/spi` → buvid3
3. `getDanmuInfo` (WBI 签名) → token + host:port；失败降级 `getConf`
4. `net.DialTimeout("tcp", host:port)` → TCP 2243
5. auth: `{uid:0, roomid, protover:3, buvid, key:token, platform:"danmuji", type:2}`
6. 流式读取：header(16B) → brotli 解压 → streamRead → 逐子包 parseMessages

## TODO

- [ ] **SEND_GIFT 解析**：TCP 流中未抓到 SEND_GIFT cmd。C# 项目 bililive_dm 使用同样协议可收到。可能原因：brotli 库差异、B站风控、需特定 host。HTTP 礼物轮询 (`gift_poller.go`) 已实现，需要主播 Cookie 才能用。
- [ ] getDanmuInfo -352：WBI 签名已加但仍 352，目前降级到 getConf

## 构建

```powershell
cd frontend && npm install && npx vite build --mode production && cd ..
go build -o BiliQueue.exe .
```

## 配置

`config.json`（自动生成）：主题、房间号、超时、帮类型、服务器、付费模式、专注模式、Cookie
