package shortener

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var customCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,64}$`)

func GenerateCode(length int) (string, error) {
	if length < 3 || length > 64 {
		return "", fmt.Errorf("code length must be between 3 and 64")
	}

	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(alphabet)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}

	return builder.String(), nil
}

func GenerateRequestID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func ValidateCustomCode(code string) error {
	if code == "" {
		return nil
	}
	if !customCodePattern.MatchString(code) {
		return errors.New("custom code must be 3-64 characters and contain only letters, numbers, underscores, or hyphens")
	}
	return nil
}

func ValidateTargetURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("target_url must be a valid absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("target_url must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("target_url must include a host")
	}
	if isPrivateTargetHost(parsed.Hostname()) {
		return errors.New("target_url cannot point to localhost or private network addresses")
	}
	return nil
}

func isPrivateTargetHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

func NormalizeDomain(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return ""
	}
	return host
}

func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func HashIP(ip string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ip)))
	return hex.EncodeToString(sum[:])
}

func CountryFromHeaders(h http.Header) string {
	for _, key := range []string{"CF-IPCountry", "X-Geo-Country", "X-Country"} {
		if value := strings.TrimSpace(h.Get(key)); value != "" {
			return strings.ToUpper(value)
		}
	}
	return "Unknown"
}

func DeviceFromUserAgent(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider"):
		return "Bot"
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		return "Tablet"
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "android"):
		return "Mobile"
	case ua == "":
		return "Unknown"
	default:
		return "Desktop"
	}
}

func ReferrerDomain(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "Direct"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "Unknown"
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" {
		return "Unknown"
	}
	return host
}
