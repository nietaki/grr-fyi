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
	"github.com/nietaki/epstein-file-review/internal/stats"
	"github.com/nietaki/epstein-file-review/internal/util"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(c *echo.Context, w io.Writer, name string, data any) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func NoContentRanges(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header()["Accept-Ranges"] = nil
		err := next(c)
		return err
	}
}

func CacheHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache, no-store")
		return next(c)
	}
}

func indexValues() map[string]any {
	p := message.NewPrinter(language.English)
	memoryUsage, err := stats.MemoryUsage()
	if err != nil {
		memoryUsage = uint64(0)
	}
	return map[string]any{
		"fileCount":   p.Sprintf("%d", filedb.FileCount()),
		"memoryUsage": util.HumanizeMemory(memoryUsage),
	}
}

func RandomFileHandler(c *echo.Context) error {
	filetypes, err := echo.FormValues[string](c, "filetypes[]")
	if err != nil {
		filetypes = []string{}
		// return c.String(400, fmt.Sprintf("invalid filetype parameter: %v", categories))
	}

	query, err := echo.FormValue[string](c, "query")
	if err != nil {
		query = ""
	}

	query = strings.TrimSpace(query)

	if query != "" {
		filename := filedb.GetRandomFilenameByQuery(query)
		return c.Redirect(303, "/files/"+filename)
	}

	fmt.Printf("filetypes: %v\n", filetypes)
	// filename := filedb.GetRandomFilename()
	filename := filedb.GetRandomFilenameByTypes(filetypes)

	// return ServeFilenameHandler(filename)(c)
	return c.Redirect(303, "/files/"+filename)
}

func ServeFilenameHandler(filename string) echo.HandlerFunc {
	if filename == "" {
		filename = "not_found.png"
	}
	return func(c *echo.Context) error {
		if !(filename == "not_found.png" || strings.HasPrefix(filename, "raw_data/")) {
			return c.String(404, "file not found")
		}
		base := path.Base(filename)
		switch filedb.FileType(filename) {
		case "pdf", "video", "audio", "image":
			return c.Inline(filename, base)
		default:
			return c.Attachment(filename, base)
		}
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

	// e.Use(NoContentRanges)
	// e.Use(CacheHeader)
	// e.Pre(middleware.NonWWWRedirect())
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.Static("/", "static")

	e.GET("/", func(c *echo.Context) error {
		// hello world
		return c.Render(200, "index.html", indexValues())
	})

	e.GET("/randomfile", RandomFileHandler, CacheHeader, NoContentRanges)

	e.POST("/randomfile", RandomFileHandler, CacheHeader, NoContentRanges)

	e.GET("/files/*", func(c *echo.Context) error {
		filename := c.Param("*")
		return ServeFilenameHandler(filename)(c)
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
