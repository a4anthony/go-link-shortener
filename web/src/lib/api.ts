import type {
  ApiErrorBody,
  CreateLinkInput,
  CreateWebhookInput,
  Link,
  LinkStats,
  LinksPage,
  Overview,
  UpdateLinkInput,
  Webhook,
  WebhooksPage,
} from './types';

const KEY_STORAGE = 'linkwire.apiKey';
const BASE_STORAGE = 'linkwire.baseUrl';

// The dev seed prints this key; it's the default so the demo works immediately.
export const DEFAULT_API_KEY = 'sk_live_demo-seed-key';

export function getApiKey(): string {
  return localStorage.getItem(KEY_STORAGE) ?? DEFAULT_API_KEY;
}
export function setApiKey(key: string): void {
  localStorage.setItem(KEY_STORAGE, key);
}
export function getBaseUrl(): string {
  // Empty string = same origin (nginx/vite proxy handles /api). Overridable for
  // pointing the console at a remote instance.
  return localStorage.getItem(BASE_STORAGE) ?? '';
}
export function setBaseUrl(url: string): void {
  localStorage.setItem(BASE_STORAGE, url.replace(/\/$/, ''));
}

/** ApiError carries the server's error envelope code + message. */
export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(getBaseUrl() + path, {
    method,
    headers: {
      Authorization: `Bearer ${getApiKey()}`,
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;

  if (!res.ok) {
    const envelope = data as ApiErrorBody | undefined;
    throw new ApiError(
      res.status,
      envelope?.error?.code ?? 'error',
      envelope?.error?.message ?? `request failed (${res.status})`,
    );
  }
  return data as T;
}

export const api = {
  listLinks: (limit = 100, offset = 0) =>
    request<LinksPage>('GET', `/api/v1/links?limit=${limit}&offset=${offset}`),
  createLink: (input: CreateLinkInput) => request<Link>('POST', '/api/v1/links', input),
  getLink: (id: string) => request<Link>('GET', `/api/v1/links/${id}`),
  updateLink: (id: string, input: UpdateLinkInput) =>
    request<Link>('PATCH', `/api/v1/links/${id}`, input),
  deleteLink: (id: string) => request<void>('DELETE', `/api/v1/links/${id}`),

  linkStats: (id: string, bucket: 'hour' | 'day', from?: string, to?: string) => {
    const q = new URLSearchParams({ bucket });
    if (from) q.set('from', from);
    if (to) q.set('to', to);
    return request<LinkStats>('GET', `/api/v1/links/${id}/stats?${q.toString()}`);
  },
  overview: () => request<Overview>('GET', '/api/v1/stats/overview'),

  listWebhooks: () => request<WebhooksPage>('GET', '/api/v1/webhooks'),
  createWebhook: (input: CreateWebhookInput) =>
    request<Webhook>('POST', '/api/v1/webhooks', input),
  deleteWebhook: (id: string) => request<void>('DELETE', `/api/v1/webhooks/${id}`),
};
