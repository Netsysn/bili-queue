import { Window, Events, Call } from '@wailsio/runtime';
import { isNearBottom } from './scroll.js';

const SVC = 'main.AppService';
let timeoutIdx = -1;
let timeoutMins = 5;
let currentTab = 'queue';
let queuePaused = false;
let logsFollowTail = true;

function av(name) {
  if (!name) return '?';
  const ch = name.charAt(0);
  return /[a-zA-Z]/.test(ch) ? ch.toUpperCase() : ch;
}
function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function fmtTime(ts) { return ts; }

// 弹幕可以包含表情，但不能把观众输入当作可执行的 HTML。
function renderMessage(target, content) {
  const template = document.createElement('template');
  template.innerHTML = content || '';
  function append(parent, nodes) {
    for (const node of nodes) {
      if (node.nodeType === Node.TEXT_NODE) {
        parent.appendChild(document.createTextNode(node.textContent));
      } else if (node.nodeType === Node.ELEMENT_NODE && node.tagName === 'B') {
        const bold = document.createElement('b');
        append(bold, node.childNodes);
        parent.appendChild(bold);
      } else if (node.nodeType === Node.ELEMENT_NODE && node.tagName === 'IMG') {
        try {
          const url = new URL(node.getAttribute('src'));
          if (url.protocol !== 'https:' || !/(^|\.)hdslb\.com$/i.test(url.hostname)) continue;
          const img = document.createElement('img');
          img.src = url.href;
          img.referrerPolicy = 'no-referrer';
          img.style.cssText = 'width:20px;height:20px;vertical-align:middle';
          parent.appendChild(img);
        } catch (_) { /* 无效表情地址 */ }
      } else if (node.nodeType === Node.ELEMENT_NODE) {
        parent.appendChild(document.createTextNode(node.outerHTML));
      }
    }
  }
  append(target, template.content.childNodes);
}

// ── 渲染排队 ──
function renderQueue(items) {
  const page = document.getElementById('queuePage');
  const empty = document.getElementById('queueEmpty');
  page.querySelectorAll('.q-item').forEach(e => e.remove());
  timeoutIdx = -1;
  const active = items.filter(i => i.status === 0).length;
  document.getElementById('count').innerHTML = `共 <b>${active}</b> 人排队`;

  if (items.length === 0) {
    empty.style.display = '';
    return;
  }
  empty.style.display = 'none';

  items.forEach((item, idx) => {
    const div = document.createElement('div');
    div.className = 'q-item';
    if (item.is_first) div.classList.add('current');
    if (item.status === 3) { div.classList.add('timed-out'); timeoutIdx = idx; }

    let tag = '<span class="q-tag tag-wait">排队中</span>';
    if (item.status === 3) tag = '<span class="q-tag tag-timeout">超时</span>';
    else if (item.status === 1) tag = '<span class="q-tag tag-current">进行中</span>';
    else if (item.is_first) {
      // 计算倒计时
      const elapsed = item.elapsed_sec || 0;
      const remain = Math.max(0, timeoutMins * 60 - elapsed);
      const m = Math.floor(remain / 60), s = remain % 60;
      tag = `<span class="q-tag tag-current">等待 ${m}:${String(s).padStart(2,'0')}</span>`;
    }

    const face = item.avatar ? `<img src="${item.avatar.replace(/"/g,'&quot;')}" referrerpolicy="no-referrer" style="width:26px;height:26px;border-radius:50%;object-fit:cover;flex-shrink:0" onerror="this.style.display='none'">` : `<div class="q-avatar">${esc(av(item.username))}</div>`;
    let badges = '';
    if (item.medal_name) badges += `<span style="background:rgba(52,211,153,0.12);color:#34d399;font-size:8px;padding:1px 4px;border-radius:3px;margin-left:4px">${esc(item.medal_name)} ${item.medal_level}</span>`;
    if (item.user_level > 0) badges += `<span style="background:rgba(251,191,36,0.12);color:#fbbf24;font-size:8px;padding:1px 4px;border-radius:3px;margin-left:3px">UL${item.user_level}</span>`;
    // 排队第一个且不是进行中 → hover 显示开始按钮
    let startBtn = '';
    if (item.is_first && item.status === 0) {
      startBtn = `<button class="q-start-btn" onclick="startFirst()" title="开始服务">开始</button>`;
    }
    div.innerHTML = `
      ${face}
      <div class="q-info">
        <div class="q-name">${esc(item.username)}<span class="q-uid">UID:${item.uid}</span>${badges}</div>
        <div class="q-meta"><span class="ht">${esc(item.help_type||'')}</span> ${esc(item.server||'')}</div>
      </div>
      <span class="q-time">${fmtTime(item.joined_at)}</span>
      ${tag}
      ${startBtn}
      <div class="q-actions">
        ${item.status === 0 ? `<button onclick="moveQueue('${item.uid_text}', -1)" title="上移">↑</button><button onclick="moveQueue('${item.uid_text}', 1)" title="下移">↓</button>` : ''}
        ${item.status !== 1 ? `<button onclick="removeQueue('${item.uid_text}')">移除</button>` : ''}
      </div>`;
    page.appendChild(div);
  });
}

