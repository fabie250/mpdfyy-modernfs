'use strict';

const $ = (s, r) => (r || document).querySelector(s);
const $$ = (s, r) => Array.from((r || document).querySelectorAll(s));

const state = {
  path: '/',
  mask: '',
  files: [],
  view: 'files',
  loaded: false,
  grid: false,
  sortKey: 'name',
  sortDir: 1,
  query: '',
  scope: 'local',
  searching: false,
  cfg: { title: '文件空间' },
  user: { name: 'guest' },
  authenticated: false,
  preview: { list: [], index: -1, text: '', editing: false },
  shareTarget: null
};

const ICONS = {
  folder: '<path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"/>',
  file: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5"/>',
  image: '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="8.5" cy="9.5" r="1.6"/><path d="M21 16l-5.5-5.5L7 20"/>',
  video: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M10 9l5 3-5 3z"/>',
  audio: '<path d="M9 18V6l10-2v12"/><circle cx="6.5" cy="18.5" r="2.5"/><circle cx="16.5" cy="16.5" r="2.5"/>',
  pdf: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5"/><path d="M9 13h1.5a1.5 1.5 0 010 3H9v-3zM13 13v3M13 14.5h2"/>',
  zip: '<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M12 6v2M12 9v2M12 12v2M12 16h0"/>',
  code: '<path d="M9 6L4 12l5 6M15 6l5 6-5 6"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>',
  doc: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5"/><path d="M9 13h6M9 17h4"/>',
  sheet: '<rect x="4" y="4" width="16" height="16" rx="2"/><path d="M4 10h16M10 10v10M10 15h10"/>',
  slides: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M8 10l4 3-4 3z"/>',
  exe: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 9h6v6H9z"/>',
  download: '<path d="M12 4v12M7 11l5 5 5-5M5 20h14"/>',
  share: '<circle cx="6" cy="12" r="3"/><circle cx="18" cy="6" r="3"/><circle cx="18" cy="18" r="3"/><path d="M8.7 10.6l6.6-3.2M8.7 13.4l6.6 3.2"/>',
  qr: '<rect x="4" y="4" width="6" height="6"/><rect x="14" y="4" width="6" height="6"/><rect x="4" y="14" width="6" height="6"/><path d="M14 14h2v2h-2zM18 18h2v2h-2z"/>',
  edit: '<path d="M4 20h4l10-10-4-4L4 16z"/><path d="M14 6l4 4"/>',
  trash: '<path d="M4 7h16M9 7V5h6v2M6 7l1 13h10l1-13"/>',
  eye: '<path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6S2 12 2 12z"/><circle cx="12" cy="12" r="2.6"/>',
  refresh: '<path d="M20 12a8 8 0 10-2.4 5.7M20 4v6h-6"/>',
  upload: '<path d="M12 16V4M7 9l5-5 5 5M5 20h14"/>',
  prev: '<path d="M15 6l-6 6 6 6"/>',
  next: '<path d="M9 6l6 6-6 6"/>'
};

function icon(name) {
  return '<svg class="ic" viewBox="0 0 24 24">' + (ICONS[name] || ICONS.file) + '</svg>';
}

function h(tag, attrs) {
  const n = document.createElement(tag);
  if (attrs) {
    for (const k in attrs) {
      const v = attrs[k];
      if (v === undefined || v === null) continue;
      if (k === 'class') n.className = v;
      else if (k === 'html') n.innerHTML = v;
      else if (k === 'text') n.textContent = v;
      else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
      else n.setAttribute(k, v);
    }
  }
  for (let i = 2; i < arguments.length; i++) {
    const kid = arguments[i];
    if (kid === null || kid === undefined || kid === false) continue;
    if (Array.isArray(kid)) kid.forEach(x => x && n.append(x));
    else n.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  }
  return n;
}

function vpJoin(base, name) {
  if (!base || base === '/') return '/' + name;
  return base.replace(/\/+$/, '') + '/' + name;
}

function parentVP(vp) {
  const clean = vp.replace(/\/+$/, '');
  const i = clean.lastIndexOf('/');
  return i <= 0 ? '/' : clean.slice(0, i);
}

function encodePath(p) {
  const parts = (p || '/').split('/').filter(Boolean).map(encodeURIComponent);
  return '/' + parts.join('/');
}

function entryVP(f) {
  if (f.path) return vpJoin(f.path, f.name);
  return vpJoin(state.path, f.name);
}

function hasMask(mask, ch) {
  return !!mask && String(mask).toUpperCase().includes(ch);
}

const EXT_KIND = {
  jpg: 'image', jpeg: 'image', png: 'image', gif: 'image', webp: 'image', bmp: 'image', svg: 'image', ico: 'image',
  mp4: 'video', mkv: 'video', avi: 'video', mov: 'video', webm: 'video', flv: 'video', m4v: 'video', ts: 'video',
  mp3: 'audio', wav: 'audio', flac: 'audio', aac: 'audio', ogg: 'audio', m4a: 'audio', wma: 'audio',
  pdf: 'pdf',
  zip: 'zip', rar: 'zip', '7z': 'zip', tar: 'zip', gz: 'zip', bz2: 'zip', xz: 'zip',
  docx: 'doc', doc: 'doc', xlsx: 'sheet', xls: 'sheet', pptx: 'slides', ppt: 'slides',
  txt: 'code', md: 'code', log: 'code', ini: 'code', conf: 'code', csv: 'code', json: 'code', xml: 'code',
  go: 'code', py: 'code', js: 'code', html: 'code', css: 'code', sh: 'code', bat: 'code', yml: 'code', yaml: 'code',
  exe: 'exe', msi: 'exe', apk: 'exe', dmg: 'exe', deb: 'exe', rpm: 'exe', iso: 'exe', bin: 'exe'
};

function extOf(name) {
  const i = String(name).lastIndexOf('.');
  return i < 0 ? '' : String(name).slice(i + 1).toLowerCase();
}

function kindOf(f) {
  if (f.dir) return 'folder';
  return EXT_KIND[extOf(f.name)] || 'file';
}

const TEXT_EXT = ['txt', 'md', 'log', 'ini', 'conf', 'csv', 'json', 'xml', 'go', 'py', 'js', 'html', 'css', 'sh', 'bat', 'yml', 'yaml'];

function previewType(f) {
  if (!f || f.dir) return '';
  const k = kindOf(f);
  if (k === 'image' || k === 'video' || k === 'audio' || k === 'pdf') return k;
  const ext = extOf(f.name);
  if (ext === 'docx') return 'docx';
  if (ext === 'doc') return 'doc';
  if (ext === 'xlsx' || ext === 'xls') return 'sheet';
  if (ext === 'pptx' || ext === 'ppt') return 'slides';
  if (TEXT_EXT.includes(ext)) return 'text';
  return '';
}

