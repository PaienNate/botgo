package webhook

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sealdice/botgo/token"
)

// CredentialsResolver 根据 appID 查找 webhook 凭据。
type CredentialsResolver func(appID string) (*token.QQBotCredentials, error)

// ServerOptions webhook server 配置。
type ServerOptions struct {
	Addr         string
	Path         string
	TLSConfig    *tls.Config
	Resolve      CredentialsResolver
	Deduper      Deduper
	MaxBodyBytes int64
	AckMode      AckMode
	// SkipVerify 仅用于本地测试。生产环境不要开启。
	SkipVerify        bool
	ErrorHandler      func(error)
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Server 一个可独立运行的 webhook server。
type Server struct {
	opts       ServerOptions
	httpServer *http.Server
	mu         sync.Mutex
}

// NewServer 创建 webhook server。
func NewServer(opts ServerOptions) *Server {
	if opts.Path == "" {
		opts.Path = "/qqbot"
	}
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBodyBytes
	}
	if opts.ReadHeaderTimeout <= 0 {
		opts.ReadHeaderTimeout = 10 * time.Second
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 30 * time.Second
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 30 * time.Second
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 120 * time.Second
	}
	return &Server{opts: opts}
}

// Handler 返回 HTTP handler。
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID := extractAppID(r, s.opts.Path)
		if appID == "" {
			writeError(w, http.StatusBadRequest, "missing appid")
			return
		}
		if s.opts.Resolve == nil {
			writeError(w, http.StatusInternalServerError, "credentials resolver is nil")
			return
		}
		credentials, err := s.opts.Resolve(appID)
		if err != nil || credentials == nil {
			writeError(w, http.StatusUnauthorized, "unknown appid")
			return
		}
		if credentials.AppID == "" {
			credentials.AppID = appID
		}
		if credentials.AppID != appID {
			writeError(w, http.StatusUnauthorized, "appid mismatch")
			return
		}

		NewHandler(HandlerOptions{
			Credentials:  credentials,
			MaxBodyBytes: s.opts.MaxBodyBytes,
			Deduper:      s.opts.Deduper,
			AckMode:      s.opts.AckMode,
			SkipVerify:   s.opts.SkipVerify,
			ErrorHandler: s.opts.ErrorHandler,
		}).ServeHTTP(w, r)
	})
}

// ListenAndServe 启动 HTTP server。
func (s *Server) ListenAndServe() error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.Handler(),
		TLSConfig:         s.opts.TLSConfig,
		ReadHeaderTimeout: s.opts.ReadHeaderTimeout,
		ReadTimeout:       s.opts.ReadTimeout,
		WriteTimeout:      s.opts.WriteTimeout,
		IdleTimeout:       s.opts.IdleTimeout,
	}
	server := s.httpServer
	s.mu.Unlock()

	if s.opts.TLSConfig != nil && len(s.opts.TLSConfig.Certificates) > 0 {
		return server.ListenAndServeTLS("", "")
	}
	return server.ListenAndServe()
}

// ListenAndServeTLS 启动 HTTPS server。
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.Handler(),
		TLSConfig:         s.opts.TLSConfig,
		ReadHeaderTimeout: s.opts.ReadHeaderTimeout,
		ReadTimeout:       s.opts.ReadTimeout,
		WriteTimeout:      s.opts.WriteTimeout,
		IdleTimeout:       s.opts.IdleTimeout,
	}
	server := s.httpServer
	s.mu.Unlock()
	return server.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown 优雅关闭 server。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	server := s.httpServer
	s.mu.Unlock()
	if server == nil {
		return errors.New("webhook server is not running")
	}
	return server.Shutdown(ctx)
}

func extractAppID(r *http.Request, path string) string {
	appID := strings.TrimSpace(r.Header.Get("X-Bot-Appid"))
	if appID != "" {
		return appID
	}
	prefix := strings.TrimSuffix(path, "/")
	if prefix == "" {
		prefix = "/qqbot"
	}
	if !strings.HasPrefix(r.URL.Path, prefix+"/") {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(r.URL.Path, prefix+"/"), "/")
}
