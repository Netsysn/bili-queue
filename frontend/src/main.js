import { Window, Events, Call } from '@wailsio/runtime';

const SVC = 'main.AppService';
let timeoutIdx = -1;
let timeoutMins = 5;
let currentTab = 'queue';

function av(name) {
  if (!name) return '?';
  const ch = name.charAt(0);
  return /[a-zA-Z]/.test(ch) ? ch.toUpperCase() : ch;
}
function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function fmtTime(ts) { return ts; }

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

    const face = item.avatar ? `<img src="${item.avatar.replace(/"/g,'&quot;')}" referrerpolicy="no-referrer" style="width:26px;height:26px;border-radius:50%;object-fit:cover;flex-shrink:0" onerror="this.style.display='none'">` : `<div class="q-avatar">${av(item.username)}</div>`;
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
        <div class="q-name">${esc(item.username)}<span style="color:var(--text-dim);font-size:9px;margin-left:6px">UID:${item.uid}</span>${badges}</div>
        <div class="q-meta"><span class="ht">${esc(item.help_type||'')}</span> ${esc(item.server||'')}</div>
      </div>
      <span class="q-time">${fmtTime(item.joined_at)}</span>
      ${tag}
      ${startBtn}`;
    page.appendChild(div);
  });
}

// ── 渲染弹幕日志（含分隔线） ──
function renderLogs(items) {
  const page = document.getElementById('danmakuPage');
  const empty = document.getElementById('danmakuEmpty');
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
    const face = item.avatar ? `<img src="${item.avatar.replace(/"/g,'&quot;')}" referrerpolicy="no-referrer" style="width:20px;height:20px;border-radius:50%;object-fit:cover;flex-shrink:0" onerror="this.style.display='none'">` : `<div class="d-avatar">${av(item.username)}</div>`;
    let badges = '';
    if (item.medal_name) badges += `<span style="background:rgba(52,211,153,0.10);color:#34d399;font-size:8px;padding:1px 4px;border-radius:2px;margin-left:3px">${esc(item.medal_name)} ${item.medal_level}</span>`;
    if (item.user_level > 0) badges += `<span style="background:rgba(251,191,36,0.10);color:#fbbf24;font-size:8px;padding:1px 4px;border-radius:2px;margin-left:3px">UL${item.user_level}</span>`;
    div.innerHTML = `
      ${face}
      <div class="d-body">
        <div class="d-user">${esc(item.username)}<span style="color:var(--text-dim);margin-left:6px">UID:${item.uid}</span>${badges}</div>
        <div class="d-msg">${item.content}</div>
      </div>
      <span class="d-time">${fmtTime(item.time)}</span>`;
    page.appendChild(div);
  });
  page.scrollTop = page.scrollHeight;
}

// ── Tab 切换 ──
window.switchTab = (tab) => {
  currentTab = tab;
  document.getElementById('queuePage').classList.toggle('hidden', tab !== 'queue');
  document.getElementById('danmakuPage').classList.toggle('hidden', tab !== 'danmaku');
  document.getElementById('tabQueue').classList.toggle('active', tab === 'queue');
  document.getElementById('tabDanmaku').classList.toggle('active', tab === 'danmaku');
  document.getElementById('queueShortcuts').classList.toggle('hidden', tab !== 'queue');
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

// ── 窗口 ──
window.showCloseConfirm = () => document.getElementById('closeModal').classList.remove('hidden');
window.hideCloseConfirm = () => document.getElementById('closeModal').classList.add('hidden');
window.doClose = () => Call.ByName(SVC + '.Quit').catch(() => Window.Close());
window.refreshDanmaku = () => {
  Call.ByName(SVC + '.Refresh');
  document.getElementById('statusText').textContent = '刷新中...';
  setTimeout(() => { document.getElementById('statusText').textContent = '监听中'; }, 2000);
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
  try {
    const c = await Call.ByName(SVC + '.GetConfig');
    document.getElementById('cfgTheme').checked = c.theme === 'light';
    document.getElementById('cfgPayMode').checked = c.pay_mode || false;
    document.getElementById('cfgRoom').value = c.room_id;
    document.getElementById('cfgTimeout').value = c.timeout_minutes;
    initTags('tagHelpTypes', c.help_types||[]);
    initTags('tagServers', c.servers||[]);
    initTags('tagGifts', c.gift_queue||[]);
    const fm = c.focus_mode || false;
    document.getElementById('cfgFocus').checked = fm;
    document.body.classList.toggle('focus-mode', fm);
    Call.ByName(SVC + '.SetFocusMode', fm);
  } catch(e) {}
};
window.hideSettings = () => document.getElementById('settingsModal').classList.add('hidden');
window.onThemeToggle = () => document.body.classList.toggle('light', document.getElementById('cfgTheme').checked);
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
    room_id: parseInt(document.getElementById('cfgRoom').value) || 1926788042,
    pay_mode: document.getElementById('cfgPayMode').checked,
    focus_mode: document.getElementById('cfgFocus').checked,
    timeout_minutes: parseInt(document.getElementById('cfgTimeout').value) || 5,
    help_types: getTags('tagHelpTypes'),
    servers: getTags('tagServers'),
    gift_queue: getTags('tagGifts'),
  };
  await Call.ByName(SVC + '.SaveConfig', c);
  hideSettings();
  document.getElementById('statusText').textContent = '已保存，重启后生效';
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
    renderQueue(data.queue || []);
    renderLogs(data.logs || []);
    updateLiveStatus(data.is_live, data.live_time);
  } catch (e) {
    document.getElementById('statusDot').classList.remove('live');
    document.getElementById('statusText').textContent = '等待连接...';
  }
})();

// 监听更新
Events.On('queue:updated', (event) => {
  const data = event.data;
  if (data && data.timeout_minutes) timeoutMins = data.timeout_minutes;
  renderQueue((data && data.queue) || []);
  renderLogs((data && data.logs) || []);
  updateLiveStatus(data && data.is_live, data && data.live_time);
});

function updateLiveStatus(isLive, liveTime) {
  const dot = document.getElementById('statusDot');
  const txt = document.getElementById('statusText');
  // 调试：把 isLive 直接显示出来
  if (isLive) {
    dot.classList.add('live');
    txt.textContent = liveTime ? '开播 ' + liveTime : '监听中';
  } else {
    dot.classList.remove('live');
    txt.textContent = '未开播';
  }
}
