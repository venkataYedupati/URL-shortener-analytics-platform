package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/cache"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/events"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/model"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/shortener"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/store"
)

const maxCreateLinkBodyBytes = 32 << 10

type Server struct {
	cfg       config.Config
	store     *store.Store
	cache     *cache.RedisCache
	publisher events.Publisher
	log       *slog.Logger
	mux       *http.ServeMux
}

type createLinkRequest struct {
	TargetURL    string     `json:"target_url"`
	Title        string     `json:"title"`
	CustomCode   string     `json:"custom_code"`
	CustomDomain string     `json:"custom_domain"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

type linkResponse struct {
	model.Link
	ShortURL string `json:"short_url"`
}

func New(cfg config.Config, store *store.Store, cache *cache.RedisCache, publisher events.Publisher, log *slog.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		store:     store,
		cache:     cache,
		publisher: publisher,
		log:       log,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.cors(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.health)
	s.mux.HandleFunc("/v1/overview", s.overview)
	s.mux.HandleFunc("/v1/links", s.links)
	s.mux.HandleFunc("/v1/links/", s.linkItem)
	s.mux.HandleFunc("/", s.redirect)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := map[string]string{"status": "ok"}
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "postgres": err.Error()})
		return
	}
	if err := s.cache.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "redis": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) links(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	if allowed, err := s.allowWrite(r); err != nil {
		s.log.Warn("rate limiter failed open", "error", err)
	} else if !allowed {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	var req createLinkRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreateLinkBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain exactly one JSON object")
		return
	}

	req.TargetURL = strings.TrimSpace(req.TargetURL)
	req.Title = strings.TrimSpace(req.Title)
	req.CustomCode = strings.TrimSpace(req.CustomCode)
	req.CustomDomain = shortener.NormalizeDomain(req.CustomDomain)

	if err := shortener.ValidateTargetURL(req.TargetURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := shortener.ValidateCustomCode(req.CustomCode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "expires_at must be in the future")
		return
	}

	link, err := s.createLink(r.Context(), req)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "short code is already in use")
			return
		}
		s.log.Error("create link failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create link")
		return
	}

	if err := s.cache.SetLink(r.Context(), link); err != nil {
		s.log.Warn("failed to cache new link", "code", link.Code, "error", err)
	}
	writeJSON(w, http.StatusCreated, s.withShortURL(link))
}

func (s *Server) linkItem(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/links/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	code := parts[0]

	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		link, err := s.store.GetLink(r.Context(), code)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.withShortURL(link))
	case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "analytics":
		hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
		analytics, err := s.store.Analytics(r.Context(), code, hours)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, analytics)
	case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "qr":
		png, err := qrcode.Encode(s.shortURL(code, ""), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate QR code")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	links, err := s.store.ListLinks(r.Context(), limit)
	if err != nil {
		s.log.Error("list links failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}

	responses := make([]linkResponse, 0, len(links))
	for _, link := range links {
		responses = append(responses, s.withShortURL(link))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": responses})
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	code := strings.Trim(r.URL.Path, "/")
	if code == "" || strings.Contains(code, "/") || strings.HasPrefix(code, "v1") {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	host := shortener.NormalizeDomain(r.Host)
	link, err := s.cache.GetLink(r.Context(), code, host)
	if err != nil {
		link, err = s.store.FindActiveLink(r.Context(), code, host)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if cacheErr := s.cache.SetLink(r.Context(), link); cacheErr != nil {
			s.log.Warn("failed to cache link", "code", link.Code, "error", cacheErr)
		}
	}

	s.publishClick(r, link.Code)
	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

func (s *Server) createLink(ctx context.Context, req createLinkRequest) (model.Link, error) {
	if req.CustomCode != "" {
		return s.store.CreateLink(ctx, model.Link{
			Code:         req.CustomCode,
			TargetURL:    req.TargetURL,
			Title:        titleOrDefault(req.Title, req.CustomCode),
			CustomDomain: req.CustomDomain,
			ExpiresAt:    req.ExpiresAt,
		})
	}

	var lastErr error
	for i := 0; i < 8; i++ {
		code, err := shortener.GenerateCode(s.cfg.ShortCodeLength)
		if err != nil {
			return model.Link{}, err
		}
		link, err := s.store.CreateLink(ctx, model.Link{
			Code:         code,
			TargetURL:    req.TargetURL,
			Title:        titleOrDefault(req.Title, code),
			CustomDomain: req.CustomDomain,
			ExpiresAt:    req.ExpiresAt,
		})
		if err == nil {
			return link, nil
		}
		lastErr = err
		if !errors.Is(err, store.ErrConflict) {
			return model.Link{}, err
		}
	}
	return model.Link{}, lastErr
}

func (s *Server) publishClick(r *http.Request, code string) {
	event := model.ClickEvent{
		LinkCode:       code,
		OccurredAt:     time.Now().UTC(),
		Country:        shortener.CountryFromHeaders(r.Header),
		Device:         shortener.DeviceFromUserAgent(r.UserAgent()),
		ReferrerDomain: shortener.ReferrerDomain(r.Referer()),
		UserAgent:      r.UserAgent(),
		IPHash:         shortener.HashIP(shortener.ClientIP(r)),
		RequestID:      shortener.GenerateRequestID(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.publisher.PublishClick(ctx, event); err != nil {
			s.log.Warn("failed to publish click event", "link_code", event.LinkCode, "error", err)
		}
	}()
}

func (s *Server) allowWrite(r *http.Request) (bool, error) {
	key := "write:" + shortener.HashIP(shortener.ClientIP(r))
	return s.cache.Allow(r.Context(), key, s.cfg.RateLimitRequests, s.cfg.RateLimitWindow)
}

func (s *Server) withShortURL(link model.Link) linkResponse {
	return linkResponse{Link: link, ShortURL: s.shortURL(link.Code, link.CustomDomain)}
}

func (s *Server) shortURL(code, domain string) string {
	if domain != "" {
		return "https://" + domain + "/" + code
	}
	return s.cfg.PublicBaseURL + "/" + code
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (s.cfg.WebOrigin == "*" || origin == s.cfg.WebOrigin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func titleOrDefault(title, fallback string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fallback
	}
	return title
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	writeError(w, http.StatusInternalServerError, fmt.Sprintf("storage error: %s", err.Error()))
}
