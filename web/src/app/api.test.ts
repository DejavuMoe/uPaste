import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  parseRetryAfter,
  ApiError,
  createStandardText,
  createEncryptedText,
  createFile,
} from './api';

describe('api client', () => {
  describe('parseRetryAfter', () => {
    it('returns undefined for null or empty string', () => {
      expect(parseRetryAfter(null)).toBeUndefined();
      expect(parseRetryAfter('')).toBeUndefined();
    });

    it('parses delta-seconds integer', () => {
      expect(parseRetryAfter('30')).toBe(30);
      expect(parseRetryAfter('0')).toBe(0);
      expect(parseRetryAfter('120')).toBe(120);
    });

    it('parses HTTP-date string in the future', () => {
      const now = 1700000000000;
      vi.spyOn(Date, 'now').mockReturnValue(now);

      const targetDate = new Date(now + 45000).toUTCString();
      expect(parseRetryAfter(targetDate)).toBe(45);

      vi.restoreAllMocks();
    });

    it('returns 0 for HTTP-date in the past', () => {
      const now = 1700000000000;
      vi.spyOn(Date, 'now').mockReturnValue(now);

      const pastDate = new Date(now - 10000).toUTCString();
      expect(parseRetryAfter(pastDate)).toBe(0);

      vi.restoreAllMocks();
    });

    it('returns undefined for unparseable garbage', () => {
      expect(parseRetryAfter('invalid-header-value')).toBeUndefined();
    });
  });

  describe('createStandardText', () => {
    beforeEach(() => {
      vi.restoreAllMocks();
    });

    it('sends POST /api/v1/shares with json and returns parsed response', async () => {
      const mockResponse = {
        share: {
          id: 'share123',
          payload_kind: 'TEXT' as const,
          privacy_mode: 'STANDARD' as const,
          created_at: '2026-09-14T12:00:00Z',
          updated_at: '2026-09-14T12:00:00Z',
          expires_at: null,
        },
        owner_token: 'up_o1_secret',
      };

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockResponse,
      });

      const res = await createStandardText({
        payload_kind: 'TEXT',
        privacy_mode: 'STANDARD',
        text: { format: 'PLAIN', content: 'hello world' },
        expires_at: null,
      });

      expect(res).toEqual(mockResponse);
      expect(fetch).toHaveBeenCalledWith('/api/v1/shares', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          payload_kind: 'TEXT',
          privacy_mode: 'STANDARD',
          text: { format: 'PLAIN', content: 'hello world' },
          expires_at: null,
        }),
        signal: undefined,
      });
    });

    it('throws ApiError on HTTP 400 error response', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        headers: new Headers(),
        json: async () => ({
          error: { code: 'invalid_request', message: 'content cannot be empty' },
        }),
      });

      await expect(
        createStandardText({
          payload_kind: 'TEXT',
          privacy_mode: 'STANDARD',
          text: { format: 'PLAIN', content: '' },
          expires_at: null,
        }),
      ).rejects.toThrow(ApiError);
    });

    it('parses Retry-After header on 429 response', async () => {
      const headers = new Headers();
      headers.set('Retry-After', '60');

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers,
        json: async () => ({
          error: { code: 'rate_limit_exceeded', message: 'Rate limit exceeded' },
        }),
      });

      try {
        await createStandardText({
          payload_kind: 'TEXT',
          privacy_mode: 'STANDARD',
          text: { format: 'PLAIN', content: 'test' },
          expires_at: null,
        });
        expect.unreachable('Should have thrown');
      } catch (err: any) {
        expect(err).toBeInstanceOf(ApiError);
        expect(err.status).toBe(429);
        expect(err.code).toBe('rate_limit_exceeded');
        expect(err.retryAfterSeconds).toBe(60);
      }
    });
  });

  describe('createEncryptedText', () => {
    it('sends POST /api/v1/shares with encrypted payload', async () => {
      const mockResponse = {
        share: {
          id: 'enc123',
          payload_kind: 'TEXT' as const,
          privacy_mode: 'ENCRYPTED' as const,
          created_at: '2026-09-14T12:00:00Z',
          updated_at: '2026-09-14T12:00:00Z',
          expires_at: null,
        },
        owner_token: 'up_o1_owner',
      };

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockResponse,
      });

      const res = await createEncryptedText({
        payload_kind: 'TEXT',
        privacy_mode: 'ENCRYPTED',
        encrypted_text: {
          protocol: 'UPASTE_AES_GCM_V1',
          nonce: 'nonce123',
          ciphertext: 'cipher123',
        },
        expires_at: null,
      });

      expect(res).toEqual(mockResponse);
    });
  });

  describe('createFile', () => {
    let originalXHR: any;

    beforeEach(() => {
      originalXHR = globalThis.XMLHttpRequest;
    });

    afterEach(() => {
      globalThis.XMLHttpRequest = originalXHR;
    });

    it('submits multipart/form-data via XMLHttpRequest and tracks progress', async () => {
      const mockResponse = {
        share: {
          id: 'file123',
          payload_kind: 'FILE' as const,
          privacy_mode: 'STANDARD' as const,
          created_at: '2026-09-14T12:00:00Z',
          updated_at: '2026-09-14T12:00:00Z',
          expires_at: null,
        },
        owner_token: 'up_o1_filetoken',
      };

      let sentData: FormData | null = null;
      let mockXHR: any;
      mockXHR = {
        open: vi.fn(),
        send: vi.fn(function (data: FormData) {
          sentData = data;
          if (mockXHR.upload?.onprogress) {
            mockXHR.upload.onprogress({
              lengthComputable: true,
              loaded: 500,
              total: 1000,
            });
          }
          mockXHR.status = 201;
          mockXHR.responseText = JSON.stringify(mockResponse);
          if (mockXHR.onload) {
            mockXHR.onload();
          }
        }),
        status: 201,
        responseText: '',
        getResponseHeader: vi.fn().mockReturnValue(null),
        upload: {},
        abort: vi.fn(),
      };

      globalThis.XMLHttpRequest = vi.fn().mockImplementation(function (this: any) {
        return mockXHR;
      }) as any;

      const dummyFile = new File(['file contents'], 'test.txt', { type: 'text/plain' });
      const progressUpdates: any[] = [];

      const result = await createFile(
        dummyFile,
        null,
        (p) => progressUpdates.push(p),
      );

      expect(result).toEqual(mockResponse);
      expect(mockXHR.open).toHaveBeenCalledWith('POST', '/api/v1/shares');
      expect(sentData).toBeInstanceOf(FormData);
      expect(progressUpdates.length).toBe(1);
      expect(progressUpdates[0].percent).toBe(50);
    });

    it('rejects when aborted via signal', async () => {
      const mockXHR = {
        open: vi.fn(),
        send: vi.fn(),
        upload: { addEventListener: vi.fn() },
        addEventListener: vi.fn(),
        abort: vi.fn(),
      };

      globalThis.XMLHttpRequest = vi.fn().mockImplementation(function (this: any) {
        return mockXHR;
      }) as any;

      const controller = new AbortController();
      controller.abort();

      const dummyFile = new File(['test'], 'test.txt');

      await expect(createFile(dummyFile, null, undefined, controller.signal)).rejects.toThrow(
        /aborted/i,
      );
    });
  });
});
