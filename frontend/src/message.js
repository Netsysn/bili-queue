// 将正文拆成纯文本和表情，不解析来自直播间的 HTML。
export function messageParts(content, emotes = {}) {
  const text = String(content || '');
  const names = Object.keys(emotes || {}).filter(Boolean).sort((a, b) => b.length - a.length);
  const parts = [];
  let offset = 0;
  while (offset < text.length) {
    let next = -1, matched = '';
    for (const name of names) {
      const index = text.indexOf(name, offset);
      if (index >= 0 && (next < 0 || index < next || (index === next && name.length > matched.length))) {
        next = index; matched = name;
      }
    }
    if (next < 0) { parts.push({ text: text.slice(offset) }); break; }
    if (next > offset) parts.push({ text: text.slice(offset, next) });
    let url = '';
    try {
      const parsed = new URL(emotes[matched]?.url || '');
      if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && /(^|\.)hdslb\.com$/i.test(parsed.hostname)) {
        parsed.protocol = 'https:';
        url = parsed.href;
      }
    } catch (_) { /* 无效表情地址保留原文 */ }
    parts.push(url ? { text: matched, url, large: matched === text } : { text: matched });
    offset = next + matched.length;
  }
  return parts;
}
