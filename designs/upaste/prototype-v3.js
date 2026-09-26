(() => {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const params = new URLSearchParams(location.search);
  const fixture = {
    plain: '周五安排\n\n09:30 集合\n10:00 出发\n\n带上车票、水和相机。\n如果时间有变化，直接回复这条消息。',
    source: 'export function shareLink(id) {\n  const origin = new URL("https://paste.example.invalid");\n  return new URL(`/s/${id}`, origin).toString();\n}\n\nconst sample = "long-" + "reference-".repeat(22);\nconsole.log(shareLink("demo-share"), sample);',
    markdown: '# 周五安排\n\n09:30 集合，10:00 出发。\n\n## 物品\n\n- 车票\n- 水\n- 相机\n\n| 时间 | 事项 |\n| --- | --- |\n| 09:30 | 集合 |\n| 10:00 | 出发 |\n\n`const ready = true;`\n\n![路线图](https://example.invalid/map.png)',
    filename: '行程资料.txt', mime: 'text/plain',
  };
  const fileBlob = new Blob(['sample file\n'], { type: fixture.mime });
  const copy = {
    zh: {
      theme: '外观', system: '跟随系统', light: '浅色', dark: '深色', newShare: '新建分享', openShare: '查看分享', manageShare: '管理分享',
      plain: '纯文本', source: '源码', markdown: 'Markdown', file: '文件', standard: '标准', encrypted: '加密', never: '永久', oneHour: '1 小时', oneDay: '1 天', sevenDays: '7 天', thirtyDays: '30 天', custom: '自定义…',
      expires: '到期', size: '大小', mime: '类型', format: '格式', content: '内容', rendered: '预览', sourceView: '源码', wrap: '自动换行', unwrap: '不换行', copyContent: '复制内容', copied: '已复制', copyFailed: '复制失败，请手动选择内容。', raw: 'Raw', download: '下载文件', downloadUnavailable: '下载地址不可用。',
      managementToken: '管理令牌', tokenHelp: '请输入创建分享时保存的管理令牌。', continue: '继续', tokenRequired: '请输入管理令牌。', tokenRejected: '管理令牌未被接受，请重试。',
      saveChanges: '保存更改', saving: '正在保存…', saved: '修改已保存', backToShare: '返回分享', deleteShare: '删除分享', deleteTitle: '删除这个分享？', deleteQuestion: '分享及其内容将被永久删除。', deletePermanently: '永久删除', cancel: '取消', deleted: '分享已删除', deletedHelp: '此分享已无法访问。',
      customDate: '自定义到期时间', futureDate: '请选择未来的到期时间。', publicLimit: (date) => `最晚到期时间：${date}`, beyondLimit: '到期时间不能超过公开分享的保留上限。', emptyText: '请输入内容。', largeText: '内容超过 1 MiB。',
      noKeyManage: '此链接缺少解密密钥。仍可修改到期时间或删除分享。', wrongKeyManage: '链接中的解密密钥无法打开内容。仍可修改到期时间或删除分享。',
      unsavedTitle: '有未保存的更改', unsavedQuestion: '确定要放弃草稿吗？', keepEditing: '继续编辑', discard: '放弃并离开',
      loading: '正在加载分享…', decrypting: '正在解密内容…', notfoundTitle: '找不到这个分享', notfoundMessage: '请检查链接是否完整。', expiredTitle: '分享已到期', expiredMessage: '此分享已无法访问。', ratelimitTitle: '请求过于频繁', ratelimitMessage: '请等待 30 秒后重试。', networkTitle: '无法连接', networkMessage: '请检查网络连接后重试。', serverTitle: '暂时无法打开分享', serverMessage: '请稍后重试。', missingKeyTitle: '链接缺少解密密钥', missingKeyMessage: '请向分享者索取完整链接。管理令牌无法解密内容。', wrongKeyTitle: '无法解密内容', wrongKeyMessage: '链接中的解密密钥无效，或内容已损坏。请向分享者确认完整链接。', retry: '重试', unauthorizedSave: '管理令牌未被接受。草稿已保留，请重新输入令牌。', rateSave: '请求过于频繁，请 30 秒后重试。', serverSave: '保存失败，请重试。',
    },
    en: {
      theme: 'Theme', system: 'System', light: 'Light', dark: 'Dark', newShare: 'New share', openShare: 'Open share', manageShare: 'Manage share',
      plain: 'Plain text', source: 'Source', markdown: 'Markdown', file: 'File', standard: 'Standard', encrypted: 'Encrypted', never: 'Never', oneHour: '1 hour', oneDay: '1 day', sevenDays: '7 days', thirtyDays: '30 days', custom: 'Custom…',
      expires: 'Expires', size: 'Size', mime: 'Type', format: 'Format', content: 'Content', rendered: 'Rendered', sourceView: 'Source', wrap: 'Wrap lines', unwrap: 'Do not wrap', copyContent: 'Copy content', copied: 'Copied', copyFailed: 'Copy failed. Select the content to copy it manually.', raw: 'Raw', download: 'Download file', downloadUnavailable: 'Download unavailable.',
      managementToken: 'Management token', tokenHelp: 'Enter the management token saved when this share was created.', continue: 'Continue', tokenRequired: 'Enter a management token.', tokenRejected: 'Management token was not accepted. Try again.',
      saveChanges: 'Save changes', saving: 'Saving…', saved: 'Changes saved', backToShare: 'Back to share', deleteShare: 'Delete share', deleteTitle: 'Delete this share?', deleteQuestion: 'This permanently deletes the share and its content.', deletePermanently: 'Delete permanently', cancel: 'Cancel', deleted: 'Share deleted', deletedHelp: 'This share is no longer available.',
      customDate: 'Custom expiration date and time', futureDate: 'Choose a future expiration time.', publicLimit: (date) => `Latest expiration: ${date}`, beyondLimit: 'Expiration cannot exceed the public retention limit.', emptyText: 'Enter content.', largeText: 'Content exceeds 1 MiB.',
      noKeyManage: 'This link has no decryption key. You can still change expiration or delete the share.', wrongKeyManage: 'This link cannot decrypt the content. You can still change expiration or delete the share.',
      unsavedTitle: 'You have unsaved changes.', unsavedQuestion: 'Discard your draft?', keepEditing: 'Keep editing', discard: 'Discard and leave',
      loading: 'Loading share…', decrypting: 'Decrypting content…', notfoundTitle: 'Share not found', notfoundMessage: 'Check that the link is complete.', expiredTitle: 'Share expired', expiredMessage: 'This share is no longer available.', ratelimitTitle: 'Too many requests', ratelimitMessage: 'Wait 30 seconds before trying again.', networkTitle: 'Unable to connect', networkMessage: 'Check your connection and try again.', serverTitle: 'Share temporarily unavailable', serverMessage: 'Try again shortly.', missingKeyTitle: 'Decryption key missing', missingKeyMessage: 'Ask the sender for the complete link. A management token cannot decrypt the content.', wrongKeyTitle: 'Cannot decrypt content', wrongKeyMessage: 'The decryption key is invalid or the content has changed. Confirm the complete link with the sender.', retry: 'Try again', unauthorizedSave: 'Management token was not accepted. Your draft was kept; enter the token again.', rateSave: 'Too many requests. Try again in 30 seconds.', serverSave: 'Save failed. Try again.',
    },
  };
  const kind = ['plain', 'source', 'markdown', 'file', 'encrypted', 'encrypted-source', 'encrypted-markdown'].includes(params.get('kind')) ? params.get('kind') : 'plain';
  const encrypted = kind.startsWith('encrypted');
  const textKind = kind === 'encrypted' ? 'plain' : kind === 'encrypted-source' ? 'source' : kind === 'encrypted-markdown' ? 'markdown' : kind;
  const initialScreen = params.get('screen') === 'manage' ? 'manage' : 'read';
  const initialError = params.get('state') || '';
  const publicMode = params.get('public') === '1';
  const keyState = params.get('key') || (initialError === 'missing-key' ? 'missing' : initialError === 'wrong-key' ? 'wrong' : 'present');
  const createdAt = Date.now() - 60 * 60 * 1000;
  const publicMax = createdAt + 168 * 60 * 60 * 1000;
  const state = {
    lang: 'zh', screen: initialScreen, kind, textKind, encrypted, publicMode, keyState,
    error: initialError, token: params.get('token') || (initialScreen === 'manage' ? 'missing' : 'present'),
    markdownView: 'rendered', wrapped: false, content: fixture[textKind] || fixture.plain,
    manageText: fixture[textKind] || fixture.plain, format: textKind === 'file' ? 'PLAIN' : textKind.toUpperCase(),
    manageFormat: textKind === 'file' ? 'PLAIN' : textKind.toUpperCase(), expiry: publicMode ? '1d' : 'never', manageExpiry: publicMode ? '1d' : 'never', custom: '', originalCustom: '',
    saveStatus: '', saveFailure: params.get('failure') || '', readStatus: '', dialog: '', dialogDestination: '', rawUrl: '', lastScreen: '',
  };
  const tr = (key) => copy[state.lang][key] || key;
  const expiryKey = (value) => ({ never: 'never', '1h': 'oneHour', '1d': 'oneDay', '7d': 'sevenDays', '30d': 'thirtyDays', custom: 'custom' })[value] || 'never';
  const formatBytes = (bytes) => bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KiB`;
  const dateLabel = (ms) => new Intl.DateTimeFormat(state.lang === 'zh' ? 'zh-CN' : 'en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(ms);
  const byteLength = (value) => new TextEncoder().encode(value).length;
  const editable = () => textKind !== 'file' && (!encrypted || state.keyState === 'present');
  const dirty = () => state.screen === 'manage' && state.token === 'present' && (editable() && (state.manageText !== state.content || state.manageFormat !== state.format) || state.manageExpiry !== state.expiry || state.custom !== state.originalCustom);
  const expiresAt = () => state.manageExpiry === 'custom' ? new Date(state.custom).getTime() : state.manageExpiry === 'never' ? Infinity : Date.now() + ({ '1h': 1, '1d': 24, '7d': 168, '30d': 720 })[state.manageExpiry] * 3_600_000;
  const validExpiry = () => {
    const time = expiresAt();
    return state.manageExpiry === 'never' ? !publicMode : Number.isFinite(time) && time > Date.now() && (!publicMode || time <= publicMax);
  };
  function setTheme() {
    const value = $('theme').value;
    const dark = value === 'dark' || value === 'system' && matchMedia('(prefers-color-scheme: dark)').matches;
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  }
  function showScreen(next) {
    const changed = state.lastScreen !== next;
    state.lastScreen = next;
    state.screen = next;
    for (const name of ['read', 'state', 'manage', 'deleted']) $(`${name}-screen`).hidden = name !== next;
    $('page-heading').textContent = tr(next === 'manage' ? 'manageShare' : next === 'deleted' ? 'deleted' : 'openShare');
    document.title = `${$('page-heading').textContent} · uPaste`;
    document.documentElement.lang = state.lang === 'zh' ? 'zh-CN' : 'en';
    if (changed && next === 'manage' && state.token !== 'present') $('owner-input').focus();
    else if (changed) $('page-heading').focus();
  }
  function metadata() {
    const format = textKind === 'file' ? tr('file') : tr(textKind === 'plain' ? 'plain' : textKind === 'source' ? 'source' : 'markdown');
    return `${format} · ${tr(encrypted ? 'encrypted' : 'standard')} · ${tr(expiryKey(state.expiry))}`;
  }
  function paintMarkdown(container) {
    container.replaceChildren();
    const heading = document.createElement('h2'); heading.textContent = '周五安排'; container.append(heading);
    const paragraph = document.createElement('p'); paragraph.textContent = '09:30 集合，10:00 出发。'; container.append(paragraph);
    const subheading = document.createElement('h3'); subheading.textContent = '物品'; container.append(subheading);
    const list = document.createElement('ul');
    for (const item of ['车票', '水', '相机']) { const li = document.createElement('li'); li.textContent = item; list.append(li); }
    container.append(list);
    const table = document.createElement('table');
    const rows = [['时间', '事项'], ['09:30', '集合'], ['10:00', '出发']];
    rows.forEach((cells, index) => { const row = table.insertRow(); cells.forEach((value) => { const cell = index ? row.insertCell() : document.createElement('th'); cell.textContent = value; if (!index) row.append(cell); }); });
    container.append(table);
    const code = document.createElement('pre'); code.textContent = 'const ready = true;'; container.append(code);
    const imageLink = document.createElement('a'); imageLink.className = 'image-link'; imageLink.href = 'https://example.invalid/map.png'; imageLink.target = '_blank'; imageLink.rel = 'noopener'; imageLink.textContent = '路线图（图片链接）'; container.append(imageLink);
  }
  function renderRead() {
    $('read-meta').textContent = metadata();
    $('read-toolbar').hidden = textKind === 'file';
    $('read-content').hidden = textKind === 'file';
    $('read-file').hidden = textKind !== 'file';
    $('markdown-switch').hidden = textKind !== 'markdown';
    $('wrap-button').hidden = textKind !== 'source';
    $('wrap-button').setAttribute('aria-pressed', String(state.wrapped));
    $('wrap-button').textContent = tr(state.wrapped ? 'unwrap' : 'wrap');
    $('rendered-button').setAttribute('aria-pressed', String(state.markdownView === 'rendered'));
    $('source-button').setAttribute('aria-pressed', String(state.markdownView === 'source'));
    $('raw-link').hidden = encrypted || textKind === 'file';
    const container = $('read-content');
    container.className = 'reading-panel';
    if (textKind === 'markdown' && state.markdownView === 'rendered') { container.classList.add('markdown-rendered'); paintMarkdown(container); }
    else { const pre = document.createElement('pre'); pre.textContent = state.content; container.replaceChildren(pre); if (textKind === 'source') container.classList.add('source-content'); if (textKind === 'markdown') container.classList.add('markdown-source'); }
    if (state.wrapped) container.classList.add('is-wrapped');
    if (state.rawUrl) URL.revokeObjectURL(state.rawUrl);
    state.rawUrl = !encrypted && textKind !== 'file' ? URL.createObjectURL(new Blob([state.content], { type: 'text/plain;charset=utf-8' })) : '';
    $('raw-link').href = state.rawUrl || '#';
    $('read-file-name').textContent = fixture.filename;
    $('read-file-size').textContent = formatBytes(fileBlob.size);
    $('read-file-mime').textContent = fixture.mime;
    $('read-file-expiry').textContent = tr(expiryKey(state.expiry));
    $('download-file').disabled = state.error === 'unsafe-file';
    $('read-status').textContent = state.error === 'unsafe-file' ? tr('downloadUnavailable') : state.readStatus ? tr(state.readStatus) : '';
  }
  function renderState() {
    const errors = {
      loading: ['loading', ''], decrypting: ['decrypting', ''], notfound: ['notfoundTitle', 'notfoundMessage'], expired: ['expiredTitle', 'expiredMessage'], ratelimit: ['ratelimitTitle', 'ratelimitMessage'], network: ['networkTitle', 'networkMessage'], server: ['serverTitle', 'serverMessage'], 'missing-key': ['missingKeyTitle', 'missingKeyMessage'], 'wrong-key': ['wrongKeyTitle', 'wrongKeyMessage'],
    };
    const current = errors[state.error] || errors.server;
    $('state-title').textContent = tr(current[0]); $('state-message').textContent = current[1] ? tr(current[1]) : '';
    $('state-retry').hidden = !['ratelimit', 'network', 'server'].includes(state.error);
  }
  function renderManage() {
    $('manage-meta').textContent = metadata();
    $('token-gate').hidden = state.token === 'present';
    $('manage-body').hidden = state.token !== 'present';
    $('gate-error').hidden = state.token !== 'wrong';
    $('gate-error').textContent = state.token === 'wrong' ? tr(state.saveStatus === 'unauthorized' ? 'unauthorizedSave' : 'tokenRejected') : '';
    const canEdit = editable();
    $('manage-text-wrap').hidden = !canEdit;
    $('manage-format-wrap').hidden = !canEdit;
    $('manage-file-wrap').hidden = textKind !== 'file';
    $('manage-key-note').hidden = !encrypted || canEdit;
    $('manage-key-note').textContent = encrypted && !canEdit ? tr(state.keyState === 'wrong' ? 'wrongKeyManage' : 'noKeyManage') : '';
    $('manage-text').value = state.manageText;
    $('manage-format').value = state.manageFormat;
    $('manage-file-name').textContent = fixture.filename;
    $('manage-file-size').textContent = formatBytes(fileBlob.size);
    $('manage-file-mime').textContent = fixture.mime;
    $('manage-size').textContent = `${formatBytes(byteLength(state.manageText))} / 1 MiB`;
    $('manage-expiry').value = state.manageExpiry;
    $('manage-custom-wrap').hidden = state.manageExpiry !== 'custom';
    $('manage-custom').value = state.custom;
    $('retention-note').hidden = !publicMode;
    $('retention-note').textContent = publicMode ? tr('publicLimit')(dateLabel(publicMax)) : '';
    let validationError = '';
    if (!validExpiry()) validationError = publicMode && expiresAt() > publicMax ? tr('beyondLimit') : tr('futureDate');
    if (canEdit && byteLength(state.manageText) > 1_048_576) validationError = tr('largeText');
    if (canEdit && !byteLength(state.manageText)) validationError = tr('emptyText');
    const error = state.saveStatus === 'rate' ? tr('rateSave') : state.saveStatus === 'server' ? tr('serverSave') : validationError;
    $('manage-error').hidden = !error;
    $('manage-error').textContent = error;
    $('manage-status').textContent = state.saveStatus === 'saved' ? tr('saved') : '';
    $('save-changes').textContent = tr(state.saveStatus === 'saving' ? 'saving' : 'saveChanges');
    $('save-changes').disabled = !dirty() || !!validationError || state.saveStatus === 'saving';
  }
  function render() {
    document.querySelectorAll('[data-i18n]').forEach((node) => { node.textContent = tr(node.dataset.i18n); });
    $('language-zh').setAttribute('aria-pressed', String(state.lang === 'zh'));
    $('language-en').setAttribute('aria-pressed', String(state.lang === 'en'));
    $('theme').setAttribute('aria-label', tr('theme'));
    $('read-content').setAttribute('aria-label', tr('content'));
    $('markdown-switch').setAttribute('aria-label', `Markdown ${tr('sourceView')}`);
    setTheme();
    if (state.screen === 'read') renderRead();
    if (state.screen === 'state') renderState();
    if (state.screen === 'manage') renderManage();
    showScreen(state.screen);
  }
  function openDialog(kind, destination) {
    state.dialog = kind; state.dialogDestination = destination;
    $('dialog-title').textContent = tr(kind === 'delete' ? 'deleteTitle' : 'unsavedTitle');
    $('dialog-message').textContent = tr(kind === 'delete' ? 'deleteQuestion' : 'unsavedQuestion');
    $('dialog-cancel').textContent = tr(kind === 'delete' ? 'cancel' : 'keepEditing');
    $('dialog-confirm').textContent = tr(kind === 'delete' ? 'deletePermanently' : 'discard');
    $('dialog-backdrop').hidden = false; $('main').inert = true; document.querySelector('.site-header').inert = true; document.body.style.overflow = 'hidden'; $('dialog-cancel').focus();
  }
  function closeDialog() {
    $('dialog-backdrop').hidden = true; $('main').inert = false; document.querySelector('.site-header').inert = false; document.body.style.overflow = '';
    state.dialog = ''; state.dialogDestination = ''; $('page-heading').focus();
  }
  function tryLeave(destination) {
    if (dirty()) openDialog('discard', destination);
    else if (destination === 'create') location.href = 'prototype-v2.html';
    else { state.screen = 'read'; state.error = encrypted && state.keyState !== 'present' ? state.keyState === 'wrong' ? 'wrong-key' : 'missing-key' : ''; if (state.error) state.screen = 'state'; render(); }
  }

  $('language-zh').addEventListener('click', () => { state.lang = 'zh'; render(); });
  $('language-en').addEventListener('click', () => { state.lang = 'en'; render(); });
  $('theme').addEventListener('change', setTheme);
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if ($('theme').value === 'system') setTheme(); });
  $('brand').addEventListener('click', (event) => { if (dirty()) { event.preventDefault(); tryLeave('create'); } });
  document.querySelector('.new-link').addEventListener('click', (event) => { if (dirty()) { event.preventDefault(); tryLeave('create'); } });
  $('markdown-switch').addEventListener('click', (event) => { if (event.target.id === 'rendered-button' || event.target.id === 'source-button') { state.markdownView = event.target.id === 'rendered-button' ? 'rendered' : 'source'; render(); } });
  $('wrap-button').addEventListener('click', () => { state.wrapped = !state.wrapped; render(); });
  $('copy-content').addEventListener('click', async () => { try { await navigator.clipboard.writeText(state.content); state.readStatus = 'copied'; } catch { state.readStatus = 'copyFailed'; } render(); });
  $('download-file').addEventListener('click', () => { if (state.error === 'unsafe-file') return; const url = URL.createObjectURL(fileBlob); const link = document.createElement('a'); link.href = url; link.download = fixture.filename; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); });
  $('state-retry').addEventListener('click', () => { state.error = 'loading'; render(); setTimeout(() => { state.error = ''; state.screen = 'read'; render(); }, 550); });
  $('owner-continue').addEventListener('click', () => { if (!$('owner-input').value.trim()) { $('gate-error').hidden = false; $('gate-error').textContent = tr('tokenRequired'); return; } state.token = 'present'; state.saveStatus = ''; render(); $(editable() ? 'manage-text' : 'manage-expiry').focus(); });
  $('owner-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') $('owner-continue').click(); });
  $('manage-text').addEventListener('input', (event) => { state.manageText = event.target.value; state.saveStatus = ''; renderManage(); });
  $('manage-text').addEventListener('keydown', (event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); $('save-changes').click(); } });
  $('manage-format').addEventListener('change', (event) => { state.manageFormat = event.target.value; state.saveStatus = ''; renderManage(); });
  $('manage-expiry').addEventListener('change', (event) => { state.manageExpiry = event.target.value; state.saveStatus = ''; renderManage(); });
  $('manage-custom').addEventListener('input', (event) => { state.custom = event.target.value; state.saveStatus = ''; renderManage(); });
  $('save-changes').addEventListener('click', () => {
    if ($('save-changes').disabled) return;
    state.saveStatus = 'saving'; renderManage();
    setTimeout(() => {
      const failure = state.saveFailure; state.saveFailure = '';
      if (failure === 'unauthorized') { state.token = 'wrong'; state.saveStatus = 'unauthorized'; render(); return; }
      if (failure === 'notfound' || failure === 'expired') { state.error = failure; state.screen = 'state'; render(); return; }
      if (failure === 'rate' || failure === 'server') { state.saveStatus = failure; renderManage(); return; }
      state.content = state.manageText; state.format = state.manageFormat; state.expiry = state.manageExpiry; state.originalCustom = state.custom; state.saveStatus = 'saved'; render();
    }, 550);
  });
  $('manage-back').addEventListener('click', () => tryLeave('read'));
  $('delete-share').addEventListener('click', () => openDialog('delete', 'deleted'));
  $('dialog-cancel').addEventListener('click', closeDialog);
  $('dialog-confirm').addEventListener('click', () => { const kind = state.dialog, destination = state.dialogDestination; closeDialog(); if (kind === 'delete') { state.token = 'missing'; state.content = ''; state.manageText = ''; state.keyState = 'missing'; state.screen = 'deleted'; render(); } else if (destination === 'create') location.href = 'prototype-v2.html'; else { state.manageText = state.content; state.manageFormat = state.format; state.manageExpiry = state.expiry; state.custom = state.originalCustom; state.error = encrypted && state.keyState !== 'present' ? state.keyState === 'wrong' ? 'wrong-key' : 'missing-key' : ''; state.screen = state.error ? 'state' : 'read'; render(); } });
  $('dialog-backdrop').addEventListener('keydown', (event) => {
    if (event.key === 'Escape') { event.preventDefault(); closeDialog(); }
    if (event.key === 'Tab') { const first = $('dialog-cancel'), last = $('dialog-confirm'); if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); } }
  });
  window.addEventListener('beforeunload', (event) => { if (dirty()) { event.preventDefault(); event.returnValue = ''; } });

  if (publicMode) for (const value of ['never', '30d']) $('manage-expiry').querySelector(`option[value="${value}"]`).remove();
  if (initialScreen === 'read' && initialError && initialError !== 'unsafe-file') state.screen = 'state';
  if (initialScreen === 'manage' && ['notfound', 'expired', 'network', 'server'].includes(initialError)) state.screen = 'state';
  if (textKind === 'file') state.content = state.manageText = '';
  render();
})();
