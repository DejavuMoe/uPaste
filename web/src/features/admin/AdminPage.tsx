import React, { useCallback, useEffect, useMemo, useState } from 'react';
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

export const AdminPage: React.FC = () => {
  const { config } = useDeploymentConfig();
  const [session, setSession] = useState<{ csrf: string } | null>(null);
  const [tokenInput, setTokenInput] = useState('');
  const [error, setError] = useState('');
  const [summary, setSummary] = useState<AdminSummary | null>(null);
  const [shares, setShares] = useState<AdminListItem[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [kind, setKind] = useState('');
  const [privacy, setPrivacy] = useState('');
  const [lifecycle, setLifecycle] = useState<Lifecycle>('');
  const [sort, setSort] = useState<'newest' | 'oldest'>('newest');
  const [exactId, setExactId] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [detail, setDetail] = useState<AdminShareDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<AdminListItem | null>(null);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!config.admin_enabled) return;
    getAdminSession()
      .then((value) => setSession(value ? { csrf: value.csrf } : null))
      .catch(() => setSession(null));
  }, [config.admin_enabled]);

  const load = useCallback(
    async (cursor?: string, append = false) => {
      if (!session) return;
      setBusy(true);
      try {
        const [summaryResult, listResult] = await Promise.all([
          getAdminSummary(),
          listAdminShares({ kind: kind as any, privacy: privacy as any, lifecycle: lifecycle || undefined, sort, id: exactId || undefined, cursor, limit: 50 }),
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
    [session, kind, privacy, lifecycle, sort, exactId],
  );

  useEffect(() => {
    if (session) void load();
  }, [session, load]);

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
    setSession(null);
    setShares([]);
    setSummary(null);
    setSelected(new Set());
  };

  const toggleSelected = (id: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else if (next.size < 100) next.add(id);
      return next;
    });
  };

  const confirmDelete = async () => {
    if (!session || !deleteTarget) return;
    setBusy(true);
    try {
      await deleteAdminShare(deleteTarget.id, session.csrf);
      setDeleteTarget(null);
      setDetail(null);
      setSelected((current) => { const next = new Set(current); next.delete(deleteTarget.id); return next; });
      await load();
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Delete failed.');
    } finally {
      setBusy(false);
    }
  };

  const confirmBulk = async () => {
    if (!session || selected.size === 0) return;
    setBusy(true);
    try {
      await bulkDeleteAdminShares(Array.from(selected), session.csrf);
      setSelected(new Set());
      setBulkOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Bulk delete failed.');
    } finally {
      setBusy(false);
    }
  };

  const confirmCleanup = async () => {
    if (!session) return;
    setBusy(true);
    try {
      await cleanupAdminExpired(session.csrf);
      setCleanupOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof AdminApiError ? err.message : 'Cleanup failed.');
    } finally {
      setBusy(false);
    }
  };

  const openDetail = async (id: string) => {
    setDetail(null);
    setDetailError('');
    setDetailLoading(true);
    try {
      const result = await getAdminShare(id);
      setDetail(result.share);
    } catch (err) {
      setDetailError(err instanceof AdminApiError ? err.message : 'Could not load Share detail.');
    } finally {
      setDetailLoading(false);
    }
  };

  const summaryLine = useMemo(() => {
    if (!summary) return '';
    return `Stored files: ${formatFileSize(summary.file_bytes)} · active ${summary.active} · expired ${summary.expired} · text ${summary.text} · file ${summary.file} · encrypted ${summary.encrypted}`;
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

  return (
    <main className="page-container" id="main-content">
      <section className="viewer-card admin-card">
        <div className="admin-header">
          <h1>uPaste / Administration</h1>
          <div className="admin-actions">
            <Button variant="secondary" onClick={() => setCleanupOpen(true)}>Clean expired</Button>
            <Button variant="secondary" onClick={() => void handleLogout()}>Log out</Button>
          </div>
        </div>
        <p className="admin-summary" aria-label="Storage and share summary">{summaryLine}</p>
        {error && <p className="form-error" role="alert">{error}</p>}

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
          <form className="admin-lookup" onSubmit={(event) => { event.preventDefault(); void load(); }}>
            <label className="sr-only" htmlFor="admin-share-lookup">Exact Share ID</label>
            <input id="admin-share-lookup" aria-label="Exact Share ID" placeholder="Exact Share ID" value={exactId} onChange={(event) => setExactId(event.target.value)} />
            <Button type="submit" variant="secondary">Find</Button>
          </form>
          {selected.size > 0 && <Button variant="danger" onClick={() => setBulkOpen(true)}>Delete selected ({selected.size})</Button>}
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
                  <td>{formatFileSize(share.payload_bytes)}</td>
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

      {(detailLoading || detailError || detail) && (
        <Dialog title="Share detail" onClose={() => { setDetail(null); setDetailError(''); setDetailLoading(false); }}>
          {detailLoading && <p role="status">Loading Share detail…</p>}
          {detailError && <p role="alert">{detailError}</p>}
          {detail && (
            <>
              <dl className="admin-detail">
                <dt>ID</dt><dd className="font-mono">{detail.id}</dd>
                <dt>Type</dt><dd>{detail.payload_kind}</dd>
                <dt>Privacy</dt><dd>{detail.privacy_mode}</dd>
                <dt>Created</dt><dd>{new Date(detail.created_at).toLocaleString()}</dd>
                <dt>Expires</dt><dd>{detail.expires_at ? new Date(detail.expires_at).toLocaleString() : 'Never'}</dd>
                <dt>Size</dt><dd>{formatFileSize(detail.payload_bytes)}</dd>
              </dl>
              {detail.text && (
                <>
                  <h3>Standard text ({detail.text.format})</h3>
                  <pre className="admin-text-preview" tabIndex={0}>{detail.text.content}</pre>
                </>
              )}
              {detail.encrypted_text && (
                <div className="privacy-notice" role="note">
                  Client-side encrypted — plaintext unavailable to the server. Ciphertext size: {detail.encrypted_text.ciphertext_bytes} bytes.
                </div>
              )}
              {detail.file && (
                <>
                  <h3>File metadata</h3>
                  <dl className="admin-detail">
                    <dt>Filename</dt><dd>{detail.file.filename}</dd>
                    <dt>Size</dt><dd>{formatFileSize(detail.file.size)}</dd>
                    <dt>Media type</dt><dd>{detail.file.media_type}</dd>
                    <dt>SHA-256</dt><dd className="font-mono">{detail.file.sha256}</dd>
                  </dl>
                  <a className="btn btn-secondary" href={detail.file.download_url} target="_blank" rel="noopener noreferrer">Download file</a>
                </>
              )}
              <div className="viewer-actions">
                <Button variant="secondary" onClick={() => setDetail(null)}>Close</Button>
                <Button variant="danger" onClick={() => setDeleteTarget(detail)}>Delete share</Button>
              </div>
            </>
          )}
        </Dialog>
      )}

      {deleteTarget && (
        <Dialog title="Delete this share?" onClose={() => !busy && setDeleteTarget(null)}>
          <p>This permanently deletes the share and any File payload. This action cannot be undone.</p>
          <Button variant="secondary" disabled={busy} onClick={() => setDeleteTarget(null)}>Cancel</Button>
          <Button variant="danger" disabled={busy} onClick={() => void confirmDelete()}>{busy ? 'Deleting…' : 'Delete permanently'}</Button>
        </Dialog>
      )}

      {bulkOpen && (
        <Dialog title={`Delete ${selected.size} shares?`} onClose={() => !busy && setBulkOpen(false)}>
          <p>This permanently deletes the selected shares and File payloads. This action cannot be undone.</p>
          <Button variant="secondary" disabled={busy} onClick={() => setBulkOpen(false)}>Cancel</Button>
          <Button variant="danger" disabled={busy} onClick={() => void confirmBulk()}>{busy ? 'Deleting…' : 'Delete selected'}</Button>
        </Dialog>
      )}

      {cleanupOpen && (
        <Dialog title="Clean expired content?" onClose={() => !busy && setCleanupOpen(false)}>
          <p>This runs the established expiration purge/reconciliation pass.</p>
          <Button variant="secondary" disabled={busy} onClick={() => setCleanupOpen(false)}>Cancel</Button>
          <Button variant="primary" disabled={busy} onClick={() => void confirmCleanup()}>{busy ? 'Cleaning…' : 'Clean expired'}</Button>
        </Dialog>
      )}
    </main>
  );
};