function fmtSize(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + ' B';
  const u = ['KB', 'MB', 'GB', 'TB'];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  return (n >= 100 ? n.toFixed(0) : n.toFixed(1)) + ' ' + u[i];
}

function shortTime(s) {
  if (!s) return '';
  return String(s).replace(/^\d{4}-/, '');
}

async function api(url, opts) {
  const res = await fetch(url, Object.assign({ credentials: 'same-origin' }, opts || {}));
  const ct = res.headers.get('Content-Type') || '';
  let data = null;
  if (ct.includes('json')) { try { data = await res.json(); } catch (e) { data = null; } }
  else data = await res.text();
  if (!res.ok) {
    const msg = (data && data.error) || (typeof data === 'string' && data.trim()) || ('HTTP ' + res.status);
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

function fd(obj) {
  const f = new FormData();
  Object.keys(obj).forEach(k => f.append(k, obj[k]));
  return f;
}

/* ---------- modal ---------- */
function openModal(sel) { $(sel).classList.add('open'); }
function closeModal(sel) {
  $(sel).classList.remove('open');
  if (sel === '#confirmModal' && confirmResolve) { const r = confirmResolve; confirmResolve = null; r(false); }
  if (sel === '#promptModal' && promptResolve) { const r = promptResolve; promptResolve = null; r(null); }
}

let confirmResolve = null;
function confirmBox(title, text, okText) {
  $('#confirmTitle').textContent = title;
  $('#confirmText').textContent = text;
  $('#btnConfirm').textContent = okText || '确定';
  openModal('#confirmModal');
  return new Promise(res => { confirmResolve = res; });
}

let promptResolve = null;
function promptBox(opts) {
  $('#promptTitle').textContent = opts.title || '输入';
  $('#promptLabel').textContent = opts.label || '名称';
  $('#promptInput').value = opts.value || '';
  $('#promptHint').textContent = opts.hint || '';
  $('#promptField').hidden = !!opts.hideName;
  $('#textField').hidden = !opts.multiline;
  $('#promptText').value = opts.text || '';
  openModal('#promptModal');
  setTimeout(() => { (opts.multiline ? $('#promptText') : $('#promptInput')).focus(); }, 60);
  return new Promise(res => { promptResolve = res; });
}

/* ---------- toast ---------- */
function toast(msg, kind) {
  const t = h('div', { class: 'toast ' + (kind || '') }, msg);
  $('#toasts').append(t);
  setTimeout(() => { t.style.opacity = '0'; t.style.transition = 'opacity .25s'; }, 2100);
  setTimeout(() => t.remove(), 2500);
}

/* ---------- user ---------- */
function updateUserUI() {
  const name = state.user.name || 'guest';
  const btn = $('#userBtn');
  const av = $('#userAvatar');
  if (state.authenticated) {
    btn.classList.remove('guest');
    btn.title = '已登录：' + name + '，点击可退出';
    $('#userName').textContent = name;
    $('#userSub').textContent = '点击退出登录';
    av.textContent = name.slice(0, 1).toUpperCase();
  } else {
    btn.classList.add('guest');
    btn.title = '登录后可上传、删除和分享文件';
    $('#userName').textContent = '登录';
    $('#userSub').textContent = '游客可查看和下载';
    av.innerHTML = '<svg class="ic" viewBox="0 0 24 24"><path d="M14 4h4v16h-4M10 8l4 4-4 4M14 12H4"/></svg>';
  }
  const bar = $('#guestBar');
  if (bar) bar.hidden = state.authenticated;
}

async function refreshSession() {
  try {
    const s = await api('/chfs/session');
    state.authenticated = !!s.authenticated;
    state.user = { name: s.user || 'guest' };
  } catch (e) {
    state.authenticated = false;
    state.user = { name: 'guest' };
  }
  updateUserUI();
}

function openLogin() {
  $('#loginUser').value = state.user.name === 'guest' ? '' : state.user.name;
  $('#loginPwd').value = '';
  $('#loginHint').textContent = '';
  $('#loginHint').classList.remove('error');
  openModal('#loginModal');
  setTimeout(() => $('#loginUser').focus(), 60);
}

async function doLogin() {
  const user = $('#loginUser').value.trim();
  const pwd = $('#loginPwd').value;
  if (!user) { $('#loginHint').textContent = '请输入用户名'; return; }
  try {
    await api('/chfs/session', { method: 'POST', body: fd({ user: user, pwd: pwd }) });
    await refreshSession();
    closeModal('#loginModal');
    toast('欢迎回来，' + state.user.name, 'ok');
    if (state.view === 'files') loadFiles(state.path);
    else if (state.view === 'trash') loadTrash();
    else loadShares();
  } catch (e) {
    $('#loginHint').textContent = e.message || '登录失败，请检查用户名和密码';
    $('#loginHint').classList.add('error');
  }
}

async function doLogout() {
  if (!await confirmBox('退出登录', '确定要退出当前账号吗？', '退出')) return;
  try { await api('/chfs/session', { method: 'DELETE' }); } catch (e) {}
  await refreshSession();
  toast('已退出登录');
  if (state.view === 'files') loadFiles(state.path);
  else if (state.view === 'trash') loadTrash();
  else loadShares();
}

/* ---------- routing ---------- */
function setNav(view) {
  $$('.nav-item').forEach(a => a.classList.toggle('active', a.dataset.view === view));
}

function route() {
  const hash = location.hash.replace(/^#/, '');
  let p = '/';
  try { p = decodeURIComponent(hash || '/'); } catch (e) { p = '/'; }
  if (p === '/trash') { showTrash(); return; }
  if (p === '/shares') { showShares(); return; }
  if (!p.startsWith('/')) p = '/' + p;
  if (state.view === 'files' && state.loaded && p === state.path) return;
  loadFiles(p);
}

function showFilesPanel() {
  state.view = 'files';
  setNav('files');
  const search = $('.search');
  if (search) search.hidden = false;
  $('#fileTools').hidden = false;
  $('#trashView').hidden = true;
  $('#sharesView').hidden = true;
  $('#panelHead').hidden = true;
  $('#loadingState').hidden = true;
  $('#emptyState').hidden = true;
}

async function loadFiles(p) {
  showFilesPanel();
  state.query = '';
  $('#searchInput').value = '';
  state.searching = false;
  $('#loadingState').hidden = false;
  $('#emptyState').hidden = true;
  try {
    const d = await api('/chfs/files?filepath=' + encodeURIComponent(p));
    state.path = p;
    state.mask = d.currentMask || '';
    state.files = d.files || [];
    $('#pageHeading').textContent = p === '/' ? '文件' : (p.split('/').filter(Boolean).pop() || '文件');
    state.loaded = true;
    renderCrumbs();
    renderFiles();
    updateTools();
    const target = '#' + encodePath(p);
    if (location.hash !== target) location.hash = target;
  } catch (e) {
    state.loaded = false;
    $('#loadingState').hidden = true;
    $('#emptyState').hidden = false;
    $('#emptyIcon').innerHTML = icon('file');
    $('#emptyTitle').textContent = e.status === 401 ? '需要登录后查看' : '加载失败';
    $('#emptyTip').textContent = e.message || '请稍后刷新重试';
    toast(e.message, 'err');
    if (e.status === 404) loadFiles('/');
  }
}

function crumb(label, path, current, isRoot) {
  const a = h('a', { class: 'crumb' + (current ? ' current' : ''), href: '#' + encodePath(path), title: label },
    h('span', { html: isRoot ? icon('folder') : '' }), label);
  return a;
}

function renderCrumbs() {
  const bc = $('#breadcrumb');
  bc.innerHTML = '';
  const segs = state.path.split('/').filter(Boolean);
  bc.append(crumb('全部文件', '/', segs.length === 0, true));
  let acc = '';
  segs.forEach((seg, i) => {
    acc += '/' + seg;
    bc.append(h('span', { class: 'crumb-sep' }, '/'));
    bc.append(crumb(seg, acc, i === segs.length - 1, false));
  });
}

function sortedFiles() {
  let list = state.files.slice();
  if (state.query && state.scope === 'local') {
    const q = state.query.toLowerCase();
    list = list.filter(f => f.name.toLowerCase().includes(q));
  }
  const key = state.sortKey;
  const dir = state.sortDir;
  list.sort((a, b) => {
    if (!!a.dir !== !!b.dir) return a.dir ? -1 : 1;
    let r = 0;
    if (key === 'name') r = a.name.localeCompare(b.name, 'zh-Hans-CN', { numeric: true });
    else if (key === 'size') r = (a.size || 0) - (b.size || 0);
    else r = String(a.modified || '').localeCompare(String(b.modified || ''));
    return r * dir;
  });
  return list;
}

function iconBox(f) {
  const k = kindOf(f);
  return h('span', { class: 'ficon', 'data-kind': k, html: icon(k) });
}

function actBtn(name, title, fn, cls) {
  return h('button', { class: 'act' + (cls ? ' ' + cls : ''), type: 'button', title: title, 'aria-label': title, html: icon(name), onclick: fn });
}

function actionsFor(f) {
  const box = h('div', { class: 'rowact' });
  if (hasMask(f.mask, 'R')) {
    if (!f.dir && previewType(f)) box.append(actBtn('eye', '预览', () => openPreview(f)));
    box.append(actBtn('download', f.dir ? '打包下载' : '下载', () => download(f), 'act-dl'));
    if (state.authenticated) box.append(actBtn('share', '创建分享链接', () => openShare(f), 'act-share'));
  }
  if (hasMask(f.mask, 'D')) {
    box.append(actBtn('edit', '重命名', () => renameEntry(f), 'act-rename'));
    box.append(actBtn('trash', '删除', () => removeEntry(f), 'danger'));
  }
  return box;
}

function renderFiles() {
  const list = sortedFiles();
  const grid = state.grid;
  const empty = list.length === 0;
  $('#fileBody').innerHTML = '';
  $('#gridView').innerHTML = '';
  $('#loadingState').hidden = true;
  $('#listView').hidden = empty || grid;
  $('#gridView').hidden = empty || !grid;
  $('#emptyState').hidden = !empty;
  const noResult = state.searching || !!state.query.trim();
  if (empty) {
    $('#emptyIcon').innerHTML = noResult ? icon('search') : icon('folder');
    $('#emptyTitle').textContent = noResult ? '没有找到相关文件' : '这个文件夹还是空的';
    $('#emptyTip').textContent = noResult
      ? '换个关键词，或切换到全站搜索试试'
      : (hasMask(state.mask, 'W')
        ? '把文件拖到这里，或点上方“上传文件”'
        : (!state.authenticated ? '游客可查看和下载，登录后即可上传和管理文件' : '当前目录没有上传和管理权限'));
  }

  if (grid) {
    list.forEach(f => $('#gridView').append(cardFor(f)));
  } else {
    list.forEach(f => $('#fileBody').append(rowFor(f)));
  }
  updateSortHeaders();
  updateFoot(list);
}

function rowFor(f) {
  const tr = h('tr', { onclick: ev => {
    if (ev.target.closest('button, a, .rowact')) return;
    openEntry(f);
  }});
  const nameWrap = h('div', { class: 'fname' }, iconBox(f), h('div', { style: 'min-width:0' },
    h('a', { class: 'linky' + (f.dir ? ' dirname' : ''), href: '#', title: f.name, onclick: ev => { ev.preventDefault(); openEntry(f); } }, f.name),
    f.path ? h('span', { class: 'subpath' }, f.path) : null
  ));
  tr.append(h('td', { class: 'col-name' }, nameWrap));
  tr.append(h('td', { class: 'col-size meta' }, f.dir ? '—' : fmtSize(f.size)));
  tr.append(h('td', { class: 'col-time meta' }, f.modified || ''));
  tr.append(h('td', { class: 'col-act' }, actionsFor(f)));
  return tr;
}

function cardFor(f) {
  const k = kindOf(f);
  let thumb = h('div', { class: 'card-thumb', 'data-kind': k, html: '<span class="bigicon">' + icon(k) + '</span>' });
  if (k === 'image' && hasMask(f.mask, 'R')) {
    const img = h('img', { src: thumbURL(f, 320), loading: 'lazy', alt: '' });
    img.onerror = () => { img.replaceWith(h('span', { class: 'bigicon', html: icon('image') })); };
    thumb = h('div', { class: 'card-thumb', 'data-kind': k }, img);
  }
  const actions = actionsFor(f);
  actions.className = 'card-actions';
  const body = h('div', { class: 'card-body' },
    h('a', { class: 'card-name', href: '#', title: f.name, onclick: ev => { ev.preventDefault(); openEntry(f); } }, f.name),
    h('div', { class: 'card-meta' },
      h('span', {}, f.dir ? '文件夹' : fmtSize(f.size)),
      h('span', {}, shortTime(f.modified))
    )
  );
  const card = h('div', { class: 'card', onclick: ev => {
    if (ev.target.closest('button, a, .card-actions')) return;
    openEntry(f);
  }});
  card.append(thumb, body, actions);
  return card;
}

function updateSortHeaders() {
  const labels = { name: '名称', size: '大小', time: '修改时间' };
  $$('#listView .filetable thead th[data-sort]').forEach(th => {
    const k = th.dataset.sort;
    th.textContent = labels[k] + (state.sortKey === k ? (state.sortDir > 0 ? ' ▲' : ' ▼') : '');
  });
}

function updateFoot(list) {
  const dirs = list.filter(f => f.dir).length;
  const files = list.length - dirs;
  const info = '共 ' + list.length + ' 项 · ' + dirs + ' 个文件夹 · ' + files + ' 个文件';
  let right;
  if (hasMask(state.mask, 'W') && hasMask(state.mask, 'D')) right = '可上传、可管理';
  else if (hasMask(state.mask, 'W')) right = '可上传、可下载';
  else right = '可查看、可下载';
  if (!state.authenticated) right += '（游客）';
  $('#panelFoot').innerHTML = '';
  $('#panelFoot').append(h('span', {}, info), h('span', {}, right));
}

function updateTools() {
  const canW = hasMask(state.mask, 'W');
  ['#btnUpload', '#btnUploadDir', '#btnNewDir', '#btnNewText'].forEach(s => {
    $(s).style.display = canW ? '' : 'none';
  });
}

function openEntry(f) {
  if (f.dir) loadFiles(entryVP(f));
  else if (previewType(f)) openPreview(f);
  else download(f);
}

/* ---------- preview ---------- */
function previewCandidates() {
  return sortedFiles().filter(f => !f.dir && previewType(f));
}

function rawURL(f) { return '/chfs/raw?file=' + encodeURIComponent(entryVP(f)); }

const scriptLoads = {};
function loadScriptOnce(src) {
  if (scriptLoads[src]) return scriptLoads[src];
  scriptLoads[src] = new Promise((resolve, reject) => {
    const s = document.createElement('script');
    s.src = src;
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => { delete scriptLoads[src]; reject(new Error('预览组件加载失败')); };
    document.head.append(s);
  });
  return scriptLoads[src];
}

async function fetchArrayBuffer(f) {
  const res = await fetch(rawURL(f), { credentials: 'same-origin' });
  if (!res.ok) throw new Error('读取文件失败（HTTP ' + res.status + '）');
  return await res.arrayBuffer();
}

function showPreviewMessage(body, msg, f) {
  const box = h('div', { class: 'preview-message' }, h('div', { text: msg }));
  if (f) box.append(h('a', { class: 'btn', href: sharedURL(f) }, '下载文件'));
  body.append(box);
}

async function renderDocx(body, f, kind) {
  if (kind === 'doc') {
    showPreviewMessage(body, '旧版 .doc 格式暂不支持在线预览，请下载后用 Word 打开。', f);
    return;
  }
  body.append(h('div', { class: 'preview-message', text: '正在加载文档…' }));
  try {
    await loadScriptOnce('/asset/vendor/jszip.min.js');
    await loadScriptOnce('/asset/vendor/docx-preview.min.js');
    const buf = await fetchArrayBuffer(f);
    body.innerHTML = '';
    const frame = h('iframe', { class: 'docx-frame', sandbox: 'allow-same-origin', title: f.name });
    body.append(frame);
    const idoc = frame.contentDocument || frame.contentWindow.document;
    idoc.open();
    idoc.write('<!DOCTYPE html><html><head><meta charset="utf-8"></head><body></body></html>');
    idoc.close();
    idoc.head.append(h('style', { text: 'html,body{margin:0;padding:24px;background:#f1f2f4;} body{font-family:"Microsoft YaHei","PingFang SC",sans-serif;} .docx-wrapper{background:transparent;padding:0;} .docx{background:#fff;color:#000;margin:0 auto 18px;box-shadow:0 2px 12px rgba(0,0,0,.12);} img{max-width:100%;}' }));
    if (!window.docx || !window.docx.renderAsync) throw new Error('文档组件未就绪');
    await window.docx.renderAsync(buf, idoc.body, idoc.head, {
      className: 'docx', inWrapper: true, breakPages: true, ignoreLastRenderedPageBreak: true
    });
  } catch (e) {
    body.innerHTML = '';
    showPreviewMessage(body, '无法在线预览这个 Word 文档：' + e.message + '。可以下载后用 Word 打开。', f);
  }
}

async function renderSheet(body, f) {
  body.append(h('div', { class: 'preview-message', text: '正在解析表格…' }));
  try {
    await loadScriptOnce('/asset/vendor/xlsx.full.min.js');
    const buf = await fetchArrayBuffer(f);
    const wb = window.XLSX.read(buf, { type: 'array' });
    if (!wb.SheetNames || !wb.SheetNames.length) throw new Error('表格内容为空');
    body.innerHTML = '';
    const wrap = h('div', { class: 'sheet-preview' });
    const tabs = h('div', { class: 'sheet-tabs' });
    const host = h('div', { class: 'sheet-host' });
    const note = h('div', { class: 'sheet-note' });
    wrap.append(tabs, host, note);
    body.append(wrap);
    const render = name => {
      tabs.querySelectorAll('.sheet-tab').forEach(b => b.classList.toggle('active', b.textContent === name));
      host.innerHTML = '';
      const rows = window.XLSX.utils.sheet_to_json(wb.Sheets[name], { header: 1, raw: false, defval: '', blankrows: false });
      const maxR = Math.min(rows.length, 300);
      let cols = 0;
      rows.slice(0, 50).forEach(r => { cols = Math.max(cols, r.length); });
      cols = Math.min(cols, 30);
      const table = h('table', { class: 'sheet-table' });
      for (let i = 0; i < maxR; i++) {
        const tr = h('tr');
        const row = rows[i] || [];
        for (let j = 0; j < cols; j++) tr.append(h('td', { text: String(row[j] == null ? '' : row[j]) }));
        table.append(tr);
      }
      host.append(table);
      note.textContent = '仅预览前 ' + maxR + ' 行、' + cols + ' 列；完整内容请下载查看。';
    };
    wb.SheetNames.forEach((name, i) => {
      const b = h('button', { class: 'sheet-tab' + (i === 0 ? ' active' : ''), type: 'button', text: name });
      b.addEventListener('click', () => render(name));
      tabs.append(b);
    });
    render(wb.SheetNames[0]);
  } catch (e) {
    body.innerHTML = '';
    showPreviewMessage(body, '无法在线预览这个表格：' + e.message + '。可以下载后用 Excel 打开。', f);
  }
}
function thumbURL(f, w) { return '/chfs/thumb?file=' + encodeURIComponent(entryVP(f)) + '&w=' + (w || 320); }
function sharedURL(f) { return '/chfs/shared' + encodePath(entryVP(f)); }
function zipURL(f) { return '/chfs/downloaddir' + encodePath(entryVP(f)) + '.zip'; }

async function openPreview(f) {
  const list = previewCandidates();
  const idx = list.findIndex(x => entryVP(x) === entryVP(f));
  state.preview.list = list;
  state.preview.index = idx < 0 ? 0 : idx;
  openModal('#previewModal');
  await renderPreview();
}

async function renderPreview() {
  const list = state.preview.list;
  const f = list[state.preview.index];
  state.preview.editing = false;
  $('#btnPreviewEdit').hidden = true;
  $('#btnPreviewEdit').textContent = '编辑';
  $('#btnPrevFile').disabled = state.preview.index <= 0;
  $('#btnNextFile').disabled = state.preview.index >= list.length - 1;
  if (!f) return;
  $('#previewTitle').textContent = f.name;
  $('#btnPreviewDownload').href = sharedURL(f);
  const body = $('#previewBody');
  body.innerHTML = '';
  const type = previewType(f);
  if (type === 'image') {
    body.append(h('img', { src: rawURL(f), alt: f.name }));
  } else if (type === 'video') {
    body.append(h('video', { src: rawURL(f), controls: 'controls', playsinline: 'playsinline' }));
  } else if (type === 'audio') {
    body.append(h('div', { class: 'audio-preview' },
      h('div', { class: 'audio-name', text: f.name }),
      h('audio', { src: rawURL(f), controls: 'controls', style: 'width:100%' })));
  } else if (type === 'pdf') {
    body.append(h('iframe', { src: rawURL(f), title: f.name }));
  } else if (type === 'docx' || type === 'doc') {
    await renderDocx(body, f, type);
  } else if (type === 'sheet') {
    await renderSheet(body, f);
  } else if (type === 'slides') {
    showPreviewMessage(body, 'PPT 暂不支持在线预览，请下载后用 PowerPoint 打开。', f);
  } else if (type === 'text') {
    body.append(h('pre', {}, '加载中…'));
    try {
      const txt = await api('/chfs/text?file=' + encodeURIComponent(entryVP(f)));
      state.preview.text = txt;
      body.innerHTML = '';
      body.append(h('pre', {}, txt));
      if (hasMask(f.mask, 'W')) $('#btnPreviewEdit').hidden = false;
    } catch (e) {
      body.innerHTML = '';
      body.append(h('pre', {}, '无法预览：' + e.message));
    }
  }
}

function editPreview() {
  const f = state.preview.list[state.preview.index];
  if (!f) return;
  if (!state.preview.editing) {
    const body = $('#previewBody');
    body.innerHTML = '';
    body.append(h('textarea', { id: 'previewTextArea' }, state.preview.text));
    state.preview.editing = true;
    $('#btnPreviewEdit').textContent = '保存';
    $('#btnPrevFile').disabled = true;
    $('#btnNextFile').disabled = true;
    return;
  }
  if (!hasMask(f.mask, 'W')) { toast('没有写入权限', 'err'); return; }
  const ta = $('#previewTextArea');
  api('/chfs/updatetext', {
    method: 'POST',
    body: fd({ filepath: parentVP(entryVP(f)), filename: f.name, text: ta ? ta.value : '' })
  }).then(() => {
    toast('已保存', 'ok');
    renderPreview();
    reloadCurrent();
  }).catch(e => toast(e.message, 'err'));
}

function previewStep(delta) {
  const i = state.preview.index + delta;
  if (i < 0 || i >= state.preview.list.length) return;
  state.preview.index = i;
  renderPreview();
}

function download(f) {
  const a = h('a', { href: f.dir ? zipURL(f) : sharedURL(f), download: '', style: 'display:none' });
  document.body.append(a);
  a.click();
  setTimeout(() => a.remove(), 1000);
}

/* ---------- entry ops ---------- */
async function renameEntry(f) {
  const vp = entryVP(f);
  const r = await promptBox({ title: '重命名', label: '新名称', value: f.name });
  if (!r || !r.name || r.name === f.name) return;
  if (r.name.includes('/')) { toast('名称不能包含 /', 'err'); return; }
  try {
    await api('/chfs/rename', { method: 'POST', body: fd({ old: vp, new: vpJoin(parentVP(vp), r.name) }) });
    toast('已重命名');
    reloadCurrent();
  } catch (e) { toast(e.message, 'err'); }
}

async function removeEntry(f) {
  const vp = entryVP(f);
  const msg = hasMask(f.mask, 'D') ? '删除「' + f.name + '」？' + (state.cfg.remove == 3 ? '此操作不可恢复。' : '可在回收站恢复。') : '没有删除权限。';
  if (!await confirmBox('删除', msg, '删除')) return;
  try {
    await api('/chfs/rmfiles', { method: 'DELETE', body: fd({ filepath: vp }) });
    toast(state.cfg.remove == 3 ? '已删除' : '已移入回收站');
    reloadCurrent();
  } catch (e) { toast(e.message, 'err'); }
}

function reloadCurrent() {
  if (state.view === 'files') loadFiles(state.path);
  else if (state.view === 'trash') loadTrash();
  else if (state.view === 'shares') loadShares();
}

/* ---------- create ---------- */
async function newDir() {
  const r = await promptBox({ title: '新建文件夹', label: '文件夹名称', value: '' });
  if (!r || !r.name) return;
  try {
    await api('/chfs/newdir', { method: 'POST', body: fd({ filepath: vpJoin(state.path, r.name) }) });
    toast('已创建文件夹');
    loadFiles(state.path);
  } catch (e) { toast(e.message, 'err'); }
}

async function newText() {
  const r = await promptBox({ title: '新建文本', label: '文件名', value: '未命名.txt', multiline: true, text: '' });
  if (!r || !r.name) return;
  try {
    await api('/chfs/newtext', { method: 'POST', body: fd({ filepath: state.path, title: r.name, text: r.text || '' }) });
    toast('已创建文本');
    loadFiles(state.path);
  } catch (e) { toast(e.message, 'err'); }
}

/* ---------- share ---------- */
function openShare(f) {
  state.shareTarget = f;
  $('#shareName').textContent = f.name;
  $('#sharePwd').value = '';
  $('#shareResult').hidden = true;
  $('#shareUrl').value = '';
  $('#shareQR').removeAttribute('src');
  $('#btnShareCreate').disabled = false;
  $('#btnShareCreate').hidden = false;
  $('#btnShareCreate').textContent = '生成链接';
  openModal('#shareModal');
}

async function createShare() {
  const f = state.shareTarget;
  if (!f) return;
  $('#btnShareCreate').disabled = true;
  try {
    const d = await api('/chfs/share', { method: 'POST', body: fd({ filepath: entryVP(f), ttl: $('#shareTTL').value, password: $('#sharePwd').value.trim() }) });
    const abs = location.origin + d.url;
    $('#shareUrl').value = abs;
    $('#shareQR').src = '/chfs/qr?text=' + encodeURIComponent(abs);
    $('#shareResult').hidden = false;
    $('#btnShareCreate').textContent = '重新生成';
    toast('分享链接已生成', 'ok');
  } catch (e) {
    toast(e.message, 'err');
    $('#btnShareCreate').disabled = false;
  }
}

async function copyShare() {
  const t = $('#shareUrl').value;
  try { await navigator.clipboard.writeText(t); } catch (e) {
    const i = $('#shareUrl');
    i.select();
    document.execCommand('copy');
  }
  toast('已复制到剪贴板', 'ok');
}

/* ---------- upload ---------- */
function upItem(name) {
  const bar = h('i');
  const pct = h('span', { class: 'uplpct' }, '0%');
  const nameEl = h('span', { class: 'uplname' }, name);
  const item = h('div', { class: 'upitem' },
    h('div', { class: 'upline' }, nameEl, pct),
    h('div', { class: 'upbar' }, bar)
  );
  $('#uploads').append(item);
  return {
    set(p) { bar.style.width = Math.round(p * 100) + '%'; pct.textContent = Math.round(p * 100) + '%'; },
    finish(ok, msg) {
      item.classList.add(ok ? 'ok' : 'err');
      pct.textContent = ok ? '完成' : (msg || '失败');
      if (!ok) nameEl.title = msg || '';
      setTimeout(() => item.remove(), ok ? 1800 : 4000);
    }
  };
}

function uploadOne(file, dest, rel) {
  return new Promise(resolve => {
    const item = upItem(rel || file.name);
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/chfs/upload');
    xhr.upload.onprogress = e => { if (e.lengthComputable) item.set(e.loaded / e.total); };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) { item.set(1); item.finish(true); resolve(true); }
      else {
        let msg = 'HTTP ' + xhr.status;
        try { msg = JSON.parse(xhr.responseText).error || msg; } catch (e) {}
        item.finish(false, msg);
        resolve(false);
      }
    };
    xhr.onerror = () => { item.finish(false, '网络错误'); resolve(false); };
    const f = new FormData();
    f.append('folder', dest);
    if (rel) f.append('path', rel);
    f.append('file', file, file.name);
    xhr.send(f);
  });
}

async function uploadList(items, dest) {
  let ok = 0;
  for (const it of items) {
    if (await uploadOne(it.file, dest, it.rel)) ok++;
  }
  if (ok) toast('上传完成，共 ' + ok + ' 个文件', 'ok');
  if (dest === state.path && state.view === 'files') loadFiles(state.path);
}

function pickFiles(multiple, dirMode) {
  const input = h('input', { type: 'file', style: 'display:none' });
  if (multiple) input.multiple = true;
  if (dirMode) { input.webkitdirectory = true; input.directory = true; }
  document.body.append(input);
  input.onchange = async () => {
    const files = Array.from(input.files || []);
    input.remove();
    if (!files.length) return;
    const items = files.map(f => ({ file: f, rel: dirMode ? f.webkitRelativePath : '' }));
    await uploadList(items, state.path);
  };
  input.click();
}

/* drag & drop with directory support */
async function walkEntry(entry, prefix, out) {
  if (entry.isFile) {
    await new Promise(res => entry.file(f => { out.push({ file: f, rel: prefix + entry.name }); res(); }, () => res()));
  } else if (entry.isDirectory) {
    const reader = entry.createReader();
    const read = () => new Promise(res => reader.readEntries(res, () => res([])));
    let batch = await read();
    while (batch.length) {
      for (const e of batch) await walkEntry(e, prefix + entry.name + '/', out);
      batch = await read();
    }
  }
}

async function dropItems(dt) {
  const items = Array.from(dt.items || []);
  const entries = items.map(i => i.webkitGetAsEntry && i.webkitGetAsEntry()).filter(Boolean);
  if (entries.length) {
    const out = [];
    for (const e of entries) await walkEntry(e, '', out);
    return out;
  }
  return Array.from(dt.files || []).map(f => ({ file: f, rel: '' }));
}

/* ---------- trash ---------- */
async function showTrash() {
  state.view = 'trash';
  $('#pageHeading').textContent = '回收站';
  setNav('trash');
  const search = $('.search');
  if (search) search.hidden = true;
  $('#fileTools').hidden = true;
  $('#listView').hidden = true;
  $('#gridView').hidden = true;
  $('#sharesView').hidden = true;
  $('#trashView').hidden = false;
  $('#panelHead').hidden = true;
  $('#loadingState').hidden = true;
  $('#emptyState').hidden = true;
  $('#breadcrumb').innerHTML = '';
  $('#breadcrumb').append(crumb('回收站', '/trash', true, false));
  await loadTrash();
}

async function loadTrash() {
  const view = $('#trashView');
  view.innerHTML = '';
  try {
    const d = await api('/chfs/trash');
    const items = d.items || [];
    const head = h('div', { class: 'view-head' },
      h('span', { class: 'vh-title' }, '回收站 ' + (items.length ? '· ' + items.length + ' 项' : '')),
      h('button', { class: 'btn danger', type: 'button', onclick: emptyTrash }, '清空回收站')
    );
    if (!items.length) {
      view.append(h('div', { class: 'empty' }, h('span', { class: 'empty-icon', html: icon('trash') }), h('p', {}, '回收站是空的'), h('span', {}, '删除的文件会先放在这里，可恢复或彻底删除')));
      $('#panelFoot').innerHTML = '';
      return;
    }
    view.append(head);
    const table = h('table', { class: 'filetable' });
    table.append(h('thead', {}, h('tr', {},
      h('th', { class: 'col-name' }, '名称'),
      h('th', { style: 'width:200px' }, '原位置'),
      h('th', { class: 'col-time' }, '删除时间'),
      h('th', { class: 'col-size' }, '大小'),
      h('th', { class: 'col-act' }, '操作')
    )));
    const tb = h('tbody');
    items.forEach(it => {
      const acts = h('div', { class: 'rowact' },
        h('button', { class: 'act', title: '恢复', html: icon('refresh'), onclick: () => restoreTrash(it.id) }),
        h('button', { class: 'act danger', title: '彻底删除', html: icon('trash'), onclick: () => purgeTrash(it) })
      );
      tb.append(h('tr', {},
        h('td', { class: 'col-name' }, h('div', { class: 'fname' }, iconBox({ dir: it.is_dir, name: it.name }), h('div', { class: 'linky' }, it.name))),
        h('td', { class: 'meta', style: 'width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap' }, it.original_vp),
        h('td', { class: 'col-time meta' }, new Date((it.deleted_at || 0) * 1000).toLocaleString('zh-CN')),
        h('td', { class: 'col-size meta' }, it.is_dir ? '—' : fmtSize(it.size)),
        h('td', { class: 'col-act' }, acts)
      ));
    });
    table.append(tb);
    view.append(table);
    $('#panelFoot').innerHTML = '';
    $('#panelFoot').append(h('span', {}, '共 ' + items.length + ' 项'));
  } catch (e) {
    if (e.status === 401) {
      view.append(h('div', { class: 'empty' }, h('p', {}, '需要登录'), h('button', { class: 'btn primary', type: 'button', onclick: openLogin }, '登录')));
    } else {
      view.append(h('div', { class: 'empty' }, h('p', {}, e.message)));
    }
  }
}

async function restoreTrash(id) {
  try { await api('/chfs/trash', { method: 'POST', body: fd({ action: 'restore', id }) }); toast('已恢复', 'ok'); loadTrash(); }
  catch (e) { toast(e.message, 'err'); }
}

async function purgeTrash(it) {
  if (!await confirmBox('彻底删除', '「' + it.name + '」将被永久删除，无法恢复。', '永久删除')) return;
  try { await api('/chfs/trash', { method: 'POST', body: fd({ action: 'purge', id: it.id }) }); toast('已永久删除'); loadTrash(); }
  catch (e) { toast(e.message, 'err'); }
}

async function emptyTrash() {
  if (!await confirmBox('清空回收站', '回收站内所有文件将被永久删除。', '清空')) return;
  try { await api('/chfs/trash', { method: 'POST', body: fd({ action: 'empty' }) }); toast('回收站已清空'); loadTrash(); }
  catch (e) { toast(e.message, 'err'); }
}

/* ---------- shares ---------- */
async function showShares() {
  state.view = 'shares';
  $('#pageHeading').textContent = '我的分享';
  setNav('shares');
  const search = $('.search');
  if (search) search.hidden = true;
  $('#fileTools').hidden = true;
  $('#listView').hidden = true;
  $('#gridView').hidden = true;
  $('#trashView').hidden = true;
  $('#sharesView').hidden = false;
  $('#panelHead').hidden = true;
  $('#loadingState').hidden = true;
  $('#emptyState').hidden = true;
  $('#breadcrumb').innerHTML = '';
  $('#breadcrumb').append(crumb('我的分享', '/shares', true, false));
  await loadShares();
}

async function loadShares() {
  const view = $('#sharesView');
  view.innerHTML = '';
  try {
    const items = (await api('/chfs/share')) || [];
    if (!items.length) {
      view.append(h('div', { class: 'empty' }, h('span', { class: 'empty-icon', html: icon('share') }), h('p', {}, '还没有分享链接'), h('span', {}, '找到要分享的文件，点击分享按钮即可生成链接')));
      $('#panelFoot').innerHTML = '';
      return;
    }
    const table = h('table', { class: 'filetable' });
    table.append(h('thead', {}, h('tr', {},
      h('th', { class: 'col-name' }, '文件'),
      h('th', { class: 'col-time' }, '创建时间'),
      h('th', { class: 'col-time' }, '有效期'),
      h('th', { class: 'col-size' }, '下载'),
      h('th', { class: 'col-act' }, '操作')
    )));
    const tb = h('tbody');
    items.forEach(sh => {
      const url = location.origin + '/s/' + sh.token;
      const acts = h('div', { class: 'rowact' },
        h('button', { class: 'act', title: '复制链接', html: icon('share'), onclick: async () => {
          try { await navigator.clipboard.writeText(url); } catch (e) {}
          toast('已复制链接', 'ok');
        } }),
        h('button', { class: 'act', title: '二维码', html: icon('qr'), onclick: () => showShareQR(sh) }),
        h('button', { class: 'act danger', title: '删除分享', html: icon('trash'), onclick: () => deleteShare(sh) })
      );
      const exp = sh.expires_at ? new Date(sh.expires_at * 1000).toLocaleString('zh-CN') : '永久';
      tb.append(h('tr', {},
        h('td', { class: 'col-name' }, h('div', { class: 'fname' }, iconBox({ dir: sh.is_dir, name: sh.name }), h('div', { class: 'linky' }, sh.name))),
        h('td', { class: 'col-time meta' }, new Date(sh.created_at * 1000).toLocaleString('zh-CN')),
        h('td', { class: 'col-time meta' }, exp),
        h('td', { class: 'col-size meta' }, String(sh.downloads || 0)),
        h('td', { class: 'col-act' }, acts)
      ));
    });
    table.append(tb);
    view.append(table);
    $('#panelFoot').innerHTML = '';
    $('#panelFoot').append(h('span', {}, '共 ' + items.length + ' 个分享'), h('span', {}, ''));
  } catch (e) {
    if (e.status === 401) view.append(h('div', { class: 'empty' }, h('p', {}, '需要登录'), h('button', { class: 'btn primary', type: 'button', onclick: openLogin }, '登录')));
    else view.append(h('div', { class: 'empty' }, h('p', {}, e.message)));
  }
}

function showShareQR(sh) {
  const url = location.origin + '/s/' + sh.token;
  $('#shareName').textContent = sh.name;
  $('#shareResult').hidden = false;
  $('#shareUrl').value = url;
  $('#shareQR').src = '/chfs/qr?text=' + encodeURIComponent(url);
  $('#btnShareCreate').hidden = true;
  openModal('#shareModal');
}

async function deleteShare(sh) {
  if (!await confirmBox('删除分享', '删除后该链接将立即失效。', '删除')) return;
  try {
    await api('/chfs/share', { method: 'DELETE', body: fd({ token: sh.token }) });
    toast('已删除分享');
    loadShares();
  } catch (e) { toast(e.message, 'err'); }
}

/* ---------- search ---------- */
async function globalSearch() {
  const q = $('#searchInput').value.trim();
  if (!q) return;
  try {
    const d = await api('/chfs/search?str=' + encodeURIComponent(q));
    state.files = (d.files || []).map(f => Object.assign({}, f, { path: f.path || '/' }));
    state.searching = true;
    state.view = 'files';
    state.mask = 'R';
    $('#panelHead').hidden = false;
    $('#searchResultLabel').textContent = '全站搜索「' + q + '」· 共找到 ' + state.files.length + ' 项';
    $('#fileTools').hidden = true;
    $('#trashView').hidden = true;
    $('#sharesView').hidden = true;
    renderFiles();
  } catch (e) { toast(e.message, 'err'); }
}

function clearSearch(reload) {
  state.searching = false;
  state.query = '';
  $('#searchInput').value = '';
  $('#panelHead').hidden = true;
  $('#fileTools').hidden = false;
  if (reload !== false) loadFiles(state.path);
}

/* ---------- bind ---------- */
function bind() {
  $('#userBtn').addEventListener('click', () => state.authenticated ? doLogout() : openLogin());
  const guestLogin = $('#guestLogin');
  if (guestLogin) guestLogin.addEventListener('click', openLogin);
  ['#loginUser', '#loginPwd'].forEach(sel => {
    const el = $(sel);
    if (el) el.addEventListener('input', () => {
      $('#loginHint').textContent = '';
      $('#loginHint').classList.remove('error');
    });
  });
  $('#btnLogin').addEventListener('click', doLogin);
  $('#loginPwd').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
  $('#loginUser').addEventListener('keydown', e => { if (e.key === 'Enter') $('#loginPwd').focus(); });

  $('#btnUpload').addEventListener('click', () => pickFiles(true, false));
  $('#btnUploadDir').addEventListener('click', () => pickFiles(true, true));
  $('#btnNewDir').addEventListener('click', newDir);
  $('#btnNewText').addEventListener('click', newText);
  $('#btnRefresh').addEventListener('click', () => reloadCurrent());
  $('#btnClearSearch').addEventListener('click', () => clearSearch(true));

  $('#viewToggle').addEventListener('click', e => {
    const b = e.target.closest('button[data-view]');
    if (!b) return;
    state.grid = b.dataset.view === 'grid';
    $$('#viewToggle button').forEach(x => x.classList.toggle('active', x === b));
    renderFiles();
  });

  $$('#listView thead th[data-sort]').forEach(th => th.addEventListener('click', () => {
    const k = th.dataset.sort;
    if (state.sortKey === k) state.sortDir = -state.sortDir;
    else { state.sortKey = k; state.sortDir = 1; }
    renderFiles();
  }));

  const input = $('#searchInput');
  input.addEventListener('input', () => {
    if (state.scope !== 'local') return;
    state.query = input.value.trim();
    if (state.view === 'files' && !state.searching) renderFiles();
  });
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter') {
      if (state.scope === 'local') {
        state.query = input.value.trim();
        renderFiles();
      } else globalSearch();
    }
  });
  $('#searchScope').addEventListener('click', () => {
    state.scope = state.scope === 'local' ? 'all' : 'local';
    const btn = $('#searchScope');
    btn.textContent = state.scope === 'local' ? '当前目录' : '全站';
    btn.classList.toggle('active', state.scope === 'local');
    btn.title = state.scope === 'local' ? '在当前目录内筛选' : '搜索全部可访问文件';
    input.placeholder = state.scope === 'local' ? '搜索当前目录' : '全站搜索，回车确认';
    if (state.scope === 'local' && state.searching) clearSearch(true);
  });

  $('#btnPromptOk').addEventListener('click', () => {
    if (!promptResolve) return;
    const r = promptResolve;
    promptResolve = null;
    closeModal('#promptModal');
    r({ name: $('#promptInput').value.trim(), text: $('#promptText').value });
  });
  $('#promptInput').addEventListener('keydown', e => { if (e.key === 'Enter' && $('#textField').hidden) $('#btnPromptOk').click(); });

  $('#btnConfirm').addEventListener('click', () => {
    if (!confirmResolve) return;
    const r = confirmResolve;
    confirmResolve = null;
    closeModal('#confirmModal');
    r(true);
  });

  $('#btnShareCreate').addEventListener('click', createShare);
  $('#btnCopyShare').addEventListener('click', copyShare);

  $('#btnPreviewEdit').addEventListener('click', editPreview);
  $('#btnPrevFile').addEventListener('click', () => previewStep(-1));
  $('#btnNextFile').addEventListener('click', () => previewStep(1));

  $$('[data-close]').forEach(b => b.addEventListener('click', () => closeModal('#' + b.closest('.modal').id)));
  $$('.modal').forEach(m => m.addEventListener('mousedown', e => { if (e.target === m) closeModal('#' + m.id); }));

  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
      const open = $$('.modal.open').pop();
      if (open) closeModal('#' + open.id);
    }
    if ($('#previewModal').classList.contains('open') && state.view === 'files') {
      if (e.key === 'ArrowLeft') previewStep(-1);
      if (e.key === 'ArrowRight') previewStep(1);
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's' && state.preview.editing) {
        e.preventDefault();
        editPreview();
      }
    }
  });

  /* drag & drop */
  let dragDepth = 0;
  window.addEventListener('dragenter', e => {
    if (state.view !== 'files' || !hasMask(state.mask, 'W')) return;
    if (!e.dataTransfer || !Array.from(e.dataTransfer.types || []).includes('Files')) return;
    dragDepth++;
    $('#dropPath').textContent = state.path;
    $('#dropmask').classList.add('on');
  });
  window.addEventListener('dragleave', () => {
    if (dragDepth > 0) dragDepth--;
    if (dragDepth === 0) $('#dropmask').classList.remove('on');
  });
  window.addEventListener('dragover', e => { if (state.view === 'files') e.preventDefault(); });
  window.addEventListener('drop', async e => {
    e.preventDefault();
    dragDepth = 0;
    $('#dropmask').classList.remove('on');
    if (state.view !== 'files' || !hasMask(state.mask, 'W')) return;
    const items = await dropItems(e.dataTransfer);
    if (items.length) await uploadList(items, state.path);
  });

  window.addEventListener('hashchange', route);
}

