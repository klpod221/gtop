package agent

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"gtop/internal/config"
)

// Sender handles authenticated, retry-capable HTTP POST of telemetry payloads.
type Sender struct {
	cfg    config.ServerConfig
	client *http.Client
}

// NewSender creates a Sender configured from cfg.
// It validates TLS settings and constructs the HTTP client.
func NewSender(cfg config.ServerConfig) (*Sender, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify, //nolint:gosec // user-controlled option
	}

	if cfg.TLSCACert != "" {
		caPEM, err := os.ReadFile(cfg.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", cfg.TLSCACert, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA cert from %s", cfg.TLSCACert)
		}
		tlsCfg.RootCAs = pool
	}

	transport := &http.Transport{TLSClientConfig: tlsCfg}
	client := &http.Client{
		Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
		Transport: transport,
	}

	return &Sender{cfg: cfg, client: client}, nil
}

// Send posts jsonPayload to the configured endpoint.
// It retries up to cfg.RetryCount times with exponential backoff on failure.
func (s *Sender) Send(jsonPayload []byte) error {
	body, contentEncoding, err := s.prepareBody(jsonPayload)
	if err != nil {
		return fmt.Errorf("preparing request body: %w", err)
	}

	delay := time.Duration(s.cfg.RetryDelaySeconds) * time.Second
	var lastErr error

	for attempt := 0; attempt <= s.cfg.RetryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay *= 2
		}

		req, err := http.NewRequest(http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		if s.cfg.AuthToken != "" {
			header := s.cfg.AuthHeader
			if header == "" {
				header = "Authorization"
			}
			req.Header.Set(header, s.cfg.AuthToken)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("attempt %d: server returned %s", attempt+1, resp.Status)
	}

	return fmt.Errorf("all %d attempts failed; last error: %w", s.cfg.RetryCount+1, lastErr)
}

// prepareBody optionally gzip-compresses data, returning the body bytes and encoding header value.
func (s *Sender) prepareBody(data []byte) ([]byte, string, error) {
	if !s.cfg.Compress {
		return data, "", nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, "", err
	}
	if err := gz.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "gzip", nil
}
