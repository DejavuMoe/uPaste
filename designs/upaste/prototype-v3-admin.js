(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const words = {
    zh: {
      heading: '管理', theme: '外观', system: '跟随系统', light: '浅色', dark: '深色',
      unavailable: '管理入口不可用', token: '管理员令牌', signIn: '登录', logout: '退出',
      authenticationFailed: '令牌未被接受。', rateLimited: '尝试次数过多，请稍后再试。', sessionExpired: '会话已结束，请重新登录。',
      summary: '概览', active: '有效', expired: '已到期', text: '文本', file: '文件', encrypted: '加密', fileBytes: '文件占用',
      shares: '分享', type: '类型', privacy: '隐私', state: '状态', sort: '排序', id: 'ID',
      allTypes: '全部类型', allPrivacy: '全部隐私', allStates: '全部状态', standard: '标准',
      newest: '最新创建', oldest: '最早创建', exactId: '精确分享 ID', find: '查找', clear: '清除',
      invalidId: '分享 ID 应为 22 个 URL 安全字符。', select: '选择', selected: '已选', actions: '操作',
      created: '创建', expires: '到期', size: '大小', never: '永久', inspect: '查看',
      loading: '正在加载分享…', loadingDetail: '正在加载分享详情…', loadError: '无法加载分享。',
      detailError: '无法加载分享详情。', retry: '重试', empty: '没有符合条件的分享。',
      clearFilters: '清除筛选', loadMore: '加载更多', cleanup: '清理到期分享',
      deleteSelected: '删除所选', detail: '分享详情', close: '关闭', deleteShare: '删除分享',
      deleteTitle: '删除这个分享？', bulkTitle: '删除所选分享？',
      deleteQuestion: '分享及其内容将被永久删除，此操作无法撤销。',
      bulkQuestion: '所选分享及其内容将被永久删除，此操作无法撤销。',
      cleanupTitle: '清理到期分享？', cleanupQuestion: '到期分享将被清理。',
      cancel: '取消', deletePermanently: '永久删除', confirmCleanup: '清理',
      deletedCount: (count) => `已删除 ${count} 个分享。`,
      partialCount: (deleted, failed) => `已删除 ${deleted} 个分享；${failed} 个未能删除。`,
      purgedCount: (count) => `已清理 ${count} 个到期分享。`,
      results: (count) => `${count} 项`, selectedCount: (count) => `已选 ${count} 项`,
      standardText: '标准文本', encryptedText: '加密文本', plaintextUnavailable: '服务器无法查看明文。',
      fileMetadata: '文件信息', filename: '文件名', mediaType: '媒体类型', sha256: 'SHA-256', download: '下载文件',
      plain: '纯文本', source: '源码', markdown: 'Markdown',
    },
    en: {
      heading: 'Administration', theme: 'Theme', system: 'System', light: 'Light', dark: 'Dark',
      unavailable: 'Administration unavailable', token: 'Admin token', signIn: 'Sign in', logout: 'Log out',
      authenticationFailed: 'Token was not accepted.', rateLimited: 'Too many attempts. Try again shortly.', sessionExpired: 'Session ended. Sign in again.',
      summary: 'Summary', active: 'Active', expired: 'Expired', text: 'Text', file: 'File', encrypted: 'Encrypted', fileBytes: 'Stored files',
      shares: 'Shares', type: 'Type', privacy: 'Privacy', state: 'State', sort: 'Sort', id: 'ID',
      allTypes: 'All types', allPrivacy: 'All privacy', allStates: 'All states', standard: 'Standard',
      newest: 'Newest', oldest: 'Oldest', exactId: 'Exact Share ID', find: 'Find', clear: 'Clear',
      invalidId: 'Share IDs are 22 URL-safe characters.', select: 'Select', selected: 'Selected', actions: 'Actions',
      created: 'Created', expires: 'Expires', size: 'Size', never: 'Never', inspect: 'Inspect',
      loading: 'Loading shares…', loadingDetail: 'Loading Share detail…', loadError: 'Could not load shares.',
      detailError: 'Could not load Share detail.', retry: 'Retry', empty: 'No shares match these filters.',
      clearFilters: 'Clear filters', loadMore: 'Load more', cleanup: 'Clean expired',
      deleteSelected: 'Delete selected', detail: 'Share detail', close: 'Close', deleteShare: 'Delete share',
      deleteTitle: 'Delete this share?', bulkTitle: 'Delete selected shares?',
      deleteQuestion: 'This permanently deletes the share and its content. This action cannot be undone.',
      bulkQuestion: 'This permanently deletes the selected shares and their content. This action cannot be undone.',
      cleanupTitle: 'Clean expired content?', cleanupQuestion: 'Expired shares will be cleaned up.',
      cancel: 'Cancel', deletePermanently: 'Delete permanently', confirmCleanup: 'Clean expired',
      deletedCount: (count) => `Deleted ${count} share${count === 1 ? '' : 's'}.`,
      partialCount: (deleted, failed) => `Deleted ${deleted} share${deleted === 1 ? '' : 's'}; ${failed} could not be deleted.`,
      purgedCount: (count) => `Cleaned up ${count} expired share${count === 1 ? '' : 's'}.`,
      results: (count) => `${count} result${count === 1 ? '' : 's'}`, selectedCount: (count) => `${count} selected`,
      standardText: 'Standard text', encryptedText: 'Encrypted text', plaintextUnavailable: 'Plaintext is unavailable to the server.',
      fileMetadata: 'File metadata', filename: 'Filename', mediaType: 'Media type', sha256: 'SHA-256', download: 'Download file',
      plain: 'Plain text', source: 'Source', markdown: 'Markdown',
    },
  };

  const IDs = {
    a: 'AAAAAAAAAAAAAAAAAAAAAA', b: 'BBBBBBBBBBBBBBBBBBBBBA',
    c: 'CCCCCCCCCCCCCCCCCCCCCA', d: 'DDDDDDDDDDDDDDDDDDDDDA',
    e: 'EEEEEEEEEEEEEEEEEEEEEA', f: 'FFFFFFFFFFFFFFFFFFFFFA',
    missing: 'ZZZZZZZZZZZZZZZZZZZZZA',
  };
  const day = 86_400_000;
  const now = Date.now();
  const stamp = (offset) => new Date(now + offset * day).toISOString();
  const sampleShares = [
    { id: IDs.a, kind: 'TEXT', privacy: 'STANDARD', format: 'PLAIN', text: { zh: '周五安排\n09:30 集合\n10:00 出发', en: 'Friday schedule\n09:30 Meet\n10:00 Depart' }, bytes: 48, created: stamp(-1), expires: stamp(1) },
    { id: IDs.b, kind: 'TEXT', privacy: 'ENCRYPTED', bytes: 184, created: stamp(-2), expires: stamp(2) },
    { id: IDs.c, kind: 'FILE', privacy: 'STANDARD', filename: 'schedule.pdf', mediaType: 'application/pdf', sha256: 'a'.repeat(64), bytes: 348_160, created: stamp(-3), expires: stamp(3) },
    { id: IDs.d, kind: 'TEXT', privacy: 'STANDARD', format: 'MARKDOWN', text: { zh: '# 已到期的示例', en: '# Expired sample' }, bytes: 21, created: stamp(-10), expires: stamp(-1) },
    { id: IDs.e, kind: 'FILE', privacy: 'STANDARD', filename: 'archive.txt', mediaType: 'text/plain', sha256: 'b'.repeat(64), bytes: 8_192, created: stamp(-11), expires: stamp(-2) },
    { id: IDs.f, kind: 'TEXT', privacy: 'STANDARD', format: 'SOURCE', text: { zh: 'const value = 1;', en: 'const value = 1;' }, bytes: 16, created: stamp(-4), expires: stamp(4) },
  ];

  const params = new URLSearchParams(location.search);
  const initial = params.get('state') || 'login';
  const state = {
    lang: params.get('lang') === 'en' ? 'en' : 'zh',
    unavailable: initial === 'unavailable',
    authenticated: !['login', 'login-error', 'rate-limited', 'session-expired', 'unavailable'].includes(initial),
    rows: sampleShares.slice(), kind: '', privacy: '', lifecycle: '', sort: 'newest',
    exactId: '', selected: new Set(), visibleCount: 3,
    listStatus: initial === 'loading' ? 'loading' : initial === 'error' ? 'error' : 'ready',
    loginError: initial === 'login-error' ? 'authenticationFailed' : initial === 'rate-limited' ? 'rateLimited' : initial === 'session-expired' ? 'sessionExpired' : '',
    lookupError: '', notice: null, modal: null, busy: false,
    detailErrorOnce: initial === 'detail-error',
  };
  if (initial === 'empty') {
    state.exactId = IDs.missing;
    $('exact-id').value = IDs.missing;
  }
  if (initial === 'partial') {
    state.rows = state.rows.filter((row) => row.id !== IDs.a && row.id !== IDs.c);
    state.selected.add(IDs.b);
    state.notice = { key: 'partialCount', args: [2, 1] };
  }
  if (params.get('theme') === 'dark' || params.get('theme') === 'light') $('theme').value = params.get('theme');

  const tr = (key, ...args) => {
    const value = words[state.lang][key];
    return typeof value === 'function' ? value(...args) : value || key;
  };
  const statusOf = (row) => Date.parse(row.expires) <= now ? 'expired' : 'active';
  const formatDate = (value) => value ? new Intl.DateTimeFormat(state.lang === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : tr('never');
  const formatBytes = (value) => value < 1024 ? `${value} B` : `${(value / 1024).toFixed(1)} KiB`;
  const category = (value) => tr(value === 'TEXT' ? 'text' : value === 'FILE' ? 'file' : value === 'ENCRYPTED' ? 'encrypted' : 'standard');

  function theme() {
    const value = $('theme').value;
    document.documentElement.dataset.theme = value === 'dark' || value === 'system' && matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  function filteredShares() {
    return state.rows.filter((row) =>
      (!state.kind || row.kind === state.kind) &&
      (!state.privacy || row.privacy === state.privacy) &&
      (!state.lifecycle || statusOf(row) === state.lifecycle) &&
      (!state.exactId || row.id === state.exactId),
    ).sort((a, b) => state.sort === 'newest' ? Date.parse(b.created) - Date.parse(a.created) : Date.parse(a.created) - Date.parse(b.created));
  }

  function renderSummary() {
    const active = state.rows.filter((row) => statusOf(row) === 'active').length;
    $('summary-active').textContent = String(active);
    $('summary-expired').textContent = String(state.rows.length - active);
    $('summary-text').textContent = String(state.rows.filter((row) => row.kind === 'TEXT').length);
    $('summary-file').textContent = String(state.rows.filter((row) => row.kind === 'FILE').length);
    $('summary-encrypted').textContent = String(state.rows.filter((row) => row.privacy === 'ENCRYPTED').length);
    $('summary-file-bytes').textContent = formatBytes(state.rows.filter((row) => row.kind === 'FILE').reduce((sum, row) => sum + row.bytes, 0));
  }

  function tableCell(row, key, value, className = '') {
    const cell = document.createElement('td');
    cell.dataset.label = tr(key);
    cell.className = className;
    cell.textContent = value;
    row.append(cell);
    return cell;
  }

  function renderRows(rows) {
    const body = $('shares-body');
    body.replaceChildren();
    for (const share of rows.slice(0, state.visibleCount)) {
      const row = document.createElement('tr');
      const selectCell = document.createElement('td');
      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.dataset.id = share.id;
      checkbox.setAttribute('aria-label', `${tr('select')} ${share.id}`);
      checkbox.checked = state.selected.has(share.id);
      selectCell.append(checkbox);
      row.append(selectCell);
      tableCell(row, 'id', share.id, 'share-id');
      tableCell(row, 'type', category(share.kind));
      tableCell(row, 'privacy', category(share.privacy));
      tableCell(row, 'created', formatDate(share.created));
      tableCell(row, 'expires', formatDate(share.expires));
      tableCell(row, 'state', tr(statusOf(share)));
      tableCell(row, 'size', formatBytes(share.bytes));
      const actionCell = document.createElement('td');
      const action = document.createElement('button');
      action.type = 'button';
      action.className = 'row-action';
      action.dataset.inspect = share.id;
      action.textContent = tr('inspect');
      actionCell.append(action);
      row.append(actionCell);
      body.append(row);
    }
  }

  function renderList() {
    const rows = filteredShares();
    $('result-count').textContent = state.listStatus === 'ready' ? tr('results', rows.length) : '';
    $('selection-count').textContent = state.selected.size ? tr('selectedCount', state.selected.size) : '';
    $('list-bar').hidden = state.selected.size === 0;
    $('bulk-delete').hidden = state.selected.size === 0;
    $('bulk-delete').textContent = `${tr('deleteSelected')} (${state.selected.size})`;
    $('list-loading').hidden = state.listStatus !== 'loading';
    $('list-error').hidden = state.listStatus !== 'error';
    $('list-empty').hidden = state.listStatus !== 'ready' || rows.length !== 0;
    $('table-wrap').hidden = state.listStatus !== 'ready' || rows.length === 0;
    $('load-more').hidden = state.listStatus !== 'ready' || rows.length <= state.visibleCount;
    $('clear-lookup').hidden = !$('exact-id').value && !state.exactId;
    $('lookup-error').hidden = !state.lookupError;
    $('lookup-error').textContent = state.lookupError ? tr(state.lookupError) : '';
    if (state.listStatus === 'ready') renderRows(rows);
  }

  function detailEntry(list, label, value) {
    const dt = document.createElement('dt');
    const dd = document.createElement('dd');
    dt.textContent = tr(label);
    dd.textContent = value;
    list.append(dt, dd);
  }

  function makeDetail(share) {
    const fragment = document.createDocumentFragment();
    const list = document.createElement('dl');
    list.className = 'detail-list';
    detailEntry(list, 'id', share.id);
    detailEntry(list, 'type', category(share.kind));
    detailEntry(list, 'privacy', category(share.privacy));
    detailEntry(list, 'created', formatDate(share.created));
    detailEntry(list, 'expires', formatDate(share.expires));
    detailEntry(list, 'state', tr(statusOf(share)));
    detailEntry(list, 'size', formatBytes(share.bytes));
    fragment.append(list);
    const section = document.createElement('section');
    section.className = 'detail-section';
    const heading = document.createElement('h3');
    heading.textContent = tr(share.kind === 'FILE' ? 'fileMetadata' : share.privacy === 'ENCRYPTED' ? 'encryptedText' : 'standardText');
    section.append(heading);
    if (share.kind === 'FILE') {
      const fileList = document.createElement('dl');
      fileList.className = 'detail-list';
      detailEntry(fileList, 'filename', share.filename);
      detailEntry(fileList, 'mediaType', share.mediaType);
      detailEntry(fileList, 'sha256', share.sha256);
      section.append(fileList);
      const link = document.createElement('a');
      link.href = `https://files.example.invalid/f/${share.id}`;
      link.target = '_blank';
      link.rel = 'noopener noreferrer';
      link.textContent = tr('download');
      section.append(link);
    } else if (share.privacy === 'ENCRYPTED') {
      const note = document.createElement('p');
      note.textContent = `${tr('plaintextUnavailable')} ${tr('size')}: ${formatBytes(share.bytes)}`;
      section.append(note);
    } else {
      const pre = document.createElement('pre');
      pre.tabIndex = 0;
      pre.textContent = share.text.zh;
      section.append(pre);
    }
    fragment.append(section);
    return fragment;
  }

  function renderDialog() {
    const modal = state.modal;
    $('dialog-backdrop').hidden = !modal;
    $('main').inert = !!modal;
    $('site-header').inert = !!modal;
    document.body.style.overflow = modal ? 'hidden' : '';
    if (!modal) return;
    const body = $('dialog-body');
    body.replaceChildren();
    const cancel = $('dialog-cancel');
    const confirm = $('dialog-confirm');
    $('dialog-error').hidden = true;
    cancel.disabled = state.busy;
    confirm.disabled = state.busy;
    confirm.hidden = false;
    confirm.className = 'primary-button';
    if (modal.kind === 'detail-loading' || modal.kind === 'detail-error' || modal.kind === 'detail') {
      $('dialog-title').textContent = tr('detail');
      cancel.textContent = tr('close');
      confirm.hidden = modal.kind !== 'detail';
      confirm.textContent = tr('deleteShare');
      if (modal.kind === 'detail') {
        const share = state.rows.find((row) => row.id === modal.id);
        if (share) body.append(makeDetail(share));
      } else {
        const p = document.createElement('p');
        p.textContent = tr(modal.kind === 'detail-error' ? 'detailError' : 'loadingDetail');
        p.setAttribute('role', modal.kind === 'detail-error' ? 'alert' : 'status');
        body.append(p);
      }
    } else {
      $('dialog-title').textContent = tr(modal.kind === 'delete' ? 'deleteTitle' : modal.kind === 'bulk' ? 'bulkTitle' : 'cleanupTitle');
      cancel.textContent = tr('cancel');
      confirm.textContent = tr(modal.kind === 'cleanup' ? 'confirmCleanup' : 'deletePermanently');
      if (modal.kind !== 'cleanup') confirm.className = 'primary-button danger-confirm';
      const p = document.createElement('p');
      p.textContent = tr(modal.kind === 'delete' ? 'deleteQuestion' : modal.kind === 'bulk' ? 'bulkQuestion' : 'cleanupQuestion');
      body.append(p);
    }
  }

  function render() {
    document.documentElement.lang = state.lang === 'zh' ? 'zh-CN' : 'en';
    document.title = `${tr('heading')} · uPaste`;
    document.querySelectorAll('[data-i18n]').forEach((node) => { node.textContent = tr(node.dataset.i18n); });
    $('language-zh').setAttribute('aria-pressed', String(state.lang === 'zh'));
    $('language-en').setAttribute('aria-pressed', String(state.lang === 'en'));
    $('theme').setAttribute('aria-label', tr('theme'));
    document.querySelector('.filter-grid').setAttribute('aria-label', tr('shares'));
    $('exact-id').placeholder = state.lang === 'zh' ? '22 字符' : '22 characters';
    $('unavailable-screen').hidden = !state.unavailable;
    $('login-screen').hidden = state.unavailable || state.authenticated;
    $('dashboard-screen').hidden = state.unavailable || !state.authenticated;
    $('login-error').hidden = !state.loginError;
    $('login-error').textContent = state.loginError ? tr(state.loginError) : '';
    $('sign-in').disabled = !$('admin-token').value.trim();
    $('notice').hidden = !state.notice;
    $('notice').textContent = state.notice ? tr(state.notice.key, ...state.notice.args) : '';
    theme();
    if (state.authenticated) { renderSummary(); renderList(); }
    renderDialog();
  }

  let restoreFocus = null;
  function openModal(modal) {
    if (!state.modal) restoreFocus = document.activeElement;
    state.modal = modal;
    renderDialog();
    $('dialog-cancel').focus();
  }
  function closeModal() {
    state.modal = null;
    state.busy = false;
    renderDialog();
    if (restoreFocus?.isConnected) restoreFocus.focus();
    else $('page-heading').focus();
    restoreFocus = null;
  }
  function openDetail(id) {
    openModal({ kind: 'detail-loading', id });
    setTimeout(() => {
      if (state.modal?.kind !== 'detail-loading' || state.modal.id !== id) return;
      state.modal = { kind: state.detailErrorOnce ? 'detail-error' : 'detail', id };
      state.detailErrorOnce = false;
      renderDialog();
    }, 320);
  }
  function cancelModal() {
    if (state.busy) return;
    if (state.modal?.kind === 'delete' && state.modal.fromDetail) {
      state.modal = { kind: 'detail', id: state.modal.id };
      renderDialog();
      $('dialog-cancel').focus();
    } else closeModal();
  }
  function confirmModal() {
    const modal = state.modal;
    if (!modal || state.busy) return;
    if (modal.kind === 'detail') {
      state.modal = { kind: 'delete', id: modal.id, fromDetail: true };
      renderDialog();
      $('dialog-cancel').focus();
      return;
    }
    state.busy = true;
    renderDialog();
    setTimeout(() => {
      if (modal.kind === 'delete') {
        state.rows = state.rows.filter((row) => row.id !== modal.id);
        state.selected.delete(modal.id);
        state.notice = { key: 'deletedCount', args: [1] };
      } else if (modal.kind === 'bulk') {
        const ids = Array.from(state.selected);
        const failed = ids.filter((id) => id === IDs.b);
        const deleted = ids.filter((id) => !failed.includes(id));
        state.rows = state.rows.filter((row) => !deleted.includes(row.id));
        state.selected = new Set(failed.filter((id) => state.rows.some((row) => row.id === id)));
        state.notice = { key: failed.length ? 'partialCount' : 'deletedCount', args: failed.length ? [deleted.length, failed.length] : [deleted.length] };
      } else if (modal.kind === 'cleanup') {
        const purged = state.rows.filter((row) => statusOf(row) === 'expired').length;
        state.rows = state.rows.filter((row) => statusOf(row) !== 'expired');
        state.selected = new Set(Array.from(state.selected).filter((id) => state.rows.some((row) => row.id === id)));
        state.notice = { key: 'purgedCount', args: [purged] };
      }
      closeModal();
      render();
    }, 380);
  }

  function loadList() {
    state.listStatus = 'loading';
    render();
    setTimeout(() => { state.listStatus = 'ready'; render(); }, 280);
  }
  function clearFilters() {
    state.kind = ''; state.privacy = ''; state.lifecycle = ''; state.sort = 'newest'; state.exactId = '';
    state.lookupError = ''; state.selected.clear(); state.visibleCount = 3;
    $('filter-kind').value = ''; $('filter-privacy').value = ''; $('filter-lifecycle').value = ''; $('filter-sort').value = 'newest'; $('exact-id').value = '';
    loadList();
  }

  $('language-zh').addEventListener('click', () => { state.lang = 'zh'; render(); });
  $('language-en').addEventListener('click', () => { state.lang = 'en'; render(); });
  $('theme').addEventListener('change', theme);
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if ($('theme').value === 'system') theme(); });
  $('admin-token').addEventListener('input', () => { state.loginError = ''; render(); });
  $('login-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const candidate = $('admin-token').value.trim();
    if (!candidate) return;
    if (candidate.toLowerCase() === 'bad' || candidate.toLowerCase() === 'rate') {
      state.loginError = candidate.toLowerCase() === 'bad' ? 'authenticationFailed' : 'rateLimited';
      render();
      return;
    }
    $('admin-token').value = '';
    state.loginError = '';
    state.authenticated = true;
    loadList();
    $('page-heading').focus();
  });
  $('logout').addEventListener('click', () => {
    state.authenticated = false; state.selected.clear(); state.notice = null;
    if (state.modal) closeModal();
    render();
    $('admin-token').focus();
  });
  for (const [control, field] of [['filter-kind', 'kind'], ['filter-privacy', 'privacy'], ['filter-lifecycle', 'lifecycle'], ['filter-sort', 'sort']]) {
    $(control).addEventListener('change', (event) => {
      state[field] = event.target.value;
      state.selected.clear(); state.visibleCount = 3; state.notice = null;
      loadList();
    });
  }
  $('exact-id').addEventListener('input', () => { state.lookupError = ''; renderList(); });
  $('lookup-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const value = $('exact-id').value.trim();
    if (value && !/^[A-Za-z0-9_-]{22}$/.test(value)) {
      state.lookupError = 'invalidId';
      renderList();
      return;
    }
    state.lookupError = ''; state.exactId = value;
    state.selected.clear(); state.visibleCount = 3; state.notice = null;
    loadList();
  });
  $('clear-lookup').addEventListener('click', () => {
    $('exact-id').value = ''; state.exactId = ''; state.lookupError = '';
    state.selected.clear(); state.visibleCount = 3;
    loadList();
  });
  $('clear-filters').addEventListener('click', clearFilters);
  $('retry').addEventListener('click', loadList);
  $('load-more').addEventListener('click', () => { state.visibleCount += 3; renderList(); });
  $('shares-body').addEventListener('change', (event) => {
    const id = event.target.dataset.id;
    if (!id) return;
    if (event.target.checked && state.selected.size < 100) state.selected.add(id);
    else state.selected.delete(id);
    renderList();
  });
  $('shares-body').addEventListener('click', (event) => {
    const id = event.target.dataset.inspect;
    if (id) openDetail(id);
  });
  $('bulk-delete').addEventListener('click', () => { if (state.selected.size) openModal({ kind: 'bulk' }); });
  $('cleanup').addEventListener('click', () => openModal({ kind: 'cleanup' }));
  $('dialog-cancel').addEventListener('click', cancelModal);
  $('dialog-confirm').addEventListener('click', confirmModal);
  $('dialog-backdrop').addEventListener('click', (event) => { if (event.target === $('dialog-backdrop')) cancelModal(); });
  document.addEventListener('keydown', (event) => {
    if (!state.modal) return;
    if (event.key === 'Escape') { event.preventDefault(); cancelModal(); }
    if (event.key !== 'Tab') return;
    const focusable = Array.from(document.querySelectorAll('.dialog a[href], .dialog [tabindex="0"], .dialog button:not([hidden]):not(:disabled)'));
    if (!focusable.length) return;
    if (event.shiftKey && document.activeElement === focusable[0]) { event.preventDefault(); focusable.at(-1).focus(); }
    else if (!event.shiftKey && document.activeElement === focusable.at(-1)) { event.preventDefault(); focusable[0].focus(); }
  });

  render();
  if (initial === 'detail' || initial === 'detail-error') openDetail(IDs.a);
})();
