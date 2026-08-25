package site

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/nietaki/grr-fyi/internal/env"
)

type SiteTestSuite struct {
	suite.Suite
	projectRoot string
}

func (s *SiteTestSuite) SetupTest() {
	_, filename, _, _ := runtime.Caller(0)
	s.projectRoot = filepath.Join(filepath.Dir(filename), "../..")
}

func (s *SiteTestSuite) TestRead() {
	s.T().Run("loads site config from yaml file", func(t *testing.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/site.yml")}
		siteConf := Read(cfg)

		require.Equal(t, "Test Site", siteConf.Get("title"))
		require.Equal(t, "Test Author", siteConf.Get("author"))
	})
}

func (s *SiteTestSuite) TestReadPanicsOnMissingFile() {
	s.T().Run("panics when config file does not exist", func(t *testing.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/nonexistent.yml")}
		assert.Panics(t, func() {
			Read(cfg)
		})
	})
}

func (s *SiteTestSuite) TestGetReturnsEmptyForMissingKey() {
	s.T().Run("returns empty string for missing key", func(t *testing.T) {
		cfg := env.Config{SiteFilePath: filepath.Join(s.projectRoot, "test_support/site.yml")}
		siteConf := Read(cfg)

		require.Equal(t, "", siteConf.Get("nonexistent"))
	})
}

func TestSiteTestSuite(t *testing.T) {
	suite.Run(t, new(SiteTestSuite))
}
