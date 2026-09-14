import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Link, UNSAFE_DataRouterContext, useBlocker, useLocation, useNavigate, useParams } from 'react-router';
import { ApiError, deleteShare, getShare, updateShare } from '../../app/api';
import type { ShareMetadata, TextFormat } from '../../app/types';
import { decryptWithFragment, encryptWithFragment, type EncryptedPayloadV1 } from '../../crypto/encryptedText';
import { useOwnerCapabilities } from '../../app/ownerCapabilities';
import { Button } from '../../components/Button';
import { Dialog } from '../../components/Dialog';
import { useDocumentTitle } from '../../app/useDocumentTitle';
import { formatFileSize } from '../share/viewerHelpers';

const maxBytes = 1_048_576;
const bytes = (value: string) => new TextEncoder().encode(value).byteLength;

function validPayload(share: ShareMetadata) {
  return (share.payload_kind === 'TEXT' && ((share.privacy_mode === 'STANDARD' && share.text) || (share.privacy_mode === 'ENCRYPTED' && share.encrypted_text))) ||
    (share.payload_kind === 'FILE' && share.privacy_mode === 'STANDARD' && share.file);
}

function localDate(value: string) {
  const date = new Date(value); date.setMinutes(date.getMinutes() - date.getTimezoneOffset());
  return date.toISOString().slice(0, 16);
}

function NavigationGuard({ dirty }: { dirty: boolean }) {
  const blocker = useBlocker(dirty);
  return blocker.state === 'blocked' ? <Dialog title="You have unsaved changes." onClose={() => blocker.reset?.()}><p>Are you sure you want to discard your draft?</p><Button variant="secondary" onClick={() => blocker.reset?.()}>Keep editing</Button><Button variant="danger" onClick={() => blocker.proceed?.()}>Discard and leave</Button></Dialog> : null;
}

