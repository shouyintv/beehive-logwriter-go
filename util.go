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

func collectFiles(dir string, prefix string, maxfiles int) (filist []fileinfo, maxid int) {
	if maxfiles > 0 {
		filist = make([]fileinfo, 0, maxfiles)
	}

	fdir, err := os.Open(dir)
	if err != nil {
		return
	}
	defer fdir.Close()

	names, err := fdir.Readdirnames(-1)
	if err != nil {
		return
	}

	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			// 过滤前缀
			continue
		}
		p := strings.LastIndexByte(name[:len(name)], '.')
		id, err := strconv.Atoi(name[p+1 : len(name)])
		if err != nil {
			// 忽略非数字结尾的文件
			continue
		}

		path := filepath.Join(dir, name)
		fi, err := os.Lstat(path)
		if err != nil {
			continue
		}

		if fi.IsDir() {
			continue
		}

		if id > maxid {
			maxid = id
		}
		if maxfiles > 0 {
			filist = append(filist, fileinfo{id: id, path: path})
		}
	}
	sort.Slice(filist, func(i, j int) bool {
		return filist[i].id < filist[j].id
	})

	head := len(filist) - maxfiles
	if head < 0 {
		head = 0
	}
	return filist[head:], maxid
}
