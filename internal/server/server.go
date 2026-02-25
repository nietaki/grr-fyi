package server

import (
	"context"
	"fmt"
	"path"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/filedb"
)

func CacheHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return next(c)
	}
}

func Start(cfg env.Config) {
	e := echo.New()
	e.Use(CacheHeader)
	e.Use(middleware.RequestLogger())

	e.GET("/randomfilename", func(c *echo.Context) error {
		// return c.String(200, "Hello, World!")
		filename := filedb.GetRandomFilename()
		return c.String(200, filename)
	})

	e.GET("/", func(c *echo.Context) error {
		filename := filedb.GetRandomFilename()

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
