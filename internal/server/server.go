package server

import (
	"github.com/labstack/echo/v4"
	"github.com/nietaki/epstein-file-review/internal/env"
)

func Start(cfg env.Config) {
	e := echo.New()

	e.GET("/", func(c echo.Context) error {
		return c.String(200, "Hello, World!")
	})

	// concatenate the dot and the port
	portSpec := ":" + cfg.ServerPort
	e.Logger.Fatal(e.Start(portSpec))
}
