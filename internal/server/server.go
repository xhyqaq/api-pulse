package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// Server HTTP 服务器
type Server struct {
	router  *chi.Mux
	port    int
	logger  *logrus.Logger
	handler *ApiNotifyHandler
	srv     *http.Server
}

// NewServer 创建新的 HTTP 服务器
func NewServer(port int, handler *ApiNotifyHandler, logger *logrus.Logger) *Server {
	r := chi.NewRouter()

	// 添加中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	return &Server{
		router:  r,
		port:    port,
		logger:  logger,
		handler: handler,
	}
}

// SetupRoutes 设置路由
func (s *Server) SetupRoutes() {
	s.router.Get("/health", s.handler.HealthCheck)
	s.router.Post("/webhook", s.handler.HandleWebhook)
}

// Start 启动服务器
func (s *Server) Start() error {
	s.SetupRoutes()

	addr := fmt.Sprintf(":%d", s.port)
	s.logger.WithField("port", s.port).Info("启动 HTTP 服务器")

	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.srv.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("正在关闭 HTTP 服务器")
	return s.srv.Shutdown(ctx)
}
