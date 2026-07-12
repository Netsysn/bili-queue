# BiliQueue 安全报告

> B站直播弹幕排队助手 · by netsysn · v1.0

---

## 概述

BiliQueue 是一个**纯本地运行**的 Windows 桌面悬浮窗，用于监听 B站直播间弹幕中的排队请求，帮助主播管理观众排队队列。

**不收集数据、不上传文件、不含广告、开源可审计。**

---

## 行为清单

### 网络请求

| 目标 | 用途 | 协议 |
|---|---|---|
| `api.live.bilibili.com` | 获取直播间状态、真实房间号、WS 鉴权 token | HTTPS |
| `api.live.bilibili.com` | 轮询弹幕历史（`gethistory`） | HTTPS |
| `broadcastlv.chat.bilibili.com` | 实时弹幕 WebSocket | WSS |
| `api.bilibili.com` | 获取用户头像 URL（仅当 gethistory 未返回头像时） | HTTPS |
| `i*.hdslb.com` | 加载头像、表情图片（前端 `<img>` 渲染） | HTTPS |

**不向除 B站官方域名以外的任何服务器发送数据。不连接第三方服务器、统计服务或遥测端点。**

### 本地文件

| 文件 | 用途 | 位置 |
|---|---|---|
| `config.json` | 用户设置（主题、房间号、帮类型、超时等） | exe 同目录 |

**仅读写自身配置文件，不访问其他本地文件。**

### 系统权限

| 操作 | 用途 |
|---|---|
| 网络访问 | 拉取弹幕数据（仅 B站域名） |
| 窗口置顶 (`WS_EX_TOPMOST`) | 悬浮窗始终在游戏上方 |
| 命名互斥体 (`CreateMutex`) | 防止重复启动 |
| WebView2 | 渲染界面（Windows 10/11 系统组件） |

**不需要管理员权限。不读取键盘输入、不截屏、不注入进程。**

---

## 数据隐私

- **不收集用户数据** — 无埋点、无统计上报、无遥测
- **不需要登录** — 不使用 B站账号 Cookie，所有 API 调用均为游客模式
- **不持久化弹幕记录** — 弹幕日志仅存内存，关闭即消失
- **不读取浏览器数据** — 不访问 Cookie、历史记录、书签

---

## 依赖清单

### Go 依赖

| 库 | 版本 | 用途 | 许可证 |
|---|---|---|---|
| `github.com/wailsapp/wails/v3` | v3.0.0-alpha | 桌面窗口框架 | MIT |
| `github.com/iyear/biligo-live` | v0.1.5 | B站弹幕 WebSocket 协议 | GPL-3.0 |
| `github.com/gorilla/websocket` | latest | WebSocket 连接 | BSD-2 |
| `github.com/coder/websocket` | v1.8 | WebSocket 连接 | MIT |
| `golang.org/x/sys` | latest | Windows API 调用 | BSD-3 |

### 前端依赖

| 库 | 用途 |
|---|---|
| `@wailsio/runtime` | Wails 前后端桥接 |
| `vite` | 前端构建工具 |

### 系统依赖

| 组件 | 说明 |
|---|---|
| WebView2 | Windows 10/11 预装，未安装会自动提示下载（Microsoft Edge 组件） |

---

## 自行编译验证

```powershell
# 前置条件：Go 1.22+, Node.js 20+
git clone <repo-url>
cd bili-queue-overlay

# 构建前端
cd frontend
npm install
npx vite build --mode production
cd ..

# 编译
go build -o BiliQueue.exe .

# 运行
.\BiliQueue.exe
```

---

## 常见顾虑

**Q: 会不会窃取 B站 Cookie 或账号信息？**
A: 不会。程序使用游客模式连接 B站弹幕，不读取浏览器 Cookie，不要求登录。

**Q: 会不会监控屏幕或键盘？**
A: 不会。程序仅读取弹幕 WebSocket 和 HTTP 数据流。快捷键（Ctrl+E/S/R）由 WebView 的 `keydown` 事件处理，不安装全局键盘钩子。

**Q: 会不会偷偷上传文件？**
A: 不会。所有网络请求仅发往 `*.bilibili.com` 和 `*.hdslb.com` 域名。

**Q: 杀毒软件报警怎么办？**
A: 因为 exe 未做数字签名（Authenticode 代码签名证书需年费约 $200-400），部分杀软可能对未签名的 exe 提示风险。这是误报，可以自行编译验证或添加信任。

**Q: 为什么需要 WebView2？**
A: 用于渲染悬浮窗界面。它是 Microsoft Edge 的浏览器引擎，Windows 10（20H2+）和 Windows 11 已预装。如未安装会自动提示下载。

**Q: B站 API 会封禁吗？**
A: 不会。程序使用 B站公开的弹幕协议和游客 API，与浏览器访问直播间行为一致，不涉及逆向或破解。

---

## 联系

安全问题或疑虑，请联系 netsysn。

---

*最后更新：2026-07-12*
