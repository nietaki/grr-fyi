package server

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"runtime"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/site"
)

// TODO: funcMap
type Template struct {
	templates *template.Template
	viewsPath string
}

func NewTemplate(siteConf site.SiteConfig) *Template {
	funcs := template.FuncMap{
		"site": func(s string) string { return siteConf.Get(s) },
	}
	tpl, err := template.New("").Funcs(funcs).ParseGlob("templates/*.html")
	if err != nil {
		panic(err)
	}
	return &Template{
		templates: tpl,
		viewsPath: "views",
	}
}

func (t *Template) Render(c *echo.Context, w io.Writer, name string, data any) error {
	tmpl := template.Must(t.templates.Clone())
	tmpl = template.Must(tmpl.ParseFiles(filepath.Join(t.viewsPath, name)))
	return tmpl.ExecuteTemplate(w, "base.html", data)
}

func NewTemplateForTest() *Template {
	funcs := template.FuncMap{
		"site": func(s string) string { return "test" },
	}
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	templatesPath := filepath.Join(projectRoot, "templates", "*.html")
	tpl, err := template.New("").Funcs(funcs).ParseGlob(templatesPath)
	if err != nil {
		panic(err)
	}
	return &Template{
		templates: tpl,
		viewsPath: filepath.Join(projectRoot, "views"),
	}
}

func Start(ctx context.Context, cfg env.Config, linkSvc *link.Service, clickSvc *click.Service) {
	e := echo.New()

	// Read site config once and share between template and handler
	siteConf := site.Read(cfg)
	e.Renderer = NewTemplate(siteConf)

	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	e.Use(middleware.Static("static"))

	// Create handler with dependencies
	handler := NewHandler(linkSvc, clickSvc, siteConf.Get("url"))

	// API routes with CORS
	api := e.Group("/_/api")
	api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	api.POST("/create_link", handler.CreateLink)
	api.POST("/slug_availability", handler.SlugAvailability)

	e.GET("/_/edit_link/:slug", handler.EditLink)
	e.POST("/_/edit_link/:slug", handler.EditLink)

	// Redirect route - must be last to avoid catching other routes
	e.GET("/:slug", handler.Redirect)

	e.GET("/", func(c *echo.Context) error {
		return c.Render(200, "index.html", map[string]any{})
	})

	// Enable pprof endpoints for performance profiling when PPROF_ENABLED=true
	if cfg.PprofEnabled {
		registerPprof(e)
	}

	portSpec := ":" + cfg.ServerPort
	sc := echo.StartConfig{Address: portSpec}
	if err := sc.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}

func registerPprof(e *echo.Echo) {
	e.GET("/debug/pprof", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	e.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	e.GET("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	e.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	e.GET("/debug/pprof/allocs", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/block", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/goroutine", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/heap", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/mutex", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	e.GET("/debug/pprof/threadcreate", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
}
