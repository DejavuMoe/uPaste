const A_COPY = {
  zh: {
    newShare: '新建分享', text: '文本', file: '文件', format: '格式', plain: '纯文本', source: '源码', markdown: 'Markdown',
    privacy: '隐私', standard: '标准', encrypted: '加密', expires: '到期', never: '永久', hour: '1 小时', day: '1 天', week: '7 天', month: '30 天', custom: '自定义…',
    placeholder: '在这里粘贴或输入内容…', create: '创建分享', selectFile: '选择文件', dropFile: '拖入一个文件，或从设备中选择', fileLimit: '仅支持一个文件，最大 64 MiB',
    sizeError: '内容超过 1 MiB。', fileError: '文件必须大于 0 B 且不超过 64 MiB。', emptyError: '请输入内容。', expiryError: '请选择未来的到期时间。',
    encryptedNote: '内容在此浏览器中加密。持有完整链接的人可以阅读；丢失密钥后无法恢复。',
    created: '分享已创建', shareLink: '分享链接', managementToken: '管理令牌', tokenNote: '请立即保存此令牌。它只显示一次，修改或删除分享时必须使用；uPaste 不保存此令牌。',
    copyLink: '复制链接', copyToken: '复制令牌', copied: '已复制', copyFailed: '复制失败，请手动复制', reveal: '显示', hide: '隐藏',
    openShare: '查看分享', manageShare: '管理分享', newAgain: '新建分享', download: '下载文件', save: '保存修改', saved: '修改已保存', deleteShare: '删除分享', deleted: '分享已删除',
    discardTitle: '有未保存的更改', discardQuestion: '确定要放弃草稿吗？', keepEditing: '继续编辑', discard: '放弃并离开',
    deleteTitle: '删除这个分享？', deleteQuestion: '分享及其内容将被永久删除。', deletePermanent: '永久删除', cancel: '取消',
    content: '内容', noKey: '缺少解密密钥', noKeyHelp: '打开完整分享链接以读取加密内容。', back: '返回', remove: '移除',
    loading: '正在创建…', fileReady: '文件已选择', raw: '原文', rendered: '预览',
  },
  en: {
    newShare: 'New share', text: 'Text', file: 'File', format: 'Format', plain: 'Plain text', source: 'Source', markdown: 'Markdown',
    privacy: 'Privacy', standard: 'Standard', encrypted: 'Encrypted', expires: 'Expires', never: 'Never', hour: '1 hour', day: '1 day', week: '7 days', month: '30 days', custom: 'Custom…',
    placeholder: 'Paste or type content here…', create: 'Create share', selectFile: 'Choose file', dropFile: 'Drop one file here or choose from your device', fileLimit: 'One file, up to 64 MiB',
    sizeError: 'Content exceeds 1 MiB.', fileError: 'File must be larger than 0 B and no more than 64 MiB.', emptyError: 'Enter content.', expiryError: 'Choose a future expiration time.',
    encryptedNote: 'Encrypted in this browser. Anyone with the complete link can read it; a lost key cannot be recovered.',
    created: 'Share created', shareLink: 'Share link', managementToken: 'Management token', tokenNote: 'Save this token now. It is shown once and is required to modify or delete this share.',
    copyLink: 'Copy link', copyToken: 'Copy token', copied: 'Copied', copyFailed: 'Copy failed; copy manually', reveal: 'Reveal', hide: 'Hide',
    openShare: 'Open share', manageShare: 'Manage share', newAgain: 'New share', download: 'Download file', save: 'Save changes', saved: 'Changes saved', deleteShare: 'Delete share', deleted: 'Share deleted',
    discardTitle: 'You have unsaved changes.', discardQuestion: 'Are you sure you want to discard your draft?', keepEditing: 'Keep editing', discard: 'Discard and leave',
    deleteTitle: 'Delete this share?', deleteQuestion: 'This permanently deletes the share and its content.', deletePermanent: 'Delete permanently', cancel: 'Cancel',
    content: 'Content', noKey: 'Decryption key missing', noKeyHelp: 'Open the complete share link to read encrypted content.', back: 'Back', remove: 'Remove',
    loading: 'Creating…', fileReady: 'File selected', raw: 'Raw', rendered: 'Rendered',
  },
};

