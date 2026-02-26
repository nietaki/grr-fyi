package filedb

import (
	"path"
	"strings"

	lo "github.com/samber/lo"
)

var allFilenames []string
var filenamesByType map[string][]string

func FileType(filename string) string {
	ext := path.Ext(filename)
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif":
		return "image"
	default:
		return "other"
	}
}

func getRandom(filenames []string) string {
	ret := lo.Sample(filenames)

	if ret == "" {
		return "not_found.png"
	}
	return ret
}

func StoreFilenames(f []string) {
	allFilenames = f
}

func GetRandomFilename() string {
	return getRandom(allFilenames)
}
