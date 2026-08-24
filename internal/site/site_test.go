package site

import (
	"path/filepath"
	"runtime"

	"github.com/mvrahden/go-test/pkg/gotest"

	"github.com/nietaki/grr-fyi/internal/env"
)

type SiteTestSuite struct {
	projectRoot string
}

func (s *SiteTestSuite) BeforeEach(t *gotest.T) {
	_, filename, _, _ := runtime.Caller(0)
	s.projectRoot = filepath.Join(filepath.Dir(filename), "../..")
}

func (s *SiteTestSuite) TestRead(t *gotest.T) {
	t.It("loads site config from yaml file", func(it *gotest.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/site.yml")}
		siteConf := Read(cfg)

		gotest.Equal(it, "Test Site", siteConf.Get("title"))
		gotest.Equal(it, "Test Author", siteConf.Get("author"))
	})
}

func (s *SiteTestSuite) TestReadPanicsOnMissingFile(t *gotest.T) {
	t.It("panics when config file does not exist", func(it *gotest.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/nonexistent.yml")}
		gotest.Panics(it, func() {
			Read(cfg)
		})
	})
}

func (s *SiteTestSuite) TestGetReturnsEmptyForMissingKey(t *gotest.T) {
	t.It("returns empty string for missing key", func(it *gotest.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/site.yml")}
		siteConf := Read(cfg)

		gotest.Equal(it, "", siteConf.Get("nonexistent"))
	})
}
