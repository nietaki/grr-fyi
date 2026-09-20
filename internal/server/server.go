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
	"github.com/nietaki/grr-fyi/internal/captcha"
	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/format"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/site"
	"github.com/nietaki/grr-fyi/internal/stats"
)

// TODO: funcMap
type Template struct {
	templates *template.Template
	viewsPath string
}

func templateFuncs(siteGetter func(string) string) template.FuncMap {
	return template.FuncMap{
		"site":                 siteGetter,
		"formatBytes":          format.Bytes,
		"formatInt":            format.Count,
		"formatFloat":          format.Number,
		"formatUptime":         format.Uptime,
		"parseDurationSeconds": format.SecondsToDuration,
	}
}

func NewTemplate(siteConf site.SiteConfig) *Template {
	funcs := templateFuncs(siteConf.Get)
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
	funcs := templateFuncs(func(s string) string { return "test" })
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

func Start(ctx context.Context, cfg env.Config, linkSvc *link.Service, clickSvc *click.Service, statsSvc *stats.Service) {
	e := echo.New()

	siteConf := site.Read(cfg)
	e.Renderer = NewTemplate(siteConf)

	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	e.Use(middleware.Static("static"))

	captchaVerifier := captcha.New(
		cfg.AltchaSecret,
		cfg.AltchaCost,
		time.Duration(cfg.AltchaExpiryMin)*time.Minute,
	)

	handler := NewHandler(linkSvc, clickSvc, statsSvc, siteConf.Get("url"), captchaVerifier)

	api := e.Group("/_/api")
	api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	api.GET("/altcha/challenge", handler.AltchaChallenge)
	api.POST("/create_link", handler.CreateLink)
	api.POST("/slug_availability", handler.SlugAvailability)

	e.GET("/_/edit_link/:slug", handler.EditLink)
	e.POST("/_/edit_link/:slug", handler.EditLink)

	e.GET("/_/stats", handler.Stats)

	e.GET("/:slug", handler.Redirect)

	e.GET("/", func(c *echo.Context) error {
		return c.Render(200, "index.html", map[string]any{
			"captchaEnabled": captchaVerifier.Enabled(),
		})
	})

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