// ── 渲染弹幕日志（含分隔线） ──
function renderLogs(items) {
  const page = document.getElementById('danmakuPage');
  const empty = document.getElementById('danmakuEmpty');
  const visible = !page.classList.contains('hidden');
  const previousTop = page.scrollTop;
  if (visible) logsFollowTail = isNearBottom(previousTop, page.clientHeight, page.scrollHeight);
  page.querySelectorAll('.d-item,.d-sep').forEach(e => e.remove());

  if (items.length === 0) {
    empty.style.display = '';
    return;
  }
  empty.style.display = 'none';

  items.forEach(item => {
    // 分隔线
    if (item.is_first_new) {
      const sep = document.createElement('div');
      sep.className = 'd-sep';
      sep.innerHTML = '<span>--- 以下为本轮直播 ---</span>';
      sep.style.cssText = 'text-align:center;padding:6px 0;font-size:9px;color:var(--text-dim);border-top:1px solid var(--border);margin:4px 0';
      page.appendChild(sep);
    }

    const div = document.createElement('div');
    div.className = 'd-item' + (item.is_queue ? ' queue' : '') + (item.is_gift ? ' gift' : '');
    const face = item.avatar ? `<img src="${item.avatar.replace(/"/g,'&quot;')}" referrerpolicy="no-referrer" style="width:20px;height:20px;border-radius:50%;object-fit:cover;flex-shrink:0" onerror="this.style.display='none'">` : `<div class="d-avatar">${esc(av(item.username))}</div>`;
    let badges = '';
    if (item.medal_name) badges += `<span style="background:rgba(52,211,153,0.10);color:#34d399;font-size:8px;padding:1px 4px;border-radius:2px;margin-left:3px">${esc(item.medal_name)} ${item.medal_level}</span>`;
    if (item.user_level > 0) badges += `<span style="background:rgba(251,191,36,0.10);color:#fbbf24;font-size:8px;padding:1px 4px;border-radius:2px;margin-left:3px">UL${item.user_level}</span>`;
    div.innerHTML = `
      ${face}
      <div class="d-body">
        <div class="d-user">${esc(item.username)}<span class="d-uid">UID:${item.uid}</span>${badges}</div>
        <div class="d-msg"></div>
      </div>
      <span class="d-time">${fmtTime(item.time)}</span>`;
    const message = div.querySelector('.d-msg');
    const replyTo = (item.reply_to || '').trim();
    const content = item.content || '';
    if (replyTo && !content.trimStart().startsWith('@' + replyTo)) {
      const reply = document.createElement('b');
      reply.textContent = '@' + replyTo;
      message.append(reply, document.createTextNode(' '));
    }
    renderMessage(message, content);
    page.appendChild(div);
  });
  if (visible) page.scrollTop = logsFollowTail ? page.scrollHeight : previousTop;
}

document.getElementById('danmakuPage').addEventListener('scroll', (event) => {
  const page = event.currentTarget;
  if (!page.classList.contains('hidden')) logsFollowTail = isNearBottom(page.scrollTop, page.clientHeight, page.scrollHeight);
});

// ── Tab 切换 ──
window.switchTab = (tab) => {
  currentTab = tab;
  document.getElementById('queuePage').classList.toggle('hidden', tab !== 'queue');
  document.getElementById('danmakuPage').classList.toggle('hidden', tab !== 'danmaku');
  document.getElementById('tabQueue').classList.toggle('active', tab === 'queue');
  document.getElementById('tabDanmaku').classList.toggle('active', tab === 'danmaku');
  document.getElementById('queueShortcuts').classList.toggle('hidden', tab !== 'queue');
  document.getElementById('queueTools').classList.toggle('hidden', tab !== 'queue');
  if (tab === 'danmaku' && logsFollowTail) {
    const page = document.getElementById('danmakuPage');
    page.scrollTop = page.scrollHeight;
  }
};

// ── 快捷操作 ──
window.act = (method) => Call.ByName(SVC + '.' + method);
window.restoreTimeout = () => { if (timeoutIdx >= 0) Call.ByName(SVC + '.Restore', timeoutIdx); };

