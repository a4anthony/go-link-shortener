// Types mirroring the go-link-shortener JSON API.

export interface Link {
  id: string;
  code: string;
  short_url: string;
  target_url: string;
  redirect_type: number; // 301 | 302
  expires_at?: string | null;
  max_clicks?: number | null;
  click_count: number;
  created_at: string;
  updated_at: string;
}

export interface LinksPage {
  links: Link[] | null;
  count: number;
}

export interface CreateLinkInput {
  url: string;
  custom_alias?: string;
  redirect_type?: number;
  expires_at?: string | null;
  max_clicks?: number | null;
}

export interface UpdateLinkInput {
  url?: string;
  redirect_type?: number;
  expires_at?: string | null;
  max_clicks?: number | null;
}

export interface TimePoint {
  time: string;
  count: number;
}

export interface LabelCount {
  label: string;
  count: number;
}

export interface LinkStats {
  link_id: string;
  from: string;
  to: string;
  bucket: 'hour' | 'day';
  total_clicks: number;
  series: TimePoint[] | null;
  top_referrers: LabelCount[] | null;
  top_countries: LabelCount[] | null;
  devices: LabelCount[] | null;
}

export interface Overview {
  total_links: number;
  total_clicks: number;
  top_links: LabelCount[] | null;
}

export type WebhookEvent = 'link.created' | 'link.clicked';

export interface Webhook {
  id: string;
  url: string;
  events: WebhookEvent[];
  active: boolean;
  created_at: string;
  secret?: string; // present only in the create response
}

export interface WebhooksPage {
  webhooks: Webhook[] | null;
  count: number;
}

export interface CreateWebhookInput {
  url: string;
  events: WebhookEvent[];
}

export interface ApiErrorBody {
  error: { code: string; message: string };
}
