package filedb

import (
	"path"
	"strings"

	lo "github.com/samber/lo"
)

var allFilenames []string
var filenamesByType map[string][]string

// 2026-02-26 22:48:06.057510+00:001 3gp
// 2026-02-26 22:48:06.057541+00:0011 DAT
// 2026-02-26 22:48:06.057549+00:001 DII
// 2026-02-26 22:48:06.057555+00:001 LFP
// 2026-02-26 22:48:06.057561+00:001 LST
// 2026-02-26 22:48:06.057579+00:0011 OPT
// 2026-02-26 22:48:06.057585+00:005 amr
// 2026-02-26 22:48:06.057592+00:001 avi
// 2026-02-26 22:48:06.057598+00:004 csv
// 2026-02-26 22:48:06.057604+00:0020 m4a
// 2026-02-26 22:48:06.057610+00:0029 m4v
// 2026-02-26 22:48:06.057616+00:001 md
// 2026-02-26 22:48:06.057626+00:00154 mov
// 2026-02-26 22:48:06.057632+00:002 mp3
// 2026-02-26 22:48:06.057638+00:001068 mp4
// 2026-02-26 22:48:06.057644+00:0016 opus
// 2026-02-26 22:48:06.057649+00:00849657 pdf
// 2026-02-26 22:48:06.057655+00:001 pluginpayloadattachment
// 2026-02-26 22:48:06.057664+00:001 txt
// 2026-02-26 22:48:06.057670+00:005 wav
// 2026-02-26 22:48:06.057676+00:002 xls
// 2026-02-26 22:48:06.057681+00:0010 xlsx
// 2026-02-26 22:48:06.057687+00:001 zip

func FileType(filename string) string {
	ext := path.Ext(filename)
	ext = strings.ToLower(ext)
	switch ext {
	case ".pdf":
		return "pdf"
	case ".3gp", ".avi", ".m4v", ".mov", ".mp4":
		return "video"
	case ".amr", ".m4a", ".mp3", ".opus", ".wav":
		return "audio"
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
	filenamesByType = lo.GroupBy(allFilenames, FileType)
}

func FileCount() int {
	return len(allFilenames)
}

func GetRandomFilename() string {
	return getRandom(allFilenames)
}