window.startFirst = async () => {
  const r = await Call.ByName(SVC + '.Start');
  if (r) {
    document.getElementById('statusText').textContent = r;
    setTimeout(() => { document.getElementById('statusText').textContent = '监听中'; }, 3000);
  }
};

document.addEventListener('keydown', (e) => {
  if (!e.ctrlKey) return;
  switch (e.key.toLowerCase()) {
    case 'b': startFirst(); break;
    case 'e': act('Complete'); break;
    case 's': act('Skip'); break;
    case 'r': restoreTimeout(); break;
    default: return;
  }
  e.preventDefault();
});

function updateQueueControls(paused) {
  queuePaused = Boolean(paused);
  const button = document.getElementById('pauseQueue');
  button.textContent = queuePaused ? '恢复接收' : '暂停接收';
  button.classList.toggle('paused', queuePaused);
}
window.toggleQueue = () => Call.ByName(SVC + '.SetQueuePaused', !queuePaused);
window.showClearQueue = () => document.getElementById('clearQueueModal').classList.remove('hidden');
window.hideClearQueue = () => document.getElementById('clearQueueModal').classList.add('hidden');
window.clearQueue = async () => { await Call.ByName(SVC + '.ClearQueue'); hideClearQueue(); };
window.showAddQueue = () => {
  ['addUID', 'addName', 'addHelp', 'addServer'].forEach(id => document.getElementById(id).value = '');
  document.getElementById('addError').textContent = '';
  document.getElementById('addQueueModal').classList.remove('hidden');
  document.getElementById('addUID').focus();
};
window.hideAddQueue = () => document.getElementById('addQueueModal').classList.add('hidden');
window.addQueue = async () => {
  const values = ['addUID', 'addName', 'addHelp', 'addServer'].map(id => document.getElementById(id).value);
  try {
    const error = await Call.ByName(SVC + '.AddQueue', ...values);
    if (error) document.getElementById('addError').textContent = error;
    else hideAddQueue();
  } catch (_) { document.getElementById('addError').textContent = '添加失败，请重试'; }
};
window.moveQueue = (uid, direction) => Call.ByName(SVC + '.MoveQueue', uid, direction);
window.removeQueue = (uid) => Call.ByName(SVC + '.RemoveQueue', uid);

// ── 窗口 ──
window.showCloseConfirm = () => document.getElementById('closeModal').classList.remove('hidden');
window.hideCloseConfirm = () => document.getElementById('closeModal').classList.add('hidden');
window.doClose = () => Call.ByName(SVC + '.Quit').catch(() => Window.Close());
window.refreshDanmaku = () => {
  Call.ByName(SVC + '.Refresh');
  document.getElementById('statusText').textContent = '刷新中...';
};
window.minimizeWin = () => Window.Minimise();

// ── 标签输入 ──
function initTags(wrapId, items) {
  const wrap = document.getElementById(wrapId);
  wrap.querySelectorAll('.tag-chip').forEach(c=>c.remove());
  (items||[]).forEach(t => addTagChip(wrap, t));
}
function addTagChip(wrap, text) {
  const chip = document.createElement('span');
  chip.className = 'tag-chip';
  chip.innerHTML = `${esc(text)}<button onclick="this.parentElement.remove()">&times;</button>`;
  wrap.insertBefore(chip, wrap.querySelector('input'));
}
function getTags(wrapId) {
  const chips = document.getElementById(wrapId).querySelectorAll('.tag-chip');
  return Array.from(chips).map(c=>c.textContent.replace('×','').trim());
}
window.tagKey = (e, wrapId) => {
  if (e.key !== 'Enter') return;
  e.preventDefault();
  const inp = e.target;
  const v = inp.value.trim();
  if (!v) return;
  addTagChip(document.getElementById(wrapId), v);
  inp.value = '';
};

