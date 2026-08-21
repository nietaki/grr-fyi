package server

import (
	"context"
	"html/template"
	"io"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/site"
)

// TODO: funcMap
type Template struct {
	templates *template.Template
}

func NewTemplate() *Template {
	siteConf := site.Read()

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

func Start(ctx context.Context, cfg env.Config) {
	e := echo.New()
	e.Renderer = NewTemplate()

	e.Use(middleware.Recover())
	e.Use(middleware.ContextTimeout(time.Second * 30))

	e.Static("/", "static")

	e.GET("/", func(c *echo.Context) error {
		return c.Render(200, "index.html", map[string]any{})
	})

	portSpec := ":" + cfg.ServerPort
	sc := echo.StartConfig{Address: portSpec}
	if err := sc.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
