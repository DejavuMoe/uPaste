(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const params = new URLSearchParams(location.search);
  const mode = params.get('demo') === 'private' ? 'private' : 'public';
  const limitText = 1_048_576;
  const limitFile = 67_108_864;
  const sampleLink = 'https://paste.example.invalid/s/sample';
  const sampleToken = 'sample-owner-token.invalid';
  const presets = [
    { value: 'never', key: 'never' },
    { value: '1h', key: 'oneHour', seconds: 3600 },
    { value: '1d', key: 'oneDay', seconds: 86400 },
    { value: '7d', key: 'sevenDays', seconds: 604800 },
    { value: '30d', key: 'thirtyDays', seconds: 2592000 },
    { value: 'custom', key: 'custom' },
  ];
  const configFixture = mode === 'public'
    ? { deployment_mode: 'public', retention: { default_seconds: 86400, max_seconds: 604800 }, challenge: { provider: 'cap' } }
    : { deployment_mode: 'private' };
  const words = {
    zh: {
      newShare: '新建分享', openShare: '查看分享', manageShare: '管理分享', text: '文本', file: '文件', content: '内容', format: '格式', plain: '纯文本', source: '源码', markdown: 'Markdown',
      privacy: '隐私', standard: '标准', encrypted: '加密', expires: '到期', never: '永久', oneHour: '1 小时', oneDay: '1 天', sevenDays: '7 天', thirtyDays: '30 天', custom: '自定义…', customDate: '自定义到期时间',
      theme: '外观', system: '跟随系统', light: '浅色', dark: '深色', textPlaceholder: '粘贴或输入内容…', dropFile: '拖入一个文件', chooseFile: '选择文件', fileLimit: '单个文件，最大 64 MiB', remove: '移除',
      create: '创建分享', creating: '正在创建…', uploading: '正在上传…', created: '分享已创建', shareLink: '分享链接', managementToken: '管理令牌', tokenNote: '请立即保存。令牌只显示一次；修改或删除分享时需要使用，uPaste 无法恢复它。', copyLink: '复制链接', copyToken: '复制令牌', copied: '已复制', copyManually: '复制失败，请手动复制', reveal: '显示', hide: '隐藏',
      encryptedNote: '内容在此浏览器中加密。持有完整链接的人可以阅读；服务器无法恢复丢失的解密密钥。', emptyText: '请输入内容。', largeText: '内容超过 1 MiB。', missingFile: '请选择一个文件。', emptyFile: '文件不能为空。', largeFile: '文件超过 64 MiB。', manyFiles: '仅支持一个文件，已选择第一个。',
      loadingSettings: '正在加载…', loadingConfig: '正在读取分享设置…', configError: '无法读取分享设置。', tryAgain: '重试', selectCustomDate: '请选择自定义到期时间。', futureDate: '到期时间必须晚于现在。', maxDate: (span) => `到期时间必须在创建后 ${span}内。`, publicLimit: (span) => `公开分享需在创建后 ${span}内到期。`,
      humanVerification: '人机验证', verify: '完成验证', challengeLoading: '正在加载验证…', challengeReady: '需要完成人机验证', challengeVerifying: '正在验证…', challengeVerified: '已验证', challengeExpired: '验证已过期，请重新完成', challengeError: '验证暂时失败，请重试', submitFailed: '创建失败，请重试。',
    },
    en: {
      newShare: 'New share', openShare: 'Open share', manageShare: 'Manage share', text: 'Text', file: 'File', content: 'Content', format: 'Format', plain: 'Plain text', source: 'Source', markdown: 'Markdown',
      privacy: 'Privacy', standard: 'Standard', encrypted: 'Encrypted', expires: 'Expires', never: 'Never', oneHour: '1 hour', oneDay: '1 day', sevenDays: '7 days', thirtyDays: '30 days', custom: 'Custom…', customDate: 'Custom expiration date and time',
      theme: 'Theme', system: 'System', light: 'Light', dark: 'Dark', textPlaceholder: 'Paste or type content…', dropFile: 'Drop one file here', chooseFile: 'Choose file', fileLimit: 'One file, up to 64 MiB', remove: 'Remove',
      create: 'Create share', creating: 'Creating…', uploading: 'Uploading…', created: 'Share created', shareLink: 'Share link', managementToken: 'Management token', tokenNote: 'Save this token now. It is shown once and is required to modify or delete this share. uPaste cannot recover it.', copyLink: 'Copy link', copyToken: 'Copy token', copied: 'Copied', copyManually: 'Copy failed; copy manually', reveal: 'Reveal', hide: 'Hide',
      encryptedNote: 'Encrypted in this browser. Anyone with the complete link can read it. The server cannot recover a lost decryption key.', emptyText: 'Enter content.', largeText: 'Content exceeds 1 MiB.', missingFile: 'Choose a file.', emptyFile: 'File cannot be empty.', largeFile: 'File exceeds 64 MiB.', manyFiles: 'Only one file is supported; the first was selected.',
      loadingSettings: 'Loading…', loadingConfig: 'Loading share settings…', configError: 'Could not load share settings.', tryAgain: 'Try again', selectCustomDate: 'Select a custom expiration date.', futureDate: 'Expiration must be in the future.', maxDate: (span) => `Expiration must be within ${span} of creation.`, publicLimit: (span) => `Public shares must expire within ${span} of creation.`,
      humanVerification: 'Human verification', verify: 'Complete verification', challengeLoading: 'Loading verification…', challengeReady: 'Verification required', challengeVerifying: 'Verifying…', challengeVerified: 'Verified', challengeExpired: 'Verification expired — solve again', challengeError: 'Temporary verification failure — try again', submitFailed: 'Could not create share. Try again.',
    },
  };

  const state = {
    lang: 'zh', kind: 'text', text: '', file: null, format: 'PLAIN',
    config: null, configStatus: 'loading', challengeStatus: 'loading', challengeVersion: 0,
    submitting: false, submitAttempts: 0, error: '', result: null, tokenVisible: false, copied: '', copyStatus: '',
  };
  let configAttempts = 0;
  const tr = (key) => words[state.lang][key] || key;
  const byteLength = (value) => new TextEncoder().encode(value).byteLength;
  const formatBytes = (n) => n < 1024 ? `${n} B` : n < 1_048_576 ? `${(n / 1024).toFixed(1)} KiB` : `${(n / 1_048_576).toFixed(1)} MiB`;
  const privacy = () => document.querySelector('input[name="privacy"]:checked').value;
  const isPublic = () => state.config?.deployment_mode === 'public';
  const spanLabel = (seconds) => {
    const [count, zhUnit, enUnit] = seconds % 86400 === 0 ? [seconds / 86400, '天', 'day']
      : seconds % 3600 === 0 ? [seconds / 3600, '小时', 'hour']
      : seconds % 60 === 0 ? [seconds / 60, '分钟', 'minute'] : [seconds, '秒', 'second'];
    return `${count} ${state.lang === 'zh' ? zhUnit : `${enUnit}${count === 1 ? '' : 's'}`}`;
  };

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
    const height = editor.scrollHeight + 8;
    editor.style.height = `${Math.min(maximum, Math.max(minimum, height))}px`;
    editor.style.overflowY = height > maximum ? 'auto' : 'hidden';
  }

  function fillExpirationOptions() {
    const select = $('expiry');
    select.replaceChildren();
    const publicMode = isPublic();
    for (const preset of presets) {
      if (publicMode && (preset.value === 'never' || preset.value === '30d' || preset.seconds > state.config.retention.max_seconds)) continue;
      const option = document.createElement('option');
      option.value = preset.value;
      option.dataset.i18n = preset.key;
      option.textContent = tr(preset.key);
      select.append(option);
    }
    const defaultSeconds = state.config.retention?.default_seconds;
    const defaultPreset = publicMode ? presets.find((preset) => preset.seconds === defaultSeconds && select.querySelector(`option[value="${preset.value}"]`)) : null;
    select.value = publicMode ? defaultPreset?.value ?? 'custom' : 'never';
    if (publicMode && !defaultPreset) {
      const target = new Date(Date.now() + defaultSeconds * 1000);
      target.setMinutes(target.getMinutes() - target.getTimezoneOffset());
      $('custom-expiry').value = target.toISOString().slice(0, 16);
    }
  }

  function expirationError() {
    if (state.configStatus !== 'ready') return 'loadingSettings';
    if ($('expiry').value !== 'custom') return '';
    const raw = $('custom-expiry').value;
    if (!raw) return 'selectCustomDate';
    const target = new Date(raw).getTime();
    if (!Number.isFinite(target) || target <= Date.now()) return 'futureDate';
    if (isPublic() && target > Date.now() + state.config.retention.max_seconds * 1000) return 'maxDate';
    return '';
  }

  function expirationAt(createdAt) {
    const preset = presets.find((item) => item.value === $('expiry').value);
    if (preset.value === 'never') return null;
    if (preset.value === 'custom') return new Date($('custom-expiry').value).toISOString();
    return new Date(createdAt + preset.seconds * 1000).toISOString();
  }

  function validContent() {
    return state.kind === 'text'
      ? byteLength(state.text) > 0 && byteLength(state.text) <= limitText
      : !!state.file && state.file.size > 0 && state.file.size <= limitFile;
  }

  function render() {
    document.querySelectorAll('[data-i18n]').forEach((node) => { node.textContent = tr(node.dataset.i18n); });
    document.documentElement.lang = state.lang === 'zh' ? 'zh-CN' : 'en';
    $('language-zh').setAttribute('aria-pressed', String(state.lang === 'zh'));
    $('language-en').setAttribute('aria-pressed', String(state.lang === 'en'));
    $('theme').setAttribute('aria-label', tr('theme'));
    $('file-input').setAttribute('aria-label', tr('chooseFile'));
    document.querySelector('.mode-tabs').setAttribute('aria-label', state.lang === 'zh' ? '内容类型' : 'Content type');
    $('text-content').placeholder = tr('textPlaceholder');
    setTheme();

    $('create-screen').hidden = !!state.result;
    $('result-screen').hidden = !state.result;
    $('page-heading').textContent = tr(state.result ? 'created' : 'newShare');
    document.title = `${tr(state.result ? 'created' : 'newShare')} · uPaste`;
    $('config-banner').hidden = state.configStatus === 'ready' || !!state.result;
    $('config-banner').dataset.state = state.configStatus;
    $('config-status').textContent = tr(state.configStatus === 'error' ? 'configError' : 'loadingConfig');
    $('config-retry').hidden = state.configStatus !== 'error';

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
    if (state.file) {
      $('file-name').textContent = state.file.name;
      $('file-size').textContent = formatBytes(state.file.size);
    }
    $('content-size').textContent = state.kind === 'text'
      ? `${formatBytes(byteLength(state.text))} / 1 MiB`
      : `${formatBytes(state.file?.size ?? 0)} / 64 MiB`;
    $('text-content').dataset.format = state.format;
    $('text-content').disabled = state.submitting;
    $('file-input').disabled = state.submitting;
    $('choose-file').disabled = state.submitting;
    $('remove-file').disabled = state.submitting;
    $('format').disabled = state.submitting;
    document.querySelectorAll('input[name="privacy"]').forEach((input) => { input.disabled = state.submitting; });
    fitEditor();

    $('expiry').disabled = state.configStatus !== 'ready' || state.submitting;
    $('custom-expiry-wrap').hidden = state.configStatus !== 'ready' || $('expiry').value !== 'custom';
    $('custom-expiry').disabled = state.submitting;
    $('expiry-help').hidden = !isPublic() || !!state.result;
    if (isPublic()) $('expiry-help').textContent = tr('publicLimit')(spanLabel(state.config.retention.max_seconds));
    const expError = expirationError();
    $('expiry-error').hidden = !expError || expError === 'loadingSettings' || $('expiry').value !== 'custom';
    $('expiry-error').textContent = expError === 'maxDate'
      ? tr('maxDate')(spanLabel(state.config.retention.max_seconds)) : tr(expError);
    $('custom-expiry').setAttribute('aria-invalid', String(!$('expiry-error').hidden));
    $('encrypted-note').hidden = state.kind !== 'text' || privacy() !== 'ENCRYPTED';

    $('challenge-gate').hidden = !isPublic() || !!state.result;
    $('challenge-gate').dataset.status = state.challengeStatus;
    $('challenge-gate').setAttribute('aria-label', tr('humanVerification'));
    $('challenge-status').textContent = tr(`challenge${state.challengeStatus[0].toUpperCase()}${state.challengeStatus.slice(1)}`);
    $('challenge-mark').textContent = state.challengeStatus === 'verified' ? '✓' : '';
    $('challenge-verify').hidden = state.challengeStatus !== 'ready';
    $('challenge-retry').hidden = !['expired', 'error'].includes(state.challengeStatus);
    $('create').disabled = state.configStatus !== 'ready' || !validContent() || !!expError || state.submitting || isPublic() && state.challengeStatus !== 'verified';
    $('create').textContent = tr(state.submitting ? state.kind === 'file' ? 'uploading' : 'creating' : 'create');
    $('create-error').hidden = !state.error;
    $('create-error').textContent = state.error === 'maxDate'
      ? tr('maxDate')(spanLabel(state.config.retention.max_seconds)) : state.error ? tr(state.error) : '';

    if (state.result) {
      const result = state.result;
      const format = result.kind === 'file' ? tr('file') : tr(result.format === 'PLAIN' ? 'plain' : result.format === 'SOURCE' ? 'source' : 'markdown');
      const privacyLabel = tr(result.privacy === 'ENCRYPTED' ? 'encrypted' : 'standard');
      const expiryLabel = result.expiry === 'custom' ? tr('custom') : tr(presets.find((item) => item.value === result.expiry).key);
      $('result-meta').textContent = `${format} · ${privacyLabel} · ${expiryLabel}`;
      const previewKind = result.kind === 'file' ? 'file' : result.privacy === 'ENCRYPTED' ? result.format === 'MARKDOWN' ? 'encrypted-markdown' : result.format === 'SOURCE' ? 'encrypted-source' : 'encrypted' : result.format.toLowerCase();
      $('open-share').href = `prototype-v3.html?screen=read&kind=${previewKind}`;
      $('manage-share').href = `prototype-v3.html?screen=manage&kind=${previewKind}&token=present${isPublic() ? '&public=1' : ''}`;
      $('share-link').value = result.privacy === 'ENCRYPTED' ? `${sampleLink}#up_e1_invalid-demo-key` : sampleLink;
      $('owner-token').value = state.tokenVisible ? sampleToken : '••••••••••••••••••••';
      $('copy-link').textContent = tr(state.copied === 'link' ? 'copied' : 'copyLink');
      $('copy-token').textContent = tr(state.copied === 'token' ? 'copied' : 'copyToken');
      $('toggle-token').textContent = tr(state.tokenVisible ? 'hide' : 'reveal');
      $('copy-status').textContent = state.copyStatus ? tr(state.copyStatus) : '';
    }
  }

  function loadConfig() {
    state.configStatus = 'loading';
    state.config = null;
    render();
    const attempt = ++configAttempts;
    setTimeout(() => {
      if (attempt === 1 && params.get('config') === 'error') {
        state.configStatus = 'error';
      } else {
        state.config = configFixture;
        state.configStatus = 'ready';
        fillExpirationOptions();
        if (isPublic()) resetChallenge();
      }
      render();
    }, 650);
  }

  function resetChallenge() {
    const version = ++state.challengeVersion;
    state.challengeStatus = 'loading';
    render();
    if (params.get('challenge') === 'loading' && version === 1) return;
    setTimeout(() => {
      if (state.challengeVersion !== version) return;
      state.challengeStatus = version === 1 && ['error', 'expired'].includes(params.get('challenge'))
        ? params.get('challenge') : 'ready';
      render();
    }, 500);
  }

  function verifyChallenge() {
    if (state.challengeStatus !== 'ready') return;
    const version = state.challengeVersion;
    state.challengeStatus = 'verifying';
    render();
    setTimeout(() => {
      if (state.challengeVersion !== version) return;
      state.challengeStatus = 'verified';
      render();
      setTimeout(() => {
        if (state.challengeVersion !== version || state.challengeStatus !== 'verified') return;
        state.challengeStatus = 'expired';
        render();
      }, 30000);
    }, 650);
  }

  function submit() {
    if ($('create').disabled) return;
    const currentExpirationError = expirationError();
    if (currentExpirationError) { state.error = currentExpirationError; render(); return; }
    const createdAt = Date.now();
    const expiresAt = expirationAt(createdAt);
    if (isPublic() && (!expiresAt || new Date(expiresAt).getTime() > createdAt + state.config.retention.max_seconds * 1000)) {
      $('expiry').value = 'custom';
      state.error = 'maxDate';
      render();
      return;
    }
    state.submitting = true;
    state.error = '';
    state.submitAttempts += 1;
    render();
    setTimeout(() => {
      const fail = params.get('submit') === 'error' || params.get('submit') === 'once-error' && state.submitAttempts === 1;
      if (fail) state.error = 'submitFailed';
      else state.result = { kind: state.kind, format: state.format, privacy: state.kind === 'file' ? 'STANDARD' : privacy(), expiry: $('expiry').value, expiresAt };
      state.submitting = false;
      if (isPublic()) resetChallenge();
      render();
      if (state.result) $('page-heading').focus();
    }, 750);
  }

  function acceptFile(files) {
    if (!files?.length || state.submitting) return;
    const file = files[0];
    state.file = file.size > 0 && file.size <= limitFile ? file : null;
    state.error = files.length > 1 ? 'manyFiles' : file.size === 0 ? 'emptyFile' : file.size > limitFile ? 'largeFile' : '';
    render();
  }

  async function copy(value, input, which) {
    try {
      await navigator.clipboard.writeText(value);
      state.copied = which;
      state.copyStatus = '';
    } catch {
      state.tokenVisible ||= which === 'token';
      state.copied = '';
      state.copyStatus = 'copyManually';
      render();
      input.focus();
      input.select();
      return;
    }
    render();
    setTimeout(() => { if (state.copied === which) { state.copied = ''; render(); } }, 1500);
  }

  function newShare() {
    state.result = null;
    state.kind = 'text';
    state.text = '';
    state.file = null;
    state.format = 'PLAIN';
    state.error = '';
    state.copied = '';
    state.copyStatus = '';
    state.tokenVisible = false;
    $('text-content').value = '';
    $('file-input').value = '';
    $('format').value = 'PLAIN';
    $('custom-expiry').value = '';
    document.querySelector('input[name="privacy"][value="STANDARD"]').checked = true;
    fillExpirationOptions();
    if (isPublic()) resetChallenge();
    render();
    $('page-heading').focus();
  }

  $('language-zh').addEventListener('click', () => { state.lang = 'zh'; render(); });
  $('language-en').addEventListener('click', () => { state.lang = 'en'; render(); });
  $('theme').addEventListener('change', render);
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if ($('theme').value === 'system') setTheme(); });
  for (const kind of ['text', 'file']) $(`mode-${kind}`).addEventListener('click', () => { state.kind = kind; state.error = ''; render(); });
  document.querySelector('.mode-tabs').addEventListener('keydown', (event) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    state.kind = event.key === 'ArrowRight' || event.key === 'End' ? 'file' : 'text';
    state.error = '';
    render();
    $(`mode-${state.kind}`).focus();
  });
  $('text-content').addEventListener('input', (event) => { state.text = event.target.value; state.error = ''; render(); });
  $('text-content').addEventListener('keydown', (event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); submit(); } });
  $('format').addEventListener('change', (event) => { state.format = event.target.value; render(); });
  document.querySelectorAll('input[name="privacy"]').forEach((input) => input.addEventListener('change', render));
  $('expiry').addEventListener('change', render);
  $('custom-expiry').addEventListener('input', render);
  $('choose-file').addEventListener('click', () => $('file-input').click());
  $('file-input').addEventListener('change', (event) => acceptFile(event.target.files));
  $('remove-file').addEventListener('click', () => { state.file = null; state.error = ''; $('file-input').value = ''; render(); });
  for (const eventName of ['dragenter', 'dragover']) $('file-panel').addEventListener(eventName, (event) => { event.preventDefault(); $('file-panel').classList.add('is-dragging'); });
  for (const eventName of ['dragleave', 'drop']) $('file-panel').addEventListener(eventName, (event) => { event.preventDefault(); $('file-panel').classList.remove('is-dragging'); });
  $('file-panel').addEventListener('drop', (event) => acceptFile(event.dataTransfer.files));
  $('config-retry').addEventListener('click', loadConfig);
  $('challenge-verify').addEventListener('click', verifyChallenge);
  $('challenge-retry').addEventListener('click', resetChallenge);
  $('create').addEventListener('click', submit);
  $('copy-link').addEventListener('click', () => void copy($('share-link').value, $('share-link'), 'link'));
  $('copy-token').addEventListener('click', () => void copy(sampleToken, $('owner-token'), 'token'));
  $('toggle-token').addEventListener('click', () => { state.tokenVisible = !state.tokenVisible; render(); });
  $('new-share').addEventListener('click', newShare);
  window.addEventListener('resize', fitEditor);

  render();
  loadConfig();
})();
