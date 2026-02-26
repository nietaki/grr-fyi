package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"path"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/filedb"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(c *echo.Context, w io.Writer, name string, data any) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func CacheHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if strings.Contains(c.Request().URL.Path, "random") {
			c.Response().Header().Set("Cache-Control", "no-store")
		}
		return next(c)
	}
}

func Start(cfg env.Config) {
	// config := echo.Config{
	// 	Filesystem: os.DirFS("/"),
	// }
	// e := echo.NewWithConfig(config)
	t := &Template{
		templates: template.Must(template.ParseGlob("public/views/*.html")),
	}
	e := echo.New()
	e.Renderer = t

	e.Use(CacheHeader)
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.Static("/", "static")

	e.GET("/randomfilename", func(c *echo.Context) error {
		// return c.String(200, "Hello, World!")
		filename := filedb.GetRandomFilename()
		return c.String(200, filename)
	})

	e.GET("/", func(c *echo.Context) error {
		// hello world
		return c.Render(200, "index.html", nil)
	})

	e.GET("/randomfile", func(c *echo.Context) error {
		filename := filedb.GetRandomFilename()

		// filename = strings.TrimPrefix(filename, "/")
		// get basename of the file
		base := path.Base(filename)
		fmt.Printf("Serving file: %q\n", filename)
		return c.Inline(filename, base)
	})

	// concatenate the dot and the port
	portSpec := ":" + cfg.ServerPort
	// Start server
	sc := echo.StartConfig{Address: portSpec}
	// e.Logger.Error(e.Start(portSpec))
	if err := sc.Start(context.Background(), e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
