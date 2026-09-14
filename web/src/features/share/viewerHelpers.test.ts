import { describe, it, expect } from 'vitest';
import {
  formatFormatLabel,
  formatContentSize,
  formatFileSize,
  formatExpiration,
  isSafeUrl,
  sanitizeFilename,
} from './viewerHelpers';

describe('viewerHelpers', () => {
  describe('formatFormatLabel', () => {
    it('returns user-facing label for each TextFormat', () => {
      expect(formatFormatLabel('PLAIN')).toBe('Plain text');
      expect(formatFormatLabel('SOURCE')).toBe('Source');
      expect(formatFormatLabel('MARKDOWN')).toBe('Markdown');
      expect(formatFormatLabel('UNKNOWN' as any)).toBe('Plain text');
    });
  });

  describe('formatContentSize', () => {
    it('formats bytes correctly', () => {
      expect(formatContentSize(0)).toBe('0 B');
      expect(formatContentSize(500)).toBe('500 B');
      expect(formatContentSize(1023)).toBe('1023 B');
      expect(formatContentSize(1024)).toBe('1.0 KB');
      expect(formatContentSize(4300)).toBe('4.2 KB');
      expect(formatContentSize(12697)).toBe('12.4 KB');
    });
  });

  describe('formatFileSize', () => {
    it('formats file bytes in B, KiB, or MiB', () => {
      expect(formatFileSize(512)).toBe('512 B');
      expect(formatFileSize(1024)).toBe('1.0 KiB');
      expect(formatFileSize(1024 * 1024)).toBe('1.0 MiB');
      expect(formatFileSize(14.2 * 1024 * 1024)).toBe('14.2 MiB');
    });
  });

  describe('formatExpiration', () => {
    const baseTime = new Date('2026-09-15T12:00:00Z');

    it('returns "Never expires" for null, undefined, or invalid strings', () => {
      expect(formatExpiration(null)).toBe('Never expires');
      expect(formatExpiration('invalid-date')).toBe('Never expires');
    });

    it('returns "Expired" for past dates', () => {
      const past = new Date(baseTime.getTime() - 1000).toISOString();
      expect(formatExpiration(past, baseTime)).toBe('Expired');
    });

    it('returns "Expires in less than a minute" for < 60s', () => {
      const soon = new Date(baseTime.getTime() + 30_000).toISOString();
      expect(formatExpiration(soon, baseTime)).toBe('Expires in less than a minute');
    });

    it('returns minutes for < 60m', () => {
      const fiveMin = new Date(baseTime.getTime() + 5 * 60_000).toISOString();
      expect(formatExpiration(fiveMin, baseTime)).toBe('Expires in 5 minutes');

      const oneMin = new Date(baseTime.getTime() + 60_000).toISOString();
      expect(formatExpiration(oneMin, baseTime)).toBe('Expires in 1 minute');
    });

    it('returns hours for < 24h', () => {
      const twoHours = new Date(baseTime.getTime() + 2 * 3600_000).toISOString();
      expect(formatExpiration(twoHours, baseTime)).toBe('Expires in 2 hours');

      const oneHour = new Date(baseTime.getTime() + 3600_000).toISOString();
      expect(formatExpiration(oneHour, baseTime)).toBe('Expires in 1 hour');
    });

    it('returns days for < 30d', () => {
      const sixDays = new Date(baseTime.getTime() + 6 * 86400_000).toISOString();
      expect(formatExpiration(sixDays, baseTime)).toBe('Expires in 6 days');

      const oneDay = new Date(baseTime.getTime() + 86400_000).toISOString();
      expect(formatExpiration(oneDay, baseTime)).toBe('Expires in 1 day');
    });

    it('returns formatted date for >= 30d', () => {
      const longTerm = new Date('2026-11-20T12:00:00Z').toISOString();
      expect(formatExpiration(longTerm, baseTime)).toBe('Expires Nov 20, 2026');
    });
  });

  describe('isSafeUrl', () => {
    it('accepts valid http, https, and mailto URLs', () => {
      expect(isSafeUrl('https://example.com')).toBe(true);
      expect(isSafeUrl('http://example.com/path?foo=bar#hash')).toBe(true);
      expect(isSafeUrl('mailto:user@example.com')).toBe(true);
      expect(isSafeUrl('  https://secure.site.org  ')).toBe(true);
    });

    it('rejects unsafe schemes', () => {
      expect(isSafeUrl('javascript:alert(1)')).toBe(false);
      expect(isSafeUrl('JAVASCRIPT:alert(1)')).toBe(false);
      expect(isSafeUrl('data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==')).toBe(false);
      expect(isSafeUrl('file:///etc/passwd')).toBe(false);
      expect(isSafeUrl('blob:https://example.com/uuid')).toBe(false);
      expect(isSafeUrl('vbscript:msgbox(1)')).toBe(false);
    });

    it('rejects relative URLs and empty/null/whitespace inputs', () => {
      expect(isSafeUrl('/relative/path')).toBe(false);
      expect(isSafeUrl('../relative/path')).toBe(false);
      expect(isSafeUrl('#fragment')).toBe(false);
      expect(isSafeUrl('//protocol-relative.com')).toBe(false);
      expect(isSafeUrl('')).toBe(false);
      expect(isSafeUrl('   ')).toBe(false);
      expect(isSafeUrl(undefined)).toBe(false);
    });

    it('rejects URLs containing control characters', () => {
      expect(isSafeUrl('javascript\u0000:alert(1)')).toBe(false);
      expect(isSafeUrl('https://example.com/\u0001evil')).toBe(false);
      expect(isSafeUrl('https://example.com/\r\nevil')).toBe(false);
    });
  });

  describe('sanitizeFilename', () => {
    it('extracts basename and removes path traversals', () => {
      expect(sanitizeFilename('report.pdf')).toBe('report.pdf');
      expect(sanitizeFilename('/etc/passwd')).toBe('passwd');
      expect(sanitizeFilename('../../../secret.txt')).toBe('secret.txt');
      expect(sanitizeFilename('C:\\Windows\\System32\\cmd.exe')).toBe('cmd.exe');
    });

    it('strips control characters', () => {
      expect(sanitizeFilename('report\u0000.pdf')).toBe('report.pdf');
      expect(sanitizeFilename('evil\r\nname.txt')).toBe('evilname.txt');
    });

    it('returns fallback for empty inputs', () => {
      expect(sanitizeFilename('')).toBe('file');
      expect(sanitizeFilename(undefined)).toBe('file');
      expect(sanitizeFilename('   ')).toBe('file');
      expect(sanitizeFilename('\u0000\u0001')).toBe('file');
    });
  });
});
