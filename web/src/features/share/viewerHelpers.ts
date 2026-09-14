import type { TextFormat } from '../../app/types';

export function formatFormatLabel(format: TextFormat): string {
  switch (format) {
    case 'PLAIN':
      return 'Plain text';
    case 'SOURCE':
      return 'Source';
    case 'MARKDOWN':
      return 'Markdown';
    default:
      return 'Plain text';
  }
}

export function formatContentSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KiB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export function formatExpiration(expiresAt: string | null, now?: Date): string {
  if (!expiresAt) {
    return 'Never expires';
  }
  const targetMs = Date.parse(expiresAt);
  if (isNaN(targetMs)) {
    return 'Never expires';
  }

  const currentMs = now ? now.getTime() : Date.now();
  const diffMs = targetMs - currentMs;

  if (diffMs <= 0) {
    return 'Expired';
  }
  if (diffMs < 60_000) {
    return 'Expires in less than a minute';
  }

  const diffMinutes = Math.round(diffMs / 60_000);
  if (diffMinutes < 60) {
    return `Expires in ${diffMinutes} ${diffMinutes === 1 ? 'minute' : 'minutes'}`;
  }

  const diffHours = Math.round(diffMs / 3600_000);
  if (diffHours < 24) {
    return `Expires in ${diffHours} ${diffHours === 1 ? 'hour' : 'hours'}`;
  }

  const diffDays = Math.round(diffMs / 86400_000);
  if (diffDays < 30) {
    return `Expires in ${diffDays} ${diffDays === 1 ? 'day' : 'days'}`;
  }

  const dateObj = new Date(targetMs);
  const formattedDate = dateObj.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  return `Expires ${formattedDate}`;
}

export function isSafeUrl(url: string | undefined): boolean {
  if (!url) return false;
  const trimmed = url.trim();
  if (/[\u0000-\u001F\u007F-\u009F]/.test(trimmed)) {
    return false;
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' || parsed.protocol === 'mailto:';
  } catch {
    return false;
  }
}

export function sanitizeFilename(filename: string | undefined): string {
  if (!filename) return 'file';
  const basename = filename.split(/[/\\]/).pop() || 'file';
  const cleaned = basename.replace(/[\u0000-\u001F\u007F-\u009F]/g, '').trim();
  return cleaned || 'file';
}