function AApp() {
  const [lang, setLang] = React.useState('zh');
  const [theme, setTheme] = React.useState('system');
  const [screen, setScreen] = React.useState('create');
  const [kind, setKind] = React.useState('text');
  const [content, setContent] = React.useState('');
  const [file, setFile] = React.useState(null);
  const [format, setFormat] = React.useState('plain');
  const [privacy, setPrivacy] = React.useState('standard');
  const [expires, setExpires] = React.useState('never');
  const [customExpiry, setCustomExpiry] = React.useState('');
  const [error, setError] = React.useState('');
  const [dialog, setDialog] = React.useState('');
  const [pendingScreen, setPendingScreen] = React.useState('create');
  const [tokenVisible, setTokenVisible] = React.useState(false);
  const [copyState, setCopyState] = React.useState('');
  const [savedContent, setSavedContent] = React.useState('');
  const [savedFile, setSavedFile] = React.useState(null);
  const [savedKind, setSavedKind] = React.useState('text');
  const [savedPrivacy, setSavedPrivacy] = React.useState('standard');
  const [savedFormat, setSavedFormat] = React.useState('plain');
  const [manageText, setManageText] = React.useState('');
  const [status, setStatus] = React.useState('');
  const [dragging, setDragging] = React.useState(false);
  const fileInputRef = React.useRef(null);
  const t = A_COPY[lang];
  const maxText = 1_048_576;
  const maxFile = 67_108_864;
  const bytes = new TextEncoder().encode(content).length;
  const dirty = screen === 'create' && (content.length > 0 || !!file);
  const fullLink = `https://paste.example.invalid/s/demo${savedPrivacy === 'encrypted' ? '#up_e1_example_key_not_valid' : ''}`;
  const token = 'up_o1_example_token_not_valid';

  React.useEffect(() => {
    document.documentElement.lang = lang;
    const dark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  }, [lang, theme]);

  React.useEffect(() => {
    document.getElementById('a-main-title')?.focus();
  }, [screen]);

  React.useEffect(() => {
    if (!dirty) return;
    const beforeUnload = event => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', beforeUnload);
    return () => window.removeEventListener('beforeunload', beforeUnload);
  }, [dirty]);

  function go(next) {
    if (dirty && next !== 'create') { setPendingScreen(next); setDialog('discard'); return; }
    setScreen(next);
    setError('');
    setStatus('');
  }

  function reset() {
    setContent(''); setFile(null); setKind('text'); setFormat('plain'); setPrivacy('standard'); setExpires('never'); setCustomExpiry('');
    setTokenVisible(false); setScreen('create'); setError(''); setStatus('');
  }

  function create() {
    setError('');
    if (kind === 'text' && bytes === 0) { setError(t.emptyError); return; }
    if (kind === 'text' && bytes > maxText) { setError(t.sizeError); return; }
    if (kind === 'file' && (!file || file.size === 0 || file.size > maxFile)) { setError(t.fileError); return; }
    if (expires === 'custom' && (!customExpiry || new Date(customExpiry).getTime() <= Date.now())) { setError(t.expiryError); return; }
    setSavedContent(content); setSavedFile(file); setSavedKind(kind); setSavedFormat(format); setSavedPrivacy(kind === 'file' ? 'standard' : privacy);
    setManageText(content); setTokenVisible(false); setScreen('result');
  }

  async function copy(value, key) {
    try { await navigator.clipboard.writeText(value); setCopyState(key); }
    catch { setCopyState('failed'); }
    window.setTimeout(() => setCopyState(''), 1500);
  }

  const field = (label, value, onChange, options) => (
    <div className="a-field">
      <label>{label}</label>
      <select aria-label={label} value={value} onChange={event => onChange(event.target.value)}>
        {options.map(([key, name]) => <option key={key} value={key}>{name}</option>)}
      </select>
    </div>
  );

  return <div className="a-shell">
    <aside className="a-rail" aria-label="uPaste"><button className="a-brand" onClick={() => go('create')}>uPaste</button><div className="a-rail-mark" aria-hidden="true"></div></aside>
    <div className="a-main">
      <header className="a-top">
        <button className="a-mobile-brand" onClick={() => go('create')}>uPaste</button>
        <div className="a-locale" role="group" aria-label="Language"><button aria-pressed={lang === 'zh'} onClick={() => setLang('zh')}>中文</button><span aria-hidden="true">/</span><button aria-pressed={lang === 'en'} onClick={() => setLang('en')}>EN</button></div>
        <select className="a-theme" aria-label={lang === 'zh' ? '外观' : 'Theme'} value={theme} onChange={event => setTheme(event.target.value)}><option value="system">{lang === 'zh' ? '跟随系统' : 'System'}</option><option value="light">{lang === 'zh' ? '浅色' : 'Light'}</option><option value="dark">{lang === 'zh' ? '深色' : 'Dark'}</option></select>
      </header>
      <main className="a-workspace">
        {screen === 'create' && <>
          <h1 id="a-main-title" tabIndex="-1" className="a-title">{t.newShare}</h1>
          <div className="a-editor-shell">
            <div className="a-tabs" role="tablist" aria-label={t.content}>
              <button role="tab" aria-selected={kind === 'text'} onClick={() => { setKind('text'); setError(''); }}>{t.text}</button>
              <button role="tab" aria-selected={kind === 'file'} onClick={() => { setKind('file'); setError(''); }}>{t.file}</button>
            </div>
            {kind === 'text' ? <div className="a-editor-wrap"><textarea className="a-editor" aria-label={t.content} placeholder={t.placeholder} value={content} onChange={event => setContent(event.target.value)}></textarea><span className={`a-count ${bytes > maxText ? 'a-over' : ''}`}>{bytes.toLocaleString()} B / 1 MiB</span></div> : <div className="a-file-zone" data-dragging={dragging} onDragOver={event => { event.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={event => { event.preventDefault(); setDragging(false); setFile(event.dataTransfer.files?.[0] || null); }}><strong>{file ? file.name : t.dropFile}</strong><input className="a-file-input" ref={fileInputRef} aria-label={t.selectFile} type="file" onChange={event => setFile(event.target.files?.[0] || null)}></input><button type="button" className="a-secondary" onClick={() => fileInputRef.current?.click()}>{t.selectFile}</button><small>{file ? `${(file.size / 1024).toFixed(1)} KiB` : t.fileLimit}</small>{file && <button className="a-secondary" onClick={() => setFile(null)}>{t.remove}</button>}</div>}
            <div className={`a-options ${kind === 'file' ? 'a-file-options' : ''}`}>
              {kind === 'text' && field(t.format, format, setFormat, [['plain', t.plain], ['source', t.source], ['markdown', t.markdown]])}
              {kind === 'text' && field(t.privacy, privacy, setPrivacy, [['standard', t.standard], ['encrypted', t.encrypted]])}
              {field(t.expires, expires, setExpires, [['never', t.never], ['hour', t.hour], ['day', t.day], ['week', t.week], ['month', t.month], ['custom', t.custom]])}
              <div className="a-submit-wrap"><button className="a-primary" disabled={(kind === 'text' && (bytes === 0 || bytes > maxText)) || (kind === 'file' && (!file || file.size === 0 || file.size > maxFile))} onClick={create}>{t.create}</button></div>
            </div>
          </div>
          {expires === 'custom' && <div className="a-field"><label htmlFor="a-custom-expiry">{t.expires}</label><input id="a-custom-expiry" type="datetime-local" value={customExpiry} onChange={event => setCustomExpiry(event.target.value)}></input></div>}
          {kind === 'text' && privacy === 'encrypted' && <p className="a-info">{t.encryptedNote}</p>}
          {error && <p className="a-error" role="alert">{error}</p>}
        </>}
        {screen === 'result' && <>
          <h1 id="a-main-title" tabIndex="-1" className="a-title">{t.created}</h1>
          <section className="a-panel">
            <div className="a-result-row"><label htmlFor="a-share-link">{t.shareLink}</label><div className="a-inline"><input id="a-share-link" className="a-readonly" readOnly value={fullLink} onFocus={event => event.target.select()}></input><button className="a-secondary" onClick={() => copy(fullLink, 'link')}>{copyState === 'link' ? t.copied : t.copyLink}</button></div></div>
            <div className="a-result-row"><label htmlFor="a-owner-token">{t.managementToken}</label><div className="a-inline"><input id="a-owner-token" className="a-readonly" readOnly value={tokenVisible ? token : 'up_o1_••••••••••••••••••••••••••••••••'} onFocus={event => event.target.select()}></input><button className="a-secondary" onClick={() => setTokenVisible(!tokenVisible)}>{tokenVisible ? t.hide : t.reveal}</button><button className="a-secondary" onClick={() => copy(token, 'token')}>{copyState === 'token' ? t.copied : t.copyToken}</button></div><p className="a-token-note">{t.tokenNote}</p></div>
            {copyState === 'failed' && <p className="a-error" role="alert">{t.copyFailed}</p>}
            <div className="a-actions"><button className="a-primary" onClick={() => go('view')}>{t.openShare}</button><button className="a-secondary" onClick={() => go('manage')}>{t.manageShare}</button><button className="a-secondary" onClick={reset}>{t.newAgain}</button></div>
          </section>
        </>}
        {screen === 'view' && <>
          <h1 id="a-main-title" tabIndex="-1" className="a-title">{savedKind === 'file' ? savedFile?.name : t.content}</h1>
          <div className="a-view-meta">{savedKind === 'file' ? `${(savedFile.size / 1024).toFixed(1)} KiB` : t[savedFormat]} · {t.expires}: {t[expires]}</div>
          {savedKind === 'file' ? <div className="a-panel"><button className="a-primary" onClick={() => { const url = URL.createObjectURL(savedFile); const link = document.createElement('a'); link.href = url; link.download = savedFile.name; link.click(); window.setTimeout(() => URL.revokeObjectURL(url), 1000); }}>{t.download}</button></div> : <div className="a-view-content">{savedContent}</div>}
          <div className="a-actions"><button className="a-secondary" onClick={() => go('manage')}>{t.manageShare}</button><button className="a-secondary" onClick={reset}>{t.newAgain}</button></div>
        </>}
        {screen === 'manage' && <>
          <h1 id="a-main-title" tabIndex="-1" className="a-title">{t.manageShare}</h1>
          <section className="a-panel">
            {savedKind === 'text' && <><label htmlFor="a-manage-content">{t.content}</label><textarea id="a-manage-content" className="a-manage-text" value={manageText} onChange={event => setManageText(event.target.value)}></textarea></>}
            {savedKind === 'file' && <p>{savedFile?.name}</p>}
            <div className="a-actions"><button className="a-primary" onClick={() => { setSavedContent(manageText); setStatus(t.saved); }}>{t.save}</button><button className="a-secondary" onClick={() => go('view')}>{t.cancel}</button><button className="a-secondary a-danger" onClick={() => setDialog('delete')}>{t.deleteShare}</button></div>
            {status && <p role="status">{status}</p>}
          </section>
        </>}
        {screen === 'deleted' && <section className="a-panel"><h1 id="a-main-title" tabIndex="-1">{t.deleted}</h1><button className="a-primary" onClick={reset}>{t.newAgain}</button></section>}
      </main>
    </div>
    {dialog && <div className="a-backdrop"><div className="a-dialog" role="dialog" aria-modal="true" aria-labelledby="a-dialog-title"><h2 id="a-dialog-title">{dialog === 'delete' ? t.deleteTitle : t.discardTitle}</h2><p>{dialog === 'delete' ? t.deleteQuestion : t.discardQuestion}</p><div className="a-actions"><button className="a-secondary" autoFocus onClick={() => setDialog('')}>{dialog === 'delete' ? t.cancel : t.keepEditing}</button><button className="a-secondary a-danger" onClick={() => { if (dialog === 'delete') setScreen('deleted'); else setScreen(pendingScreen); setDialog(''); }}>{dialog === 'delete' ? t.deletePermanent : t.discard}</button></div></div></div>}
  </div>;
}

ReactDOM.createRoot(document.getElementById('root')).render(<AApp />);
