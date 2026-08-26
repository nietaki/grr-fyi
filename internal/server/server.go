package server

import (
	"context"
	"html/template"
	"io"
	"os"
	"path/filepath"
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

// staticFileMiddleware checks if the requested path corresponds to an existing
// file in the static directory. If yes, it serves the file. If no, it continues
// to the next handler (normal routing).
func staticFileMiddleware(staticDir string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			path := c.Request().URL.Path

			// Construct the full file path
			filePath := filepath.Join(staticDir, path)

			// Check if the file exists
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				// File doesn't exist or is a directory, continue to next handler
				return next(c)
			}

			// File exists, serve it
			return c.File(filePath)
		}
	}
}

func Start(ctx context.Context, cfg env.Config, linkSvc *link.Service, clickSvc *click.Service) {
	e := echo.New()

	// Read site config once and share between template and handler
	siteConf := site.Read(cfg)
	e.Renderer = NewTemplate(siteConf)

	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	// Serve static files before routing - checks if file exists in static/
	e.Use(staticFileMiddleware("static"))

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