/* ---------- boot ---------- */
(async function boot() {
  try {
    const cfg = await api('/chfs/config');
    state.cfg = cfg;
    const title = cfg.title || '文件空间';
    document.title = title;
    $('#siteTitle').textContent = title;
    $$('[data-site-title]').forEach(el => el.textContent = title);
    $('#verText').textContent = cfg.version || '-';
    $('#footerVersion').textContent = 'v' + (cfg.version || '1.0.0');
    const icpLink = $('#icpLink');
    const icpEmpty = $('#icpEmpty');
    const policeLink = $('#policeLink');
    const footerSep = $('.footer-sep');
    if (cfg.icp) {
      icpLink.hidden = false;
      icpLink.textContent = cfg.icp;
      icpEmpty.hidden = true;
    } else {
      icpLink.hidden = true;
      icpEmpty.hidden = false;
    }
    if (cfg.police) {
      policeLink.hidden = false;
      policeLink.textContent = cfg.police;
      footerSep.hidden = !cfg.icp;
    } else {
      policeLink.hidden = true;
      footerSep.hidden = true;
    }
    if (cfg.notice) {
      $('#noticeBar').hidden = false;
      $('#noticeBar').textContent = cfg.notice;
    }
  } catch (e) {}
  await refreshSession();
  bind();
  route();
})();

