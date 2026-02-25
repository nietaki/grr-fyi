package filedb

import "math/rand"

var files []string

func StoreFilenames(f []string) {
	files = f
}

func GetRandomFilename() string {
	if len(files) == 0 {
		return "no entries"
	}
	// get random index
	index := rand.Intn(len(files))
	return files[index]
}
