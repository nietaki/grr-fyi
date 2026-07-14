package server

import (
	"context"
	"html/template"
	"io"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/stats"
	"github.com/nietaki/grr-fyi/internal/util"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// TODO: funcMap
type Template struct {
	templates *template.Template
}

func (t *Template) Render(c *echo.Context, w io.Writer, name string, data any) error {
	tmpl := template.Must(t.templates.Clone())
	tmpl = template.Must(tmpl.ParseFiles("views/" + name))
	return tmpl.ExecuteTemplate(w, "base.html", data)
	// return t.templates.ExecuteTemplate(w, name, data)
}

func indexValues(ctx context.Context) map[string]any {
	p := message.NewPrinter(language.English)
	memoryUsage, err := stats.MemoryUsage()
	if err != nil {
		memoryUsage = uint64(0)
	}

	return map[string]any{
		"fileCount":   p.Sprintf("%d", -1),
		"memoryUsage": util.HumanizeMemory(memoryUsage),
	}
}

func Start(ctx context.Context, cfg env.Config) {
	// config := echo.Config{
	// 	Filesystem: os.DirFS("/"),
	// }
	// e := echo.NewWithConfig(config)
	t := &Template{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e := echo.New()
	e.Renderer = t

	// e.Use(NoContentRanges)
	// e.Use(CacheHeader)
	// e.Pre(middleware.NonWWWRedirect())
	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	e.Static("/", "static")

	e.GET("/", func(c *echo.Context) error {
		// hello world
		return c.Render(200, "index.html", indexValues(c.Request().Context()))
	})

	// concatenate the dot and the port
	portSpec := ":" + cfg.ServerPort
	// Start server
	sc := echo.StartConfig{Address: portSpec}
	// e.Logger.Error(e.Start(portSpec))
	if err := sc.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
