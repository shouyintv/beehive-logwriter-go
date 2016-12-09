package logwriter

import (
	"compress/gzip"
	"io"
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

type fislice []fileinfo

func (s fislice) Len() int {
	return len(s)
}

func (s fislice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s fislice) Less(i, j int) bool {
	return s[i].id < s[j].id
}

func collectFiles(dir string, prefix string, maxfile int) []fileinfo {
	if maxfile == 0 {
		return nil
	}
	filist := make([]fileinfo, 0, maxfile)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || err != nil {
			return err
		}
		if !info.IsDir() {
			name := info.Name()
			//  前缀
			if !strings.HasPrefix(name, prefix) {
				return nil
			}
			p := strings.LastIndexByte(name[:len(name)], '.')
			id, err := strconv.Atoi(name[p+1 : len(name)])
			// 忽略非数字结尾
			if err != nil {
				return nil
			}
			filist = append(filist, fileinfo{id: id, path: path})
		} else if dir != path {
			return filepath.SkipDir
		}
		return nil
	})
	sort.Sort(fislice(filist))
	head := len(filist) - maxfile
	if head < 0 {
		head = 0
	}
	return filist[head:]
}

func compress(src, dst string) error {
	fsrc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fsrc.Close()

	fdst, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fdst.Close()

	comp := gzip.NewWriter(fdst)
	defer comp.Close()
	_, err = io.Copy(comp, fsrc)
	return err
}
