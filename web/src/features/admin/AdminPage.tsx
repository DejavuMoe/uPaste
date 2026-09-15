import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router';
import { useDeploymentConfig } from '../../app/config';
import {
  AdminApiError,
  bulkDeleteAdminShares,
  cleanupAdminExpired,
  deleteAdminShare,
  getAdminSession,
  getAdminShare,
  getAdminSummary,
  listAdminShares,
  loginAdmin,
  logoutAdmin,
  type AdminListItem,
  type AdminShareDetail,
  type AdminSummary,
} from '../../app/adminApi';
import { Button } from '../../components/Button';
import { Dialog } from '../../components/Dialog';
import { formatFileSize } from '../share/viewerHelpers';

type Lifecycle = '' | 'active' | 'expired';

type AdminModal =
  | { kind: 'detail'; phase: 'loading' | 'ready' | 'error'; share?: AdminShareDetail; error?: string }
  | { kind: 'delete'; target: AdminListItem; returnToDetail?: AdminShareDetail }
  | { kind: 'bulk' }
  | { kind: 'cleanup' };

const SHARE_ID_RE = /^[A-Za-z0-9_-]{22}$/;

function finiteBytes(value: number | null | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : 0;
}

function adminSize(value: number | null | undefined): string {
  return formatFileSize(finiteBytes(value));
}

function pruneSelection(current: Set<string>, visible: AdminListItem[]): Set<string> {
  const visibleIDs = new Set(visible.map((share) => share.id));
  const next = new Set<string>();
  let changed = false;
  current.forEach((id) => {
    if (visibleIDs.has(id)) next.add(id);
    else changed = true;
  });
  return changed ? next : current;
}

