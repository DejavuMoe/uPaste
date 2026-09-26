(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const limitText = 1_048_576;
  const limitFile = 67_108_864;
  const token = 'up_o1_demo_token_not_valid';
  const linkBase = 'https://paste.example.invalid/s/demo-share';
  const words = {
    zh: {
      newShare: '新建分享', text: '文本', file: '文件', content: '内容', format: '格式', plain: '纯文本', source: '源码', markdown: 'Markdown',
      privacy: '隐私', standard: '标准', encrypted: '加密', expires: '到期', never: '永久', oneHour: '1 小时', oneDay: '1 天', sevenDays: '7 天', thirtyDays: '30 天', custom: '自定义…', customDate: '自定义到期时间',
      theme: '外观', system: '跟随系统', light: '浅色', dark: '深色', textPlaceholder: '粘贴或输入内容…', dropFile: '拖入一个文件', chooseFile: '选择文件', fileLimit: '单个文件，最大 64 MiB', remove: '移除',
      create: '创建分享', created: '分享已创建', shareLink: '分享链接', managementToken: '管理令牌', tokenNote: '请立即保存。令牌只显示一次；修改或删除分享时需要使用，uPaste 无法恢复它。', copyLink: '复制链接', copyToken: '复制令牌', copied: '已复制', copyManually: '复制失败，请手动复制', reveal: '显示', hide: '隐藏',
      openShare: '查看分享', manageShare: '管理分享', saveChanges: '保存更改', saved: '修改已保存', deleteShare: '删除分享', deleteTitle: '删除这个分享？', deleteQuestion: '分享及其内容将被永久删除。', deletePermanently: '永久删除', cancel: '取消', back: '返回', deleted: '分享已删除', deletedNote: '分享已无法访问。', downloadFile: '下载文件',
      encryptedNote: '内容在此浏览器中加密。持有完整链接的人可以阅读；服务器无法恢复丢失的解密密钥。', emptyText: '请输入内容。', largeText: '内容超过 1 MiB。', missingFile: '请选择一个文件。', emptyFile: '文件不能为空。', largeFile: '文件超过 64 MiB。', manyFiles: '仅支持一个文件，已选择第一个。', futureDate: '请选择未来的到期时间。', unsavedTitle: '有未保存的更改', unsavedQuestion: '确定要放弃草稿吗？', keepEditing: '继续编辑', discard: '放弃并离开',
    },
    en: {
      newShare: 'New share', text: 'Text', file: 'File', content: 'Content', format: 'Format', plain: 'Plain text', source: 'Source', markdown: 'Markdown',
      privacy: 'Privacy', standard: 'Standard', encrypted: 'Encrypted', expires: 'Expires', never: 'Never', oneHour: '1 hour', oneDay: '1 day', sevenDays: '7 days', thirtyDays: '30 days', custom: 'Custom…', customDate: 'Custom expiration date and time',
      theme: 'Theme', system: 'System', light: 'Light', dark: 'Dark', textPlaceholder: 'Paste or type content…', dropFile: 'Drop one file here', chooseFile: 'Choose file', fileLimit: 'One file, up to 64 MiB', remove: 'Remove',
      create: 'Create share', created: 'Share created', shareLink: 'Share link', managementToken: 'Management token', tokenNote: 'Save this token now. It is shown once and is required to modify or delete this share. uPaste cannot recover it.', copyLink: 'Copy link', copyToken: 'Copy token', copied: 'Copied', copyManually: 'Copy failed; copy manually', reveal: 'Reveal', hide: 'Hide',
      openShare: 'Open share', manageShare: 'Manage share', saveChanges: 'Save changes', saved: 'Changes saved', deleteShare: 'Delete share', deleteTitle: 'Delete this share?', deleteQuestion: 'This permanently deletes the share and its content.', deletePermanently: 'Delete permanently', cancel: 'Cancel', back: 'Back', deleted: 'Share deleted', deletedNote: 'The share is no longer available.', downloadFile: 'Download file',
      encryptedNote: 'Encrypted in this browser. Anyone with the complete link can read it. The server cannot recover a lost decryption key.', emptyText: 'Enter content.', largeText: 'Content exceeds 1 MiB.', missingFile: 'Choose a file.', emptyFile: 'File cannot be empty.', largeFile: 'File exceeds 64 MiB.', manyFiles: 'Only one file is supported; the first was selected.', futureDate: 'Choose a future expiration time.', unsavedTitle: 'You have unsaved changes.', unsavedQuestion: 'Are you sure you want to discard your draft?', keepEditing: 'Keep editing', discard: 'Discard and leave',
    },
  };

  const params = new URLSearchParams(location.search);
  const state = {
    lang: 'zh', theme: 'system', screen: 'create', kind: 'text', text: '', file: null,
    format: 'PLAIN', privacy: 'STANDARD', expiry: 'never', customExpiry: '', error: '',
    result: null, tokenVisible: false, copyStatus: '', copied: '', manageText: '', manageFormat: 'PLAIN', manageExpiry: 'never',
    dialog: '', pending: '',
  };
  let lastRenderedScreen = null;
  if (params.get('demo') === 'filled' || params.get('demo') === 'result') {
    state.text = '周五安排\n\n09:30 集合\n10:00 出发\n\n带上车票、水和相机。\n如果时间有变化，直接回复这条消息。';
    $('text-content').value = state.text;
  }
  if (params.get('demo') === 'file') {
    state.kind = 'file';
    state.file = new File(['sample file'], '行程资料.txt', { type: 'text/plain' });
  }
  const tr = (key) => words[state.lang][key] || key;
  const byteLength = (text) => new TextEncoder().encode(text).length;
  const formatBytes = (n) => n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KiB`;
  const privacy = () => document.querySelector('input[name="privacy"]:checked').value;
  const expiry = () => $('expiry').value;
  const createDirty = () => state.screen === 'create' && (state.text.length > 0 || !!state.file);
  const manageDirty = () => state.screen === 'manage' && (state.manageText !== state.result?.text || state.manageFormat !== state.result?.format || state.manageExpiry !== state.result?.expiry);
  const validExpiry = () => expiry() !== 'custom' || !!state.customExpiry && new Date(state.customExpiry).getTime() > Date.now() + 60_000;
  const validContent = () => state.kind === 'text' ? byteLength(state.text) > 0 && byteLength(state.text) <= limitText : !!state.file && state.file.size > 0 && state.file.size <= limitFile;

  function setTheme() {
    const value = $('theme').value;
    const dark = value === 'dark' || value === 'system' && matchMedia('(prefers-color-scheme: dark)').matches;
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  }

  function fitEditor() {
    const editor = $('text-content');
    const minimum = innerWidth <= 370 ? 185 : innerWidth <= 700 ? 210 : 220;
    const maximum = Math.max(minimum, Math.min(560, innerHeight * .62));
    editor.style.height = 'auto';
    const contentHeight = editor.scrollHeight + 8;
    editor.style.height = `${Math.min(maximum, Math.max(minimum, contentHeight))}px`;
    editor.style.overflowY = contentHeight > maximum ? 'auto' : 'hidden';
  }

  function updateCopy() {
    document.querySelectorAll('[data-i18n]').forEach((node) => { node.textContent = tr(node.dataset.i18n); });
    document.documentElement.lang = state.lang === 'zh' ? 'zh-CN' : 'en';
    $('language-zh').setAttribute('aria-pressed', String(state.lang === 'zh'));
    $('language-en').setAttribute('aria-pressed', String(state.lang === 'en'));
    $('theme').setAttribute('aria-label', tr('theme'));
    $('file-input').setAttribute('aria-label', tr('chooseFile'));
    document.querySelector('.mode-tabs').setAttribute('aria-label', state.lang === 'zh' ? '内容类型' : 'Content type');
    $('text-content').placeholder = tr('textPlaceholder');
    $('toggle-token').textContent = tr(state.tokenVisible ? 'hide' : 'reveal');
    $('copy-link').textContent = tr(state.copied === 'link' ? 'copied' : 'copyLink');
    $('copy-token').textContent = tr(state.copied === 'token' ? 'copied' : 'copyToken');
    document.title = `${tr(state.screen === 'result' ? 'created' : state.screen === 'view' ? 'openShare' : state.screen === 'manage' ? 'manageShare' : state.screen === 'deleted' ? 'deleted' : 'newShare')} · uPaste`;
  }

  function showScreen(next) {
    const changed = lastRenderedScreen !== next;
    lastRenderedScreen = next;
    state.screen = next;
    for (const screen of ['create', 'result', 'view', 'manage', 'deleted']) $(`${screen}-screen`).hidden = screen !== next;
    $('page-heading').textContent = tr(next === 'result' ? 'created' : next === 'view' ? 'openShare' : next === 'manage' ? 'manageShare' : next === 'deleted' ? 'deleted' : 'newShare');
    updateCopy();
    if (changed && next !== 'create') $('page-heading').focus();
  }

  function render() {
    updateCopy();
    setTheme();
    $('mode-text').setAttribute('aria-selected', String(state.kind === 'text'));
    $('mode-file').setAttribute('aria-selected', String(state.kind === 'file'));
    $('mode-text').tabIndex = state.kind === 'text' ? 0 : -1;
    $('mode-file').tabIndex = state.kind === 'file' ? 0 : -1;
    $('text-panel').hidden = state.kind !== 'text';
    $('file-panel').hidden = state.kind !== 'file';
    $('format-control').hidden = state.kind !== 'text';
    $('privacy-control').hidden = state.kind !== 'text';
    $('create-controls').classList.toggle('file-mode', state.kind === 'file');
    $('file-empty').hidden = !!state.file;
    $('file-selected').hidden = !state.file;
    $('file-panel').classList.toggle('has-file', !!state.file);
    if (state.file) { $('file-name').textContent = state.file.name; $('file-size').textContent = formatBytes(state.file.size); }
    $('content-size').textContent = state.kind === 'text' ? `${formatBytes(byteLength(state.text))} / 1 MiB` : state.file ? `${formatBytes(state.file.size)} / 64 MiB` : '0 B / 64 MiB';
    $('text-content').dataset.format = state.format;
    fitEditor();
    $('custom-expiry-wrap').hidden = expiry() !== 'custom';
    $('encrypted-note').hidden = state.kind !== 'text' || privacy() !== 'ENCRYPTED';
    $('create').disabled = !validContent() || !validExpiry();
    $('create-error').hidden = !state.error;
    $('create-error').textContent = state.error ? tr(state.error) : '';
    if (state.result) {
      $('result-meta').textContent = `${tr(state.result.kind === 'file' ? 'file' : state.result.format === 'PLAIN' ? 'plain' : state.result.format === 'SOURCE' ? 'source' : 'markdown')} · ${tr(state.result.kind === 'file' ? 'standard' : state.result.privacy === 'ENCRYPTED' ? 'encrypted' : 'standard')} · ${tr(expiryLabel(state.result.expiry))}`;
      $('share-link').value = state.result.link;
      $('owner-token').value = state.tokenVisible ? token : 'up_o1_' + '•'.repeat(25);
      $('view-meta').textContent = $('result-meta').textContent;
      $('view-text').hidden = state.result.kind === 'file';
      $('view-file').hidden = state.result.kind !== 'file';
      $('view-text').textContent = state.result.text;
      if (state.result.file) { $('view-file-name').textContent = state.result.file.name; $('view-file-size').textContent = formatBytes(state.result.file.size); $('manage-file-name').textContent = state.result.file.name; $('manage-file-size').textContent = formatBytes(state.result.file.size); }
      $('manage-text-wrap').hidden = state.result.kind !== 'text';
      $('manage-file-wrap').hidden = state.result.kind !== 'file';
      $('manage-format-wrap').hidden = state.result.kind !== 'text';
      $('manage-expiry').value = state.manageExpiry;
      $('manage-format').value = state.manageFormat;
    }
    $('toggle-token').textContent = tr(state.tokenVisible ? 'hide' : 'reveal');
    $('copy-status').textContent = state.copyStatus ? tr(state.copyStatus) : '';
    showScreen(state.screen);
  }

  function expiryLabel(value) { return value === '1h' ? 'oneHour' : value === '1d' ? 'oneDay' : value === '7d' ? 'sevenDays' : value === '30d' ? 'thirtyDays' : value === 'custom' ? 'custom' : 'never'; }
  function create() {
    state.error = '';
    if (state.kind === 'text' && !state.text) state.error = 'emptyText';
    else if (state.kind === 'text' && byteLength(state.text) > limitText) state.error = 'largeText';
    else if (state.kind === 'file' && !state.file) state.error = 'missingFile';
    else if (state.kind === 'file' && state.file.size === 0) state.error = 'emptyFile';
    else if (state.kind === 'file' && state.file.size > limitFile) state.error = 'largeFile';
    else if (!validExpiry()) state.error = 'futureDate';
    if (state.error) { render(); return; }
    const mode = state.kind === 'file' ? 'STANDARD' : privacy();
    state.result = {
      kind: state.kind, text: state.text, file: state.file, format: state.format, privacy: mode, expiry: expiry(),
      link: linkBase + (mode === 'ENCRYPTED' ? '#up_e1_demo_key_not_valid' : ''),
    };
    state.manageText = state.text;
    state.manageFormat = state.format;
    state.manageExpiry = expiry();
    state.tokenVisible = false;
    state.screen = 'result';
    render();
  }

  async function copy(value, input, which) {
    try { await navigator.clipboard.writeText(value); state.copied = which; state.copyStatus = ''; }
    catch {
      if (input === $('owner-token')) state.tokenVisible = true;
      state.copied = '';
      state.copyStatus = 'copyManually';
      render();
      input.focus(); input.select();
      return;
    }
    render();
    setTimeout(() => { if (state.copied === which) { state.copied = ''; render(); } }, 1500);
  }

  function reset() {
    state.text = ''; state.file = null; state.kind = 'text'; state.format = 'PLAIN'; state.customExpiry = ''; state.error = ''; state.result = null; state.tokenVisible = false; state.copyStatus = ''; state.copied = '';
    $('text-content').value = ''; $('file-input').value = ''; $('format').value = 'PLAIN'; $('expiry').value = 'never'; $('custom-expiry').value = '';
    document.querySelector('input[name="privacy"][value="STANDARD"]').checked = true;
    state.screen = 'create'; render();
  }

  function openDialog(kind, pending) {
    state.dialog = kind; state.pending = pending;
    $('dialog-title').textContent = tr(kind === 'delete' ? 'deleteTitle' : 'unsavedTitle');
    $('dialog-message').textContent = tr(kind === 'delete' ? 'deleteQuestion' : 'unsavedQuestion');
    $('dialog-cancel').textContent = tr(kind === 'delete' ? 'cancel' : 'keepEditing');
    $('dialog-confirm').textContent = tr(kind === 'delete' ? 'deletePermanently' : 'discard');
    $('dialog-backdrop').hidden = false;
    $('main').inert = true; document.querySelector('.site-header').inert = true;
    $('dialog-cancel').focus();
  }

  function closeDialog() {
    $('dialog-backdrop').hidden = true;
    $('main').inert = false; document.querySelector('.site-header').inert = false;
    state.dialog = ''; state.pending = '';
    $('page-heading').focus();
  }

  function acceptFile(files) {
    if (!files?.length) return;
    state.file = files[0];
    state.error = files.length > 1 ? 'manyFiles' : state.file.size === 0 ? 'emptyFile' : state.file.size > limitFile ? 'largeFile' : '';
    render();
  }

  $('language-zh').addEventListener('click', () => { state.lang = 'zh'; render(); });
  $('language-en').addEventListener('click', () => { state.lang = 'en'; render(); });
  $('theme').addEventListener('change', setTheme);
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if ($('theme').value === 'system') setTheme(); });
  $('brand').addEventListener('click', () => { if (createDirty() || manageDirty()) openDialog('discard', 'create'); else reset(); });
  for (const kind of ['text', 'file']) $(`mode-${kind}`).addEventListener('click', () => { state.kind = kind; state.error = ''; render(); });
  document.querySelector('.mode-tabs').addEventListener('keydown', (event) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    state.kind = event.key === 'ArrowRight' || event.key === 'End' ? 'file' : 'text';
    render(); $(`mode-${state.kind}`).focus();
  });
  $('text-content').addEventListener('input', (event) => { state.text = event.target.value; state.error = ''; render(); });
  $('text-content').addEventListener('keydown', (event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); create(); } });
  $('format').addEventListener('change', (event) => { state.format = event.target.value; render(); });
  document.querySelectorAll('input[name="privacy"]').forEach((node) => node.addEventListener('change', render));
  $('expiry').addEventListener('change', render);
  $('custom-expiry').addEventListener('input', (event) => { state.customExpiry = event.target.value; render(); });
  $('choose-file').addEventListener('click', () => $('file-input').click());
  $('file-input').addEventListener('change', (event) => acceptFile(event.target.files));
  $('remove-file').addEventListener('click', () => { state.file = null; state.error = ''; $('file-input').value = ''; render(); });
  for (const name of ['dragenter', 'dragover']) $('file-panel').addEventListener(name, (event) => { event.preventDefault(); $('file-panel').classList.add('is-dragging'); });
  for (const name of ['dragleave', 'drop']) $('file-panel').addEventListener(name, (event) => { event.preventDefault(); $('file-panel').classList.remove('is-dragging'); });
  $('file-panel').addEventListener('drop', (event) => acceptFile(event.dataTransfer.files));
  $('create').addEventListener('click', create);
  $('toggle-token').addEventListener('click', () => { state.tokenVisible = !state.tokenVisible; render(); });
  $('copy-link').addEventListener('click', () => void copy(state.result.link, $('share-link'), 'link'));
  $('copy-token').addEventListener('click', () => void copy(token, $('owner-token'), 'token'));
  $('open-share').addEventListener('click', () => { state.screen = 'view'; render(); });
  $('manage-share').addEventListener('click', () => { state.screen = 'manage'; $('manage-text').value = state.manageText; render(); });
  $('new-share').addEventListener('click', reset);
  $('view-new').addEventListener('click', reset);
  $('download-file').addEventListener('click', () => { if (!state.result?.file) return; const url = URL.createObjectURL(state.result.file); const link = document.createElement('a'); link.href = url; link.download = state.result.file.name; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); });
  $('manage-text').addEventListener('input', (event) => { state.manageText = event.target.value; $('manage-status').textContent = ''; });
  $('manage-text').addEventListener('keydown', (event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); $('save-changes').click(); } });
  $('manage-format').addEventListener('change', (event) => { state.manageFormat = event.target.value; });
  $('manage-expiry').addEventListener('change', (event) => { state.manageExpiry = event.target.value; });
  $('save-changes').addEventListener('click', () => {
    if (state.result.kind === 'text' && (!byteLength(state.manageText) || byteLength(state.manageText) > limitText)) { $('manage-status').textContent = tr(state.manageText ? 'largeText' : 'emptyText'); return; }
    state.result.text = state.manageText; state.result.format = state.manageFormat; state.result.expiry = state.manageExpiry;
    $('manage-status').textContent = tr('saved'); render();
  });
  $('manage-back').addEventListener('click', () => { if (manageDirty()) openDialog('discard', 'view'); else { state.screen = 'view'; render(); } });
  $('delete-share').addEventListener('click', () => openDialog('delete', 'deleted'));
  $('deleted-new').addEventListener('click', reset);
  $('dialog-cancel').addEventListener('click', closeDialog);
  $('dialog-confirm').addEventListener('click', () => { const kind = state.dialog, next = state.pending; closeDialog(); if (kind === 'delete') { state.result = null; state.screen = 'deleted'; render(); } else if (next === 'create') reset(); else { state.manageText = state.result.text; state.manageFormat = state.result.format; state.manageExpiry = state.result.expiry; state.screen = next; render(); } });
  $('dialog-backdrop').addEventListener('keydown', (event) => { if (event.key === 'Escape') { event.preventDefault(); closeDialog(); } });
  window.addEventListener('beforeunload', (event) => { if (createDirty() || manageDirty()) { event.preventDefault(); event.returnValue = ''; } });
  window.addEventListener('resize', fitEditor);

  if (params.get('demo') === 'result') create();
  else render();
})();