export const ManageRoute: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation(); const navigate = useNavigate();
  const { get, remember, forget } = useOwnerCapabilities();
  const [share, setShare] = useState<ShareMetadata | null>(null);
  const [terminal, setTerminal] = useState<'loading' | 'not_found' | 'expired' | 'server_error' | 'deleted'>('loading');
  const [tokenInput, setTokenInput] = useState(''); const [reveal, setReveal] = useState(false);
  const [format, setFormat] = useState<TextFormat>('PLAIN'); const [content, setContent] = useState('');
  const [baseline, setBaseline] = useState({ format: 'PLAIN' as TextFormat, content: '' });
  const [expiry, setExpiry] = useState<string | null>(null); const [expiryDirty, setExpiryDirty] = useState(false);
  const [customExpiry, setCustomExpiry] = useState(''); const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState(''); const [deleteOpen, setDeleteOpen] = useState(false);
  const [decryptAvailable, setDecryptAvailable] = useState(false);
  const token = id ? get(id) : undefined;
  const contentDirty = format !== baseline.format || content !== baseline.content;
  const dirty = contentDirty || expiryDirty;
  const dataRouter = useContext(UNSAFE_DataRouterContext);
  useDocumentTitle(['not_found', 'expired', 'server_error', 'deleted'].includes(terminal) ? 'uPaste' : 'Manage share · uPaste');

  useEffect(() => {
    if (!dirty) return;
    const handler = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    addEventListener('beforeunload', handler); return () => removeEventListener('beforeunload', handler);
  }, [dirty]);

  useEffect(() => {
    if (!id) { setTerminal('not_found'); return; }
    const controller = new AbortController(); setTerminal('loading');
    getShare(id, controller.signal).then(async ({ share: loaded }) => {
      if (!validPayload(loaded)) { setTerminal('server_error'); return; }
      setShare(loaded); setExpiry(loaded.expires_at); setCustomExpiry(loaded.expires_at ? localDate(loaded.expires_at) : ''); setExpiryDirty(false);
      if (loaded.payload_kind === 'TEXT' && loaded.privacy_mode === 'STANDARD') {
        const text = loaded.text!; setFormat(text.format); setContent(text.content); setBaseline(text); setDecryptAvailable(true);
      } else if (loaded.payload_kind === 'TEXT') {
        const fragment = window.location.hash || location.hash;
        if (!fragment || fragment === '#') { setDecryptAvailable(false); return; }
        try { const text = await decryptWithFragment(fragment, loaded.encrypted_text as EncryptedPayloadV1); setFormat(text.format); setContent(text.content); setBaseline(text); setDecryptAvailable(true); }
        catch { setDecryptAvailable(false); }
      } else setDecryptAvailable(false);
      setTerminal('loading');
    }).catch((error) => {
      if (error instanceof ApiError && error.status === 404) setTerminal('not_found');
      else if (error instanceof ApiError && error.status === 410) setTerminal('expired');
      else setTerminal('server_error');
    });
    return () => controller.abort();
  }, [id, location.hash]);

  const expiryValue = useMemo(() => expiry === null ? 'never' : 'custom', [expiry]);
  const setExpiration = (value: string) => {
    setExpiryDirty(true);
    if (value === 'never') { setExpiry(null); setCustomExpiry(''); }
    else { setCustomExpiry(value); const parsed = new Date(value); setExpiry(Number.isNaN(parsed.getTime()) ? null : parsed.toISOString()); }
  };
  const handleError = (error: unknown) => {
    if (!(error instanceof ApiError)) { setMessage('Could not save changes.'); return; }
    if (error.status === 401 && id) { forget(id); setMessage('Management token was not accepted. Enter it again.'); return; }
    if (error.status === 404 && id) { forget(id); setShare(null); setTerminal('not_found'); return; }
    if (error.status === 410 && id) { forget(id); setShare(null); setTerminal('expired'); return; }
    setMessage(error.status === 429 ? `Too many requests${error.retryAfterSeconds !== undefined ? `. Try again in ${error.retryAfterSeconds} seconds.` : '.'}` : 'Could not save changes.');
  };
  const save = async () => {
    if (!id || !token || !share || !dirty || submitting || (contentDirty && (!bytes(content) || bytes(content) > maxBytes))) return;
    const patch: Parameters<typeof updateShare>[2] = {};
    if (contentDirty) {
      if (share.privacy_mode === 'ENCRYPTED') patch.encrypted_text = await encryptWithFragment(window.location.hash || location.hash, format, content);
      else patch.text = { format, content };
    }
    if (expiryDirty) patch.expires_at = expiry;
    setSubmitting(true); setMessage('');
    try { const result = await updateShare(id, token, patch); setShare(result.share); setBaseline({ format, content }); setExpiry(result.share.expires_at); setExpiryDirty(false); setMessage('Changes saved'); }
    catch (error) { handleError(error); } finally { setSubmitting(false); }
  };
  const remove = async () => {
    if (!id || !token || submitting) return; setSubmitting(true);
    try { await deleteShare(id, token); forget(id); setShare(null); setContent(''); setTokenInput(''); setDeleteOpen(false); setTerminal('deleted'); navigate(`/manage/${id}`, { replace: true }); }
    catch (error) { setDeleteOpen(false); handleError(error); } finally { setSubmitting(false); }
  };
  const cancel = () => navigate(`/s/${id}${location.hash}`);

  if (terminal === 'not_found' || terminal === 'expired' || terminal === 'server_error' || terminal === 'deleted') return <main className="page-container"><section className="viewer-card" role="alert"><h1>{terminal === 'deleted' ? 'Share deleted' : terminal === 'not_found' ? 'Share not found' : terminal === 'expired' ? 'This share has expired' : 'Service error'}</h1><p>{terminal === 'deleted' ? 'The share is no longer available.' : 'Could not load this share.'}</p><Link className="btn btn-primary" to="/">Create new share</Link></section></main>;
  if (!share) return <main className="page-container"><p>Loading share…</p></main>;
  if (!token) return <main className="page-container"><section className="viewer-card"><h1>Management token</h1>{message && <p role="alert">{message}</p>}<form onSubmit={(event) => { event.preventDefault(); if (id && tokenInput) { remember(id, tokenInput); setTokenInput(''); } }}><label>Management token<input aria-label="Management token" type={reveal ? 'text' : 'password'} autoComplete="off" spellCheck={false} autoCapitalize="none" placeholder="up_o1_..." value={tokenInput} onChange={(event) => setTokenInput(event.target.value)} /></label><Button type="button" variant="secondary" onClick={() => setReveal(!reveal)}>{reveal ? 'Hide' : 'Reveal'}</Button><Button type="submit" disabled={!tokenInput}>Continue</Button></form></section></main>;

  const textEditable = share.payload_kind === 'TEXT' && (share.privacy_mode === 'STANDARD' || decryptAvailable);
  return <main className="page-container"><section className="viewer-card manage-card"><h1>Manage share</h1>{message && <p role="status">{message}</p>}
    {share.payload_kind === 'FILE' && <dl><dt>Filename</dt><dd>{share.file!.filename}</dd><dt>Size</dt><dd>{formatFileSize(share.file!.size)}</dd><dt>Media type</dt><dd>{share.file!.media_type}</dd></dl>}
    {share.payload_kind === 'TEXT' && !textEditable && <p>{location.hash ? 'Unable to decrypt this share. Expiration and deletion remain available with the management token.' : 'Content editing is unavailable without the decryption key. You can still change expiration or delete this share.'}</p>}
    {textEditable && <><label>Format<select value={format} onChange={(event) => setFormat(event.target.value as TextFormat)}><option value="PLAIN">Plain text</option><option value="SOURCE">Source</option><option value="MARKDOWN">Markdown</option></select></label><label>Editor<textarea className={format === 'SOURCE' ? 'font-mono' : ''} value={content} onChange={(event) => setContent(event.target.value)} onKeyDown={(event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void save(); } }} /></label><small>{bytes(content)} / {maxBytes} B</small></>}
    <label>Expires<select aria-label="Expiration" value={expiryValue} onChange={(event) => { if (event.target.value === 'never') setExpiration('never'); else if (!customExpiry) setExpiration(localDate(new Date(Date.now() + 86400000).toISOString())); }}><option value="never">Never</option><option value="custom">Custom…</option></select></label>{expiryValue === 'custom' && <input aria-label="Custom expiration date and time" type="datetime-local" value={customExpiry} onChange={(event) => setExpiration(event.target.value)} />}
    <div className="viewer-actions"><Button onClick={() => void save()} disabled={!dirty || submitting || (contentDirty && (!bytes(content) || bytes(content) > maxBytes))}>{submitting ? 'Saving…' : textEditable ? 'Save changes' : 'Update expiration'}</Button><Button variant="secondary" onClick={cancel}>Cancel</Button><Button variant="danger" onClick={() => setDeleteOpen(true)}>Delete share</Button></div>
  </section>{deleteOpen && <Dialog title="Delete this share?" onClose={() => !submitting && setDeleteOpen(false)}><p>This permanently deletes the share and its content. This action cannot be undone.</p><Button variant="secondary" disabled={submitting} onClick={() => setDeleteOpen(false)}>Cancel</Button><Button variant="danger" disabled={submitting} onClick={() => void remove()}>{submitting ? 'Deleting…' : 'Delete permanently'}</Button></Dialog>}{dataRouter && <NavigationGuard dirty={dirty} />}</main>;
};
