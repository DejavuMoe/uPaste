import type { PayloadKind, PrivacyMode } from './types';

export interface AdminSession {
  authenticated: true;
  csrf: string;
  expires_at: string;
}

export interface AdminSummary {
  active: number;
  expired: number;
  text: number;
  file: number;
  encrypted: number;
  file_bytes: number;
}

export interface AdminListItem {
  id: string;
  payload_kind: PayloadKind;
  privacy_mode: PrivacyMode;
  state: 'active' | 'expired';
  created_at: string;
  updated_at: string;
  expires_at: string | null;
  payload_bytes: number;
  file_filename?: string;
  file_media_type?: string;
}

export interface AdminShareDetail extends AdminListItem {
  text?: { format: string; content: string };
  encrypted_text?: { protocol: string; nonce: string; ciphertext_bytes: number; notice: string };
  file?: { filename: string; size: number; media_type: string; sha256: string; download_url: string };
}

export class AdminApiError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'AdminApiError';
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, { credentials: 'same-origin', ...init });
  const text = await response.text();
  let payload: any = null;
  try {
    payload = text ? JSON.parse(text) : null;
  } catch {
    payload = null;
  }
  if (!response.ok) {
    const code = payload?.error?.code ?? 'unknown_error';
    const message = payload?.error?.message ?? `Request failed with status ${response.status}`;
    throw new AdminApiError(response.status, code, message);
  }
  return payload as T;
}

export async function getAdminSession(): Promise<AdminSession | null> {
  try {
    return await request<AdminSession>('/api/v1/admin/session');
  } catch (error) {
    if (error instanceof AdminApiError && error.status === 401) return null;
    throw error;
  }
}

export function loginAdmin(token: string): Promise<AdminSession> {
  return request<AdminSession>('/api/v1/admin/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  });
}

export function logoutAdmin(csrf: string): Promise<void> {
  return request<void>('/api/v1/admin/session', { method: 'DELETE', headers: { 'X-uPaste-CSRF': csrf } });
}

export function getAdminSummary(): Promise<{ summary: AdminSummary }> {
  return request<{ summary: AdminSummary }>('/api/v1/admin/summary');
}

export interface AdminListParams {
  kind?: PayloadKind;
  privacy?: PrivacyMode;
  lifecycle?: 'active' | 'expired';
  sort?: 'newest' | 'oldest';
  id?: string;
  cursor?: string;
  limit?: number;
}

export function listAdminShares(params: AdminListParams): Promise<{ shares: AdminListItem[]; next_cursor: string }> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') query.set(key, String(value));
  });
  const suffix = query.toString();
  return request<{ shares: AdminListItem[]; next_cursor: string }>(`/api/v1/admin/shares${suffix ? `?${suffix}` : ''}`);
}

export function getAdminShare(id: string, signal?: AbortSignal): Promise<{ share: AdminShareDetail }> {
  return request<{ share: AdminShareDetail }>(`/api/v1/admin/shares/${encodeURIComponent(id)}`, { signal });
}

export function deleteAdminShare(id: string, csrf: string): Promise<void> {
  return request<void>(`/api/v1/admin/shares/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: { 'X-uPaste-CSRF': csrf },
  });
}

export function bulkDeleteAdminShares(ids: string[], csrf: string): Promise<{ deleted: number; failed: string[] }> {
  return request<{ deleted: number; failed: string[] }>('/api/v1/admin/shares/bulk-delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-uPaste-CSRF': csrf },
    body: JSON.stringify({ ids }),
  });
}

export function cleanupAdminExpired(csrf: string): Promise<{ purged: number }> {
  return request<{ purged: number }>('/api/v1/admin/cleanup/expired', {
    method: 'POST',
    headers: { 'X-uPaste-CSRF': csrf },
  });
}
