import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  BarChart3,
  Check,
  Copy,
  ExternalLink,
  Link2,
  Plus,
  QrCode,
  RefreshCw,
  Search
} from "lucide-react";
import { api } from "./api";
import type { Analytics, DimensionCount, Link, TimeBucket } from "./types";

type FormState = {
  target_url: string;
  title: string;
  custom_code: string;
  custom_domain: string;
  expires_at: string;
};

const initialForm: FormState = {
  target_url: "",
  title: "",
  custom_code: "",
  custom_domain: "",
  expires_at: ""
};

function App() {
  const [links, setLinks] = useState<Link[]>([]);
  const [selectedCode, setSelectedCode] = useState<string>("");
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [form, setForm] = useState<FormState>(initialForm);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");

  const selected = useMemo(
    () => links.find((link) => link.code === selectedCode) ?? links[0],
    [links, selectedCode]
  );

  const filteredLinks = useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) return links;
    return links.filter((link) =>
      [link.title, link.code, link.target_url, link.custom_domain].some((value) =>
        value.toLowerCase().includes(term)
      )
    );
  }, [links, query]);

  useEffect(() => {
    void loadLinks();
  }, []);

  useEffect(() => {
    if (selected?.code) {
      setSelectedCode(selected.code);
      void loadAnalytics(selected.code);
    } else {
      setAnalytics(null);
    }
  }, [selected?.code]);

  async function loadLinks() {
    setLoading(true);
    setError("");
    try {
      const nextLinks = await api.listLinks();
      setLinks(nextLinks);
      if (!selectedCode && nextLinks.length > 0) {
        setSelectedCode(nextLinks[0].code);
      }
    } catch (err) {
      setError(readError(err));
    } finally {
      setLoading(false);
    }
  }

  async function loadAnalytics(code: string) {
    setRefreshing(true);
    setError("");
    try {
      setAnalytics(await api.getAnalytics(code));
    } catch (err) {
      setError(readError(err));
      setAnalytics(null);
    } finally {
      setRefreshing(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const payload = {
        target_url: form.target_url.trim(),
        title: form.title.trim() || undefined,
        custom_code: form.custom_code.trim() || undefined,
        custom_domain: form.custom_domain.trim() || undefined,
        expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : undefined
      };
      const created = await api.createLink(payload);
      setLinks((current) => [created, ...current.filter((link) => link.code !== created.code)]);
      setSelectedCode(created.code);
      setForm(initialForm);
    } catch (err) {
      setError(readError(err));
    } finally {
      setSaving(false);
    }
  }

  async function copyShortURL(link: Link) {
    await navigator.clipboard.writeText(link.short_url);
    setCopied(link.code);
    window.setTimeout(() => setCopied(""), 1400);
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">URL Shortener</p>
          <h1>Analytics console</h1>
        </div>
        <div className="status-strip">
          <span>{links.length} links</span>
          <span>{formatNumber(totalClicks(links))} clicks</span>
          <button className="icon-button" type="button" onClick={() => void loadLinks()} title="Refresh links">
            <RefreshCw size={18} />
          </button>
        </div>
      </header>

      {error && <div className="error-banner">{error}</div>}

      <section className="workspace">
        <aside className="sidebar">
          <form className="create-panel" onSubmit={(event) => void handleSubmit(event)}>
            <div className="panel-title">
              <Link2 size={18} />
              <h2>Create link</h2>
            </div>
            <label>
              <span>Destination</span>
              <input
                required
                type="url"
                value={form.target_url}
                placeholder="https://example.com/campaign"
                onChange={(event) => setForm({ ...form, target_url: event.target.value })}
              />
            </label>
            <label>
              <span>Title</span>
              <input
                value={form.title}
                placeholder="Launch campaign"
                onChange={(event) => setForm({ ...form, title: event.target.value })}
              />
            </label>
            <div className="form-grid">
              <label>
                <span>Custom code</span>
                <input
                  value={form.custom_code}
                  placeholder="spring-sale"
                  onChange={(event) => setForm({ ...form, custom_code: event.target.value })}
                />
              </label>
              <label>
                <span>Domain</span>
                <input
                  value={form.custom_domain}
                  placeholder="go.example.com"
                  onChange={(event) => setForm({ ...form, custom_domain: event.target.value })}
                />
              </label>
            </div>
            <label>
              <span>Expiration</span>
              <input
                type="datetime-local"
                value={form.expires_at}
                onChange={(event) => setForm({ ...form, expires_at: event.target.value })}
              />
            </label>
            <button className="primary-button" type="submit" disabled={saving}>
              <Plus size={18} />
              {saving ? "Creating" : "Create"}
            </button>
          </form>

          <div className="link-browser">
            <div className="search-row">
              <Search size={16} />
              <input value={query} placeholder="Search links" onChange={(event) => setQuery(event.target.value)} />
            </div>
            <div className="link-list">
              {loading ? (
                <div className="empty-state">Loading links</div>
              ) : filteredLinks.length === 0 ? (
                <div className="empty-state">No links</div>
              ) : (
                filteredLinks.map((link) => (
                  <button
                    type="button"
                    key={link.id}
                    className={`link-row ${selected?.code === link.code ? "active" : ""}`}
                    onClick={() => setSelectedCode(link.code)}
                  >
                    <span className="link-title">{link.title}</span>
                    <span className="link-meta">
                      /{link.code} · {formatNumber(link.total_clicks)} clicks
                    </span>
                  </button>
                ))
              )}
            </div>
          </div>
        </aside>

        <section className="detail-pane">
          {selected ? (
            <>
              <div className="link-header">
                <div className="link-heading">
                  <p className="eyebrow">{selected.custom_domain || api.baseURL.replace(/^https?:\/\//, "")}</p>
                  <h2>{selected.title}</h2>
                  <a href={selected.target_url} target="_blank" rel="noreferrer">
                    {selected.target_url}
                  </a>
                </div>
                <div className="actions">
                  <button className="icon-button" type="button" onClick={() => void copyShortURL(selected)} title="Copy short URL">
                    {copied === selected.code ? <Check size={18} /> : <Copy size={18} />}
                  </button>
                  <a className="icon-button" href={selected.short_url} target="_blank" rel="noreferrer" title="Open short URL">
                    <ExternalLink size={18} />
                  </a>
                  <button
                    className="icon-button"
                    type="button"
                    onClick={() => void loadAnalytics(selected.code)}
                    title="Refresh analytics"
                  >
                    <RefreshCw size={18} className={refreshing ? "spin" : ""} />
                  </button>
                </div>
              </div>

              <div className="metrics-grid">
                <Metric label="Total clicks" value={formatNumber(analytics?.total_clicks ?? selected.total_clicks)} />
                <Metric label="Last click" value={formatDate(analytics?.last_clicked_at ?? selected.last_clicked_at)} />
                <Metric label="Created" value={formatDate(selected.created_at)} />
                <Metric label="Expires" value={formatDate(selected.expires_at)} />
              </div>

              <div className="analytics-grid">
                <section className="panel wide">
                  <div className="panel-title">
                    <BarChart3 size={18} />
                    <h2>Hourly clicks</h2>
                  </div>
                  <HourlyChart buckets={analytics?.hourly ?? []} />
                </section>

                <section className="panel qr-panel">
                  <div className="panel-title">
                    <QrCode size={18} />
                    <h2>QR code</h2>
                  </div>
                  <img src={api.qrURL(selected.code)} alt={`QR code for ${selected.short_url}`} />
                  <code>{selected.short_url}</code>
                </section>

                <Breakdown title="Countries" rows={analytics?.countries ?? []} />
                <Breakdown title="Devices" rows={analytics?.devices ?? []} />
                <Breakdown title="Referrers" rows={analytics?.referrers ?? []} />

                <section className="panel wide">
                  <div className="panel-title">
                    <h2>Recent events</h2>
                  </div>
                  <EventTable analytics={analytics} />
                </section>
              </div>
            </>
          ) : (
            <div className="empty-dashboard">Create a link to start collecting click analytics.</div>
          )}
        </section>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function HourlyChart({ buckets }: { buckets: TimeBucket[] }) {
  if (buckets.length === 0) {
    return <div className="chart-empty">No click events in this range</div>;
  }

  const max = Math.max(...buckets.map((bucket) => bucket.clicks), 1);
  return (
    <div className="hourly-chart">
      {buckets.map((bucket) => (
        <div className="bar-column" key={bucket.bucket_start} title={`${formatDate(bucket.bucket_start)} · ${bucket.clicks} clicks`}>
          <span style={{ height: `${Math.max(8, (bucket.clicks / max) * 100)}%` }} />
        </div>
      ))}
    </div>
  );
}

function Breakdown({ title, rows }: { title: string; rows: DimensionCount[] }) {
  const max = Math.max(...rows.map((row) => row.clicks), 1);
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{title}</h2>
      </div>
      <div className="breakdown-list">
        {rows.length === 0 ? (
          <div className="chart-empty compact">No data</div>
        ) : (
          rows.map((row) => (
            <div className="breakdown-row" key={row.name}>
              <div>
                <span>{row.name}</span>
                <strong>{formatNumber(row.clicks)}</strong>
              </div>
              <progress max={max} value={row.clicks} />
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function EventTable({ analytics }: { analytics: Analytics | null }) {
  const events = analytics?.recent_events ?? [];
  if (events.length === 0) {
    return <div className="chart-empty compact">No recent events</div>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Time</th>
            <th>Country</th>
            <th>Device</th>
            <th>Referrer</th>
            <th>Request</th>
          </tr>
        </thead>
        <tbody>
          {events.map((event) => (
            <tr key={event.request_id}>
              <td>{formatDate(event.occurred_at)}</td>
              <td>{event.country}</td>
              <td>{event.device}</td>
              <td>{event.referrer_domain}</td>
              <td>
                <code>{event.request_id.slice(0, 10)}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function totalClicks(links: Link[]) {
  return links.reduce((sum, link) => sum + link.total_clicks, 0);
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function formatDate(value?: string) {
  if (!value) return "None";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit"
  }).format(new Date(value));
}

function readError(err: unknown) {
  return err instanceof Error ? err.message : "Unexpected error";
}

export default App;
