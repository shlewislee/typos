package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/shlewislee/typos/internal/printer"
)

type Template struct {
	Name           string   `toml:"name"`
	Filename       string   `toml:"filename"`
	RequiredFields []string `toml:"required_fields"`
}

type Server struct {
	Host                string
	Templates           map[string]Template
	tempDir             string
	FontPath            string
	MaxJobs             int
	Logger              *slog.Logger
	DefaultImageOptions printer.ImageOptions

	jobs        *Jobs
	printerConn *PrinterConn
	handler     *Handler
}

type Option func(*Server)

func WithMaxJobs(n int) Option {
	return func(s *Server) {
		s.MaxJobs = n
	}
}

func WithFontPath(path string) Option {
	return func(s *Server) {
		if path != "" && !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err == nil {
				path = abs
			}
		}
		s.FontPath = path
	}
}

func WithHost(host string) Option {
	return func(s *Server) {
		s.Host = host
	}
}

func WithPrinter(p *printer.Printer) Option {
	return func(s *Server) {
		s.printerConn = NewPrinterConn(p)
	}
}

func WithTemplates(templates map[string]Template) Option {
	return func(s *Server) {
		for name, t := range templates {
			if !filepath.IsAbs(t.Filename) {
				abs, err := filepath.Abs(t.Filename)
				if err == nil {
					t.Filename = abs
					templates[name] = t
				}
			}
		}
		s.Templates = templates
	}
}

func WithTempDir(tempDir string) Option {
	return func(s *Server) {
		if tempDir == "" {
			return
		}
		s.tempDir = tempDir
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Server) {
		s.Logger = logger
	}
}

func WithDefaultGamma(gamma float64) Option {
	return func(s *Server) {
		s.DefaultImageOptions.Gamma = gamma
	}
}

func WithDefaultDitherMethod(method printer.DitherMethod) Option {
	return func(s *Server) {
		s.DefaultImageOptions.DitherMethod = method
	}
}

func NewServer(opts ...Option) *Server {
	s := &Server{
		Host:    "127.0.0.1:8888",
		tempDir: os.TempDir(),
		MaxJobs: 1000,
		Logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.jobs = NewJobs(s.MaxJobs)
	s.handler = NewHandler(s.Logger, s.tempDir, s.FontPath, s.Templates, s.jobs, s.printerConn)

	return s
}

func (s *Server) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	e := echo.New()
	if s.Logger != nil {
		e.Logger = s.Logger
	}

	s.startWorker(ctx)

	e.Use(middleware.BodyLimit(20 * 1024 * 1024))

	e.Logger.Debug("Registering middleware")
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			e.Logger.Info("request",
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.String("method", v.Method),
			)
			return nil
		},
	}))

	e.Logger.Debug("Registering routes")
	RegisterRoutes(e, s.handler)

	e.Logger.Debug("Starting echo server", "host", s.Host)
	return e.Start(s.Host)
}
