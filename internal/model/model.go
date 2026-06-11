package model

import "time"

type Link struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	TargetURL     string     `json:"target_url"`
	Title         string     `json:"title"`
	CustomDomain  string     `json:"custom_domain"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsActive      bool       `json:"is_active"`
	TotalClicks   int64      `json:"total_clicks"`
	LastClickedAt *time.Time `json:"last_clicked_at,omitempty"`
}

type ClickEvent struct {
	LinkCode       string    `json:"link_code"`
	OccurredAt     time.Time `json:"occurred_at"`
	Country        string    `json:"country"`
	Device         string    `json:"device"`
	ReferrerDomain string    `json:"referrer_domain"`
	UserAgent      string    `json:"user_agent"`
	IPHash         string    `json:"ip_hash"`
	RequestID      string    `json:"request_id"`
}

type DimensionCount struct {
	Name   string `json:"name"`
	Clicks int64  `json:"clicks"`
}

type TimeBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	Clicks      int64     `json:"clicks"`
}

type Analytics struct {
	Code          string           `json:"code"`
	TotalClicks   int64            `json:"total_clicks"`
	LastClickedAt *time.Time       `json:"last_clicked_at,omitempty"`
	Since         time.Time        `json:"since"`
	Hourly        []TimeBucket     `json:"hourly"`
	Countries     []DimensionCount `json:"countries"`
	Devices       []DimensionCount `json:"devices"`
	Referrers     []DimensionCount `json:"referrers"`
	RecentEvents  []ClickEvent     `json:"recent_events"`
}