/* ---------- theme ---------- */
(function initTheme() {
  const btn = $('#themeBtn');
  if (!btn) return;
  const KEY = 'mfs-theme';
  const order = ['auto', 'light', 'dark'];
  const icons = {
    auto: '<svg class="ic" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 4a8 8 0 000 16z" fill="currentColor" stroke="none"/></svg>',
    light: '<svg class="ic" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4"/></svg>',
    dark: '<svg class="ic" viewBox="0 0 24 24"><path d="M20 13.5A8.5 8.5 0 1110.5 4a7 7 0 009.5 9.5z"/></svg>'
  };
  const labels = { auto: '跟随系统', light: '浅色模式', dark: '深色模式' };
  let mode = localStorage.getItem(KEY) || 'auto';
  if (!order.includes(mode)) mode = 'auto';

  function apply(m) {
    mode = m;
    try { localStorage.setItem(KEY, m); } catch (e) {}
    if (m === 'auto') document.documentElement.removeAttribute('data-theme');
    else document.documentElement.dataset.theme = m;
    btn.innerHTML = icons[m];
    btn.title = labels[m] + '（点击切换）';
    btn.setAttribute('aria-label', labels[m]);
  }

  btn.addEventListener('click', () => {
    apply(order[(order.indexOf(mode) + 1) % order.length]);
  });
  apply(mode);
})();
