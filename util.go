package logwriter

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type fileinfo struct {
	id   int
	path string
}

func collectFiles(dir string, prefix string, maxfile int) ([]fileinfo, int) {
	if maxfile < 0 {
		maxfile = 0
	}

	maxid := 0
	filist := make([]fileinfo, 0, maxfile)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || err != nil {
			return err
		}
		if !info.IsDir() {
			name := info.Name()
			// filter by name prefix
			if !strings.HasPrefix(name, prefix) {
				return nil
			}
			p := strings.LastIndexByte(name[:len(name)], '.')
			id, err := strconv.Atoi(name[p+1 : len(name)])
			if err != nil {
				// ignore non-numeric suffix
				return nil
			}
			if id > maxid {
				maxid = id
			}
			if maxfile > 0 {
				filist = append(filist, fileinfo{id: id, path: path})
			}
		} else if dir != path {
			return filepath.SkipDir
		}
		return nil
	})
	sort.Slice(filist, func(i, j int) bool {
		return filist[i].id < filist[j].id
	})

	head := len(filist) - maxfile
	if head < 0 {
		head = 0
	}
	return filist[head:], maxid
}
