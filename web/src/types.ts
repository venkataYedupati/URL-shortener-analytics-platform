export type Link = {
  id: string;
  code: string;
  target_url: string;
  title: string;
  custom_domain: string;
  created_at: string;
  expires_at?: string;
  is_active: boolean;
  total_clicks: number;
  last_clicked_at?: string;
  short_url: string;
};

export type DimensionCount = {
  name: string;
  clicks: number;
};

export type TimeBucket = {
  bucket_start: string;
  clicks: number;
};

export type ClickEvent = {
  link_code: string;
  occurred_at: string;
  country: string;
  device: string;
  referrer_domain: string;
  user_agent: string;
  ip_hash: string;
  request_id: string;
};

export type Analytics = {
  code: string;
  total_clicks: number;
  last_clicked_at?: string;
  since: string;
  hourly: TimeBucket[];
  countries: DimensionCount[];
  devices: DimensionCount[];
  referrers: DimensionCount[];
  recent_events: ClickEvent[];
};