export const AdminPage: React.FC = () => {
  const { config } = useDeploymentConfig();
  const [session, setSession] = useState<{ csrf: string } | null>(null);
  const [tokenInput, setTokenInput] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [summary, setSummary] = useState<AdminSummary | null>(null);
  const [shares, setShares] = useState<AdminListItem[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [kind, setKind] = useState('');
  const [privacy, setPrivacy] = useState('');
  const [lifecycle, setLifecycle] = useState<Lifecycle>('');
  const [sort, setSort] = useState<'newest' | 'oldest'>('newest');
  const [exactDraft, setExactDraft] = useState('');
  const [appliedExactId, setAppliedExactId] = useState('');
  const [lookupError, setLookupError] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [modal, setModal] = useState<AdminModal | null>(null);
  const [busy, setBusy] = useState(false);

  const detailGenerationRef = useRef(0);
  const detailControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!config.admin_enabled) return;
    getAdminSession()
      .then((value) => setSession(value ? { csrf: value.csrf } : null))
      .catch(() => setSession(null));
  }, [config.admin_enabled]);

  useEffect(() => () => {
    detailGenerationRef.current += 1;
    detailControllerRef.current?.abort();
  }, []);

  const load = useCallback(
    async (cursor?: string, append = false) => {
      if (!session) return;
      setBusy(true);
      try {
        const [summaryResult, listResult] = await Promise.all([
          getAdminSummary(),
          listAdminShares({
            kind: kind as any,
            privacy: privacy as any,
            lifecycle: lifecycle || undefined,
            sort,
            id: appliedExactId || undefined,
            cursor,
            limit: 50,
          }),
        ]);
        setSummary(summaryResult.summary);
        setShares((current) => (append ? [...current, ...listResult.shares] : listResult.shares));
        setNextCursor(listResult.next_cursor ?? '');
        setError('');
      } catch (err) {
        setError(err instanceof AdminApiError ? err.message : 'Could not load administration data.');
      } finally {
        setBusy(false);
      }
    },
    [session, kind, privacy, lifecycle, sort, appliedExactId],
  );

  useEffect(() => {
    if (session) void load();
  }, [session, load]);

  // Any change to the effective base query invalidates previously selected rows.
  useEffect(() => {
    setSelected(new Set());
  }, [kind, privacy, lifecycle, sort, appliedExactId]);

  // Keep only visible rows selected after ordinary loads and deletes.
  useEffect(() => {
    setSelected((current) => pruneSelection(current, shares));
  }, [shares]);

  const closeModal = () => {
    detailGenerationRef.current += 1;
    detailControllerRef.current?.abort();
    detailControllerRef.current = null;
    setModal(null);
  };

  const handleLogin = async (event: React.FormEvent) => {
    event.preventDefault();
    setError('');
    try {
      const value = await loginAdmin(tokenInput);
      setSession({ csrf: value.csrf });
      setTokenInput('');
    } catch (err) {
      setError(err instanceof AdminApiError && err.code === 'rate_limited' ? 'Too many attempts. Try again shortly.' : 'Authentication failed.');
    }
  };

  const handleLogout = async () => {
    if (!session) return;
    try {
      await logoutAdmin(session.csrf);
    } catch {
      // Ignore; clear local state regardless.
    }
    closeModal();
    setSession(null);
    setShares([]);
    setSummary(null);
    setSelected(new Set());
    setNotice('');
  };

  const toggleSelected = (id: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else if (next.size < 100) next.add(id);
      return next;
    });
  };

  const openDetail = async (id: string) => {
    detailControllerRef.current?.abort();
    const generation = detailGenerationRef.current + 1;
    detailGenerationRef.current = generation;
    const controller = new AbortController();
    detailControllerRef.current = controller;
    setModal({ kind: 'detail', phase: 'loading' });
    try {
      const result = await getAdminShare(id, controller.signal);
      if (controller.signal.aborted || generation !== detailGenerationRef.current) return;
      setModal({ kind: 'detail', phase: 'ready', share: result.share });
    } catch (err) {
      if (controller.signal.aborted || (err as any)?.name === 'AbortError') return;
      if (generation !== detailGenerationRef.current) return;
      setModal({
        kind: 'detail',
        phase: 'error',
        error: err instanceof AdminApiError ? err.message : 'Could not load Share detail.',
      });
    }
  };

  const cancelDelete = () => {
    if (busy || !modal || modal.kind !== 'delete') return;
    if (modal.returnToDetail) {
      setModal({ kind: 'detail', phase: 'ready', share: modal.returnToDetail });
    } else {
      setModal(null);
    }
  };

  const confirmDelete = async () => {
    if (!session || !modal || modal.kind !== 'delete') return;
    const target = modal.target;
    setBusy(true);
    setNotice('');
    try {
      await deleteAdminShare(target.id, session.csrf);
      setModal(null);
      setSelected((current) => {
        const next = new Set(current);
        next.delete(target.id);
        return next;
      });
      await load();
      const focusLookup = () => document.getElementById('admin-share-lookup')?.focus();
      if (typeof window.requestAnimationFrame === 'function') window.requestAnimationFrame(focusLookup);
      else window.setTimeout(focusLookup, 0);
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Delete failed.');
    } finally {
      setBusy(false);
    }
  };

  const confirmBulk = async () => {
    if (!session || !modal || modal.kind !== 'bulk' || selected.size === 0) return;
    setBusy(true);
    setNotice('');
    try {
      const result = await bulkDeleteAdminShares(Array.from(selected), session.csrf);
      setModal(null);
      if (result.failed.length === 0) {
        setSelected(new Set());
        setNotice(`Deleted ${result.deleted} share${result.deleted === 1 ? '' : 's'}.`);
      } else {
        // Keep failed IDs selected; the visible-row pruning effect removes any
        // that are no longer present after the reload.
        setSelected(new Set(result.failed));
        setNotice(`Deleted ${result.deleted} share${result.deleted === 1 ? '' : 's'}; ${result.failed.length} could not be deleted.`);
      }
      await load();
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Bulk delete failed.');
    } finally {
      setBusy(false);
    }
  };

  const confirmCleanup = async () => {
    if (!session || !modal || modal.kind !== 'cleanup') return;
    setBusy(true);
    setNotice('');
    try {
      await cleanupAdminExpired(session.csrf);
      setModal(null);
      await load();
      setNotice('Expired cleanup completed.');
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Cleanup failed.');
    } finally {
      setBusy(false);
    }
  };

  const applyLookup = (event: React.FormEvent) => {
    event.preventDefault();
    const value = exactDraft.trim();
    if (value === '') {
      setLookupError('');
      if (appliedExactId === '') void load();
      else setAppliedExactId('');
      return;
    }
    if (!SHARE_ID_RE.test(value)) {
      setLookupError('Share IDs are 22 URL-safe characters.');
      return;
    }
    setLookupError('');
    if (value === appliedExactId) {
      void load();
      return;
    }
    setAppliedExactId(value);
  };

  const clearLookup = () => {
    setExactDraft('');
    setLookupError('');
    if (appliedExactId === '') void load();
    else setAppliedExactId('');
  };

  const summaryLine = useMemo(() => {
    if (!summary) return '';
    return `Stored files: ${adminSize(summary.file_bytes)} · active ${summary.active} · expired ${summary.expired} · text ${summary.text} · file ${summary.file} · encrypted ${summary.encrypted}`;
  }, [summary]);

  if (!config.admin_enabled) {
    return (
      <main className="page-container" id="main-content">
        <section className="viewer-card manage-card" role="alert">
          <h1>Administration unavailable</h1>
          <p>Superadmin governance is not enabled on this deployment.</p>
          <Link className="btn btn-primary" to="/">Create a share</Link>
        </section>
      </main>
    );
  }

  if (!session) {
    return (
      <main className="page-container" id="main-content">
        <section className="viewer-card manage-card">
          <h1>uPaste / Administration</h1>
          <form onSubmit={(event) => void handleLogin(event)}>
            <label>
              Admin token
              <input
                aria-label="Admin token"
                type="password"
                autoComplete="off"
                spellCheck={false}
                value={tokenInput}
                onChange={(event) => setTokenInput(event.target.value)}
              />
            </label>
            {error && <p role="alert">{error}</p>}
            <Button type="submit" disabled={!tokenInput || busy}>Sign in</Button>
          </form>
        </section>
      </main>
    );
  }

  const detailShare = modal?.kind === 'detail' ? modal.share : undefined;

  return (
    <main className="page-container" id="main-content">
      <section className="viewer-card admin-card">
        <div className="admin-header">
          <h1>uPaste / Administration</h1>
          <div className="admin-actions">
            <Button variant="secondary" onClick={() => setModal({ kind: 'cleanup' })}>Clean expired</Button>
            <Button variant="secondary" onClick={() => void handleLogout()}>Log out</Button>
          </div>
        </div>
        <p className="admin-summary" aria-label="Storage and share summary">{summaryLine}</p>
        {error && <p className="form-error" role="alert">{error}</p>}
        {notice && <p className="admin-summary" role="status">{notice}</p>}

        <div className="admin-filters" role="group" aria-label="Share filters">
          <select aria-label="Type filter" value={kind} onChange={(event) => setKind(event.target.value)}>
            <option value="">All types</option><option value="TEXT">Text</option><option value="FILE">File</option>
          </select>
          <select aria-label="Privacy filter" value={privacy} onChange={(event) => setPrivacy(event.target.value)}>
            <option value="">All privacy</option><option value="STANDARD">Standard</option><option value="ENCRYPTED">Encrypted</option>
          </select>
          <select aria-label="Lifecycle filter" value={lifecycle} onChange={(event) => setLifecycle(event.target.value as Lifecycle)}>
            <option value="">All states</option><option value="active">Active</option><option value="expired">Expired</option>
          </select>
          <select aria-label="Sort order" value={sort} onChange={(event) => setSort(event.target.value as 'newest' | 'oldest')}>
            <option value="newest">Newest</option><option value="oldest">Oldest</option>
          </select>
          <form className="admin-lookup" onSubmit={applyLookup}>
            <label className="sr-only" htmlFor="admin-share-lookup">Exact Share ID</label>
            <input
              id="admin-share-lookup"
              aria-label="Exact Share ID"
              placeholder="Exact Share ID"
              value={exactDraft}
              onChange={(event) => setExactDraft(event.target.value)}
            />
            <Button type="submit" variant="secondary">Find</Button>
            {(exactDraft || appliedExactId) && (
              <Button type="button" variant="ghost" onClick={clearLookup}>Clear</Button>
            )}
          </form>
          {lookupError && <p className="form-error" role="alert">{lookupError}</p>}
          {selected.size > 0 && <Button variant="danger" onClick={() => setModal({ kind: 'bulk' })}>Delete selected ({selected.size})</Button>}
        </div>

        <div className="admin-table-wrap">
          <table className="admin-table">
            <thead><tr><th scope="col"><span className="sr-only">Select</span></th><th scope="col">ID</th><th scope="col">Type</th><th scope="col">Privacy</th><th scope="col">Created</th><th scope="col">Expires</th><th scope="col">State</th><th scope="col">Size</th><th scope="col"><span className="sr-only">Actions</span></th></tr></thead>
            <tbody>
              {shares.map((share) => (
                <tr key={share.id}>
                  <td><input aria-label={`Select ${share.id}`} type="checkbox" checked={selected.has(share.id)} onChange={() => toggleSelected(share.id)} /></td>
                  <td className="font-mono">{share.id}</td>
                  <td>{share.payload_kind}</td>
                  <td>{share.privacy_mode}</td>
                  <td>{new Date(share.created_at).toLocaleString()}</td>
                  <td>{share.expires_at ? new Date(share.expires_at).toLocaleString() : 'Never'}</td>
                  <td>{share.state}</td>
                  <td>{adminSize(share.payload_bytes)}</td>
                  <td><Button variant="secondary" onClick={() => void openDetail(share.id)}>Inspect</Button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="admin-actions">
          {nextCursor && <Button variant="secondary" disabled={busy} onClick={() => void load(nextCursor, true)}>Load more</Button>}
        </div>
      </section>

      {modal?.kind === 'detail' && (
        <Dialog title="Share detail" onClose={closeModal}>
          {modal.phase === 'loading' && (
            <>
              <p role="status">Loading Share detail…</p>
              <div className="viewer-actions"><Button variant="secondary" onClick={closeModal}>Close</Button></div>
            </>
          )}
          {modal.phase === 'error' && (
            <>
              <p role="alert">{modal.error}</p>
              <div className="viewer-actions"><Button variant="secondary" onClick={closeModal}>Close</Button></div>
            </>
          )}
          {modal.phase === 'ready' && detailShare && (
            <>
              <dl className="admin-detail">
                <dt>ID</dt><dd className="font-mono">{detailShare.id}</dd>
                <dt>Type</dt><dd>{detailShare.payload_kind}</dd>
                <dt>Privacy</dt><dd>{detailShare.privacy_mode}</dd>
                <dt>Created</dt><dd>{new Date(detailShare.created_at).toLocaleString()}</dd>
                <dt>Expires</dt><dd>{detailShare.expires_at ? new Date(detailShare.expires_at).toLocaleString() : 'Never'}</dd>
                <dt>Size</dt><dd>{adminSize(detailShare.payload_bytes)}</dd>
              </dl>
              {detailShare.text && (
                <>
                  <h3>Standard text ({detailShare.text.format})</h3>
                  <pre className="admin-text-preview" tabIndex={0}>{detailShare.text.content}</pre>
                </>
              )}
              {detailShare.encrypted_text && (
                <div className="privacy-notice" role="note">
                  Client-side encrypted — plaintext unavailable to the server. Ciphertext size: {finiteBytes(detailShare.encrypted_text.ciphertext_bytes)} bytes.
                </div>
              )}
              {detailShare.file && (
                <>
                  <h3>File metadata</h3>
                  <dl className="admin-detail">
                    <dt>Filename</dt><dd>{detailShare.file.filename}</dd>
                    <dt>Size</dt><dd>{adminSize(detailShare.file.size)}</dd>
                    <dt>Media type</dt><dd>{detailShare.file.media_type}</dd>
                    <dt>SHA-256</dt><dd className="font-mono">{detailShare.file.sha256}</dd>
                  </dl>
                  <a className="btn btn-secondary" href={detailShare.file.download_url} target="_blank" rel="noopener noreferrer">Download file</a>
                </>
              )}
              <div className="viewer-actions">
                <Button variant="secondary" onClick={closeModal}>Close</Button>
                <Button variant="danger" onClick={() => setModal({ kind: 'delete', target: detailShare, returnToDetail: detailShare })}>Delete share</Button>
              </div>
            </>
          )}
        </Dialog>
      )}

      {modal?.kind === 'delete' && (
        <Dialog title="Delete this share?" onClose={cancelDelete}>
          <p>This permanently deletes the share and any File payload. This action cannot be undone.</p>
          <Button variant="secondary" disabled={busy} onClick={cancelDelete}>Cancel</Button>
          <Button variant="danger" disabled={busy} onClick={() => void confirmDelete()}>{busy ? 'Deleting…' : 'Delete permanently'}</Button>
        </Dialog>
      )}

      {modal?.kind === 'bulk' && (
        <Dialog title={`Delete ${selected.size} shares?`} onClose={() => !busy && setModal(null)}>
          <p>This permanently deletes the selected shares and File payloads. This action cannot be undone.</p>
          <Button variant="secondary" disabled={busy} onClick={() => setModal(null)}>Cancel</Button>
          <Button variant="danger" disabled={busy} onClick={() => void confirmBulk()}>{busy ? 'Deleting…' : 'Delete selected'}</Button>
        </Dialog>
      )}

      {modal?.kind === 'cleanup' && (
        <Dialog title="Clean expired content?" onClose={() => !busy && setModal(null)}>
          <p>This runs the established expiration purge/reconciliation pass.</p>
          <Button variant="secondary" disabled={busy} onClick={() => setModal(null)}>Cancel</Button>
          <Button variant="primary" disabled={busy} onClick={() => void confirmCleanup()}>{busy ? 'Cleaning…' : 'Clean expired'}</Button>
        </Dialog>
      )}
    </main>
  );
};
