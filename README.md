# BiliQueue

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Wails-v3-DF0000?style=flat-square&logo=wails" alt="Wails">
  <img src="https://img.shields.io/badge/WebView2-Edge-0078D7?style=flat-square&logo=microsoftedge" alt="WebView2">
  <img src="https://img.shields.io/badge/Bilibili-Live-00A1D6?style=flat-square&logo=bilibili" alt="Bilibili">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
</p>

<p align="center"><b>B站直播弹幕排队助手</b> · 悬浮窗 · 主播工具 · by netsysn</p>

---

## 这是什么

主播打游戏时，桌面角落飘一个半透明悬浮窗，自动识别弹幕里的排队请求，帮主播管理观众排队队列。

**不挡游戏、不用切窗口、不用看聊天栏。**

---

## 功能

- **实时弹幕监听** — WebSocket 实时 + HTTP 轮询兜底
- **智能排队识别** — 识别"帮类型"（排队/帮帮/带带...）+ "服务器"（B服/官服...），配置灵活
- **双重队列管理** — 排队页展示当前队列，弹幕页滚动所有消息
- **勋章 & 表情 & 头像** — 粉丝勋章、用户等级、B站小表情/动态表情、@回复 全解析
- **开播状态灯** — 红点未开播 / 绿点呼吸灯开播中
- **去重 & 超时** — 同 UID 不重复入队，超时自动标灰下沉
- **深色/浅色主题** — 一键切换

---

## 怎么用

1. 下载 [BiliQueue.exe](https://github.com/netsysn/bili-queue/releases)
2. 双击运行
3. 去你的直播间看悬浮窗

| 快捷键 | 操作 |
|---|---|
| `Ctrl+E` | 完成当前排队 |
| `Ctrl+S` | 跳过当前排队 |
| `Ctrl+R` | 恢复超时排队 |

---

## 技术栈

| 层 | 技术 |
|---|---|
| 窗口框架 | [Wails v3](https://wails.io) |
| 后端 | Go · [biligo-live](https://github.com/iyear/biligo-live) · gorilla/websocket |
| 前端 | Vanilla JS · CSS · [@wailsio/runtime](https://www.npmjs.com/package/@wailsio/runtime) |
| 渲染 | Microsoft Edge WebView2 |
| 弹幕协议 | B站 Live WebSocket + HTTP gethistory |

---

## 自行编译

```powershell
# 需要 Go 1.22+ / Node.js 20+
git clone https://github.com/netsysn/bili-queue.git
cd bili-queue

cd frontend
npm install && npx vite build --mode production
cd ..

go build -o BiliQueue.exe .
```

---

## 安全

不含后门、不上传数据、不读浏览器 Cookie。详见 [SECURITY.md](./SECURITY.md)

---

## License

MIT © netsysn