// ── 设置 ──
window.showSettings = async () => {
  document.getElementById('settingsModal').classList.remove('hidden');
  document.getElementById('settingsError').textContent = '';
  try {
    const c = await Call.ByName(SVC + '.GetConfig');
    document.getElementById('cfgTheme').checked = c.theme === 'light';
    document.getElementById('cfgPayMode').checked = c.pay_mode || false;
    document.getElementById('cfgRoom').value = c.identity_code || c.room_id;
    document.getElementById('cfgTimeout').value = c.timeout_minutes;
    initTags('tagHelpTypes', c.help_types||[]);
    initTags('tagServers', c.servers||[]);
    initTags('tagGifts', c.gift_queue||[]);
    document.getElementById('cfgCookie').value = c.cookie || '';
    const fm = c.focus_mode || false;
    document.getElementById('cfgFocus').checked = fm;
    document.body.classList.toggle('focus-mode', fm);
    Call.ByName(SVC + '.SetFocusMode', fm);
  } catch(e) {}
};
window.hideSettings = () => document.getElementById('settingsModal').classList.add('hidden');
window.onThemeToggle = () => document.body.classList.toggle('light', document.getElementById('cfgTheme').checked);
window.showCookieHelp = () => {
  document.getElementById('cookieHelpModal').classList.remove('hidden');
};
window.hideCookieHelp = () => {
  document.getElementById('cookieHelpModal').classList.add('hidden');
};
window.toggleFocus = () => {
  const on = document.getElementById('cfgFocus').checked;
  document.body.classList.toggle('focus-mode', on);
  Call.ByName(SVC + '.SetFocusMode', on);
};
window.stepNum = (id, delta) => {
  const el = document.getElementById(id);
  if (!el) return;
  let v = parseInt(el.value, 10);
  if (isNaN(v)) v = parseInt(el.placeholder, 10);
  if (isNaN(v)) v = 0;
  v += delta;
  const min = parseInt(el.min, 10);
  const max = parseInt(el.max, 10);
  if (!isNaN(min) && v < min) v = min;
  if (!isNaN(max) && v > max) v = max;
  if (v < 1) v = 1;
  el.value = v;
};
window.saveSettings = async () => {
  const c = {
    theme: document.getElementById('cfgTheme').checked ? 'light' : 'dark',
    room_id: (function() { var v = document.getElementById("cfgRoom").value.trim(); return /^\d+$/.test(v) ? (Number(v) || 1926788042) : 1926788042; })(),
    identity_code: (function() { var v = document.getElementById("cfgRoom").value.trim(); return v && !/^\d+$/.test(v) ? v : ""; })(),
    pay_mode: document.getElementById('cfgPayMode').checked,
    cookie: document.getElementById('cfgCookie').value || '',
    focus_mode: document.getElementById('cfgFocus').checked,
    timeout_minutes: parseInt(document.getElementById('cfgTimeout').value) || 5,
    help_types: getTags('tagHelpTypes'),
    servers: getTags('tagServers'),
    gift_queue: getTags('tagGifts'),
  };
  const errorEl = document.getElementById('settingsError');
  errorEl.textContent = '';
  try {
    const error = await Call.ByName(SVC + '.SaveConfig', c);
    if (error) { errorEl.textContent = error; return; }
    hideSettings();
    document.getElementById('statusText').textContent = '已保存，重启后生效';
  } catch (_) { errorEl.textContent = '保存失败，请检查程序目录是否可写'; }
};

// ── 初始化 ──
(async function init() {
  // 1. 先加载主题（避免闪白/闪黑）
  try {
    const c = await Call.ByName(SVC + '.GetConfig');
    document.body.classList.toggle('light', c.theme === 'light');
    document.getElementById('cfgTheme').checked = c.theme === 'light';
  } catch(e) { /* 主题加载失败用默认深色 */ }

  // 2. 加载数据
  document.getElementById('statusText').textContent = '加载中...';
  try {
    const data = await Call.ByName(SVC + '.GetQueue');
    if (data.timeout_minutes) timeoutMins = data.timeout_minutes;
    updateQueueControls(data.queue_paused);
    renderQueue(data.queue || []);
    renderLogs(data.logs || []);
    updateLiveStatus(data.is_live, data.live_known, data.connection_status);
  } catch (e) {
    document.getElementById('statusDot').classList.remove('live');
    document.getElementById('statusText').textContent = '等待连接...';
  }
})();

// 专注模式自动滚底定时器
setInterval(() => {
  if (document.body.classList.contains('focus-mode') && !document.body.matches(':hover')) {
    const page = document.getElementById(currentTab === 'danmaku' ? 'danmakuPage' : 'queuePage');
    if (page) page.scrollTop = page.scrollHeight;
  }
}, 500);

// 监听更新
Events.On('queue:updated', (event) => {
  const data = event.data;
  if (data && data.timeout_minutes) timeoutMins = data.timeout_minutes;
  updateQueueControls(data && data.queue_paused);
  renderQueue((data && data.queue) || []);
  renderLogs((data && data.logs) || []);
  updateLiveStatus(data && data.is_live, data && data.live_known, data && data.connection_status);
});

function updateLiveStatus(isLive, liveKnown, connectionStatus) {
  const dot = document.getElementById('statusDot');
  const txt = document.getElementById('statusText');
  const connection = connectionStatus || '等待连接';
  const live = !liveKnown ? '直播状态未知' : (isLive ? '直播中' : '未开播');
  dot.classList.toggle('live', connection === '连接已建立' && liveKnown && isLive);
  txt.textContent = connection + ' · ' + live;
}
