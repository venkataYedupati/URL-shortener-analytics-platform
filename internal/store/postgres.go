package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/model"
)

const migrationLockID int64 = 9283746501

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, cfg config.Config) (*Store, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) ApplyMigrationFile(ctx context.Context, path string) error {
	sql, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	_, err = conn.Exec(ctx, string(sql))
	if err != nil {
		return fmt.Errorf("apply migration %s: %w", path, err)
	}
	return nil
}

func (s *Store) CreateLink(ctx context.Context, link model.Link) (model.Link, error) {
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}
	if link.Title == "" {
		link.Title = link.Code
	}
	link.IsActive = true

	row := s.pool.QueryRow(ctx, `
		INSERT INTO links (code, target_url, title, custom_domain, created_at, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		RETURNING id::text, code, target_url, title, custom_domain, created_at, expires_at, is_active, total_clicks, last_clicked_at
	`, link.Code, link.TargetURL, link.Title, link.CustomDomain, link.CreatedAt, link.ExpiresAt)

	created, err := scanLink(row)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Link{}, ErrConflict
		}
		return model.Link{}, err
	}
	return created, nil
}

func (s *Store) GetLink(ctx context.Context, code string) (model.Link, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, code, target_url, title, custom_domain, created_at, expires_at, is_active, total_clicks, last_clicked_at
		FROM links
		WHERE code = $1
	`, code)
	return scanLink(row)
}

func (s *Store) FindActiveLink(ctx context.Context, code, host string) (model.Link, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, code, target_url, title, custom_domain, created_at, expires_at, is_active, total_clicks, last_clicked_at
		FROM links
		WHERE code = $1
		  AND is_active = true
		  AND (expires_at IS NULL OR expires_at > now())
		  AND (custom_domain = '' OR custom_domain = $2 OR $2 = '')
		ORDER BY CASE WHEN custom_domain = $2 AND $2 <> '' THEN 0 ELSE 1 END
		LIMIT 1
	`, code, host)
	return scanLink(row)
}

func (s *Store) ListLinks(ctx context.Context, limit int) ([]model.Link, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, code, target_url, title, custom_domain, created_at, expires_at, is_active, total_clicks, last_clicked_at
		FROM links
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []model.Link
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) RecordClick(ctx context.Context, event model.ClickEvent) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var eventID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO click_events (link_code, occurred_at, country, device, referrer_domain, user_agent, ip_hash, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (request_id) WHERE request_id <> '' DO NOTHING
		RETURNING id
	`, event.LinkCode, event.OccurredAt, event.Country, event.Device, event.ReferrerDomain, event.UserAgent, event.IPHash, event.RequestID).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE links
		SET total_clicks = total_clicks + 1, last_clicked_at = $2
		WHERE code = $1
	`, event.LinkCode, event.OccurredAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO analytics_hourly (link_code, bucket_start, country, device, referrer_domain, clicks)
		VALUES ($1, date_trunc('hour', $2::timestamptz), $3, $4, $5, 1)
		ON CONFLICT (link_code, bucket_start, country, device, referrer_domain)
		DO UPDATE SET clicks = analytics_hourly.clicks + 1
	`, event.LinkCode, event.OccurredAt, event.Country, event.Device, event.ReferrerDomain)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err == nil {
		committed = true
	}
	return err
}

func (s *Store) Analytics(ctx context.Context, code string, hours int) (model.Analytics, error) {
	link, err := s.GetLink(ctx, code)
	if err != nil {
		return model.Analytics{}, err
	}
	if hours <= 0 || hours > 24*90 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	hourly, err := s.hourly(ctx, code, since)
	if err != nil {
		return model.Analytics{}, err
	}
	countries, err := s.dimension(ctx, code, since, "country")
	if err != nil {
		return model.Analytics{}, err
	}
	devices, err := s.dimension(ctx, code, since, "device")
	if err != nil {
		return model.Analytics{}, err
	}
	referrers, err := s.dimension(ctx, code, since, "referrer_domain")
	if err != nil {
		return model.Analytics{}, err
	}
	recent, err := s.recentEvents(ctx, code)
	if err != nil {
		return model.Analytics{}, err
	}

	return model.Analytics{
		Code:          link.Code,
		TotalClicks:   link.TotalClicks,
		LastClickedAt: link.LastClickedAt,
		Since:         since,
		Hourly:        hourly,
		Countries:     countries,
		Devices:       devices,
		Referrers:     referrers,
		RecentEvents:  recent,
	}, nil
}

func (s *Store) hourly(ctx context.Context, code string, since time.Time) ([]model.TimeBucket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bucket_start, SUM(clicks)::bigint
		FROM analytics_hourly
		WHERE link_code = $1 AND bucket_start >= $2
		GROUP BY bucket_start
		ORDER BY bucket_start ASC
	`, code, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []model.TimeBucket
	for rows.Next() {
		var bucket model.TimeBucket
		if err := rows.Scan(&bucket.BucketStart, &bucket.Clicks); err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}
	if buckets == nil {
		buckets = []model.TimeBucket{}
	}
	return buckets, rows.Err()
}

func (s *Store) dimension(ctx context.Context, code string, since time.Time, column string) ([]model.DimensionCount, error) {
	query := fmt.Sprintf(`
		SELECT %s, SUM(clicks)::bigint
		FROM analytics_hourly
		WHERE link_code = $1 AND bucket_start >= $2
		GROUP BY %s
		ORDER BY SUM(clicks) DESC
		LIMIT 10
	`, column, column)

	rows, err := s.pool.Query(ctx, query, code, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []model.DimensionCount
	for rows.Next() {
		var value model.DimensionCount
		if err := rows.Scan(&value.Name, &value.Clicks); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if values == nil {
		values = []model.DimensionCount{}
	}
	return values, rows.Err()
}

func (s *Store) recentEvents(ctx context.Context, code string) ([]model.ClickEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT link_code, occurred_at, country, device, referrer_domain, user_agent, ip_hash, request_id
		FROM click_events
		WHERE link_code = $1
		ORDER BY occurred_at DESC
		LIMIT 25
	`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.ClickEvent
	for rows.Next() {
		var event model.ClickEvent
		if err := rows.Scan(&event.LinkCode, &event.OccurredAt, &event.Country, &event.Device, &event.ReferrerDomain, &event.UserAgent, &event.IPHash, &event.RequestID); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if events == nil {
		events = []model.ClickEvent{}
	}
	return events, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLink(row rowScanner) (model.Link, error) {
	var link model.Link
	var expiresAt pgtype.Timestamptz
	var lastClickedAt pgtype.Timestamptz

	err := row.Scan(
		&link.ID,
		&link.Code,
		&link.TargetURL,
		&link.Title,
		&link.CustomDomain,
		&link.CreatedAt,
		&expiresAt,
		&link.IsActive,
		&link.TotalClicks,
		&lastClickedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Link{}, ErrNotFound
	}
	if err != nil {
		return model.Link{}, err
	}

	if expiresAt.Valid {
		t := expiresAt.Time
		link.ExpiresAt = &t
	}
	if lastClickedAt.Valid {
		t := lastClickedAt.Time
		link.LastClickedAt = &t
	}
	return link, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
