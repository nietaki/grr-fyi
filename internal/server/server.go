package server

import (
	"context"
	"html/template"
	"io"
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
	}
}

func (t *Template) Render(c *echo.Context, w io.Writer, name string, data any) error {
	tmpl := template.Must(t.templates.Clone())
	tmpl = template.Must(tmpl.ParseFiles("views/" + name))
	return tmpl.ExecuteTemplate(w, "base.html", data)
}

func Start(ctx context.Context, cfg env.Config, linkSvc *link.Service, clickSvc *click.Service) {
	e := echo.New()

	// Read site config once and share between template and handler
	siteConf := site.Read(cfg)
	e.Renderer = NewTemplate(siteConf)

	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	e.Static("/", "static")

	// Create handler with dependencies
	handler := NewHandler(linkSvc, clickSvc, siteConf.Get("url"))

	// API routes with CORS
	api := e.Group("/_")
	api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	api.POST("/create_link", handler.CreateLink)
	api.POST("/slug_availability", handler.SlugAvailability)

	// Redirect route - must be last to avoid catching other routes
	e.GET("/:slug", handler.Redirect)

	e.GET("/", func(c *echo.Context) error {
		return c.Render(200, "index.html", map[string]any{})
	})

	portSpec := ":" + cfg.ServerPort
	sc := echo.StartConfig{Address: portSpec}
	if err := sc.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
