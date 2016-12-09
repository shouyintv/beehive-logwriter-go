// 提供了一个 Writer 实现文件滚动
package logwriter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	dirPerm  os.FileMode = 0755
	filePerm os.FileMode = 0644
)

// Writer 实现了一个支持文件滚动的 io.Writer
type Writer struct {
	f     *os.File
	wmu   sync.Mutex
	wrote int

	mu   sync.Mutex
	day  int
	id   int
	ring []fileinfo
	head int
	tail int

	limit    int
	maxfiles int
	path     string
	dir      string

	ToFile    bool
	ToConsole bool
}

func (w *Writer) ringPush(id int, name string) string {
	var removed string
	if w.maxfiles+w.head == w.tail && w.maxfiles != 0 {
		removed = w.ring[w.head%len(w.ring)].path
		w.head++
	}
	w.ring[w.tail%len(w.ring)] = fileinfo{id: id, path: name}
	w.tail++
	return removed
}

// reopen [unsafe]
func (w *Writer) reopen(day int) (err error) {
	if w.f != nil {
		w.f.Close()
	}
	err = os.MkdirAll(w.dir, dirPerm)
	if err != nil {
		return
	}
	w.f, err = os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, filePerm)
	if err == nil {
		w.day = day
		info, err := os.Stat(w.path)
		if err == nil {
			w.wrote = int(info.Size())
		}
		return nil
	}
	w.day = 0
	w.wrote = 0
	return
}

// rotate [unsafe]
func (w *Writer) rotate(year, month, day int) error {
	if w.f != nil {
		// prefix.yyyy-MM-dd.id
		w.id++
		newpath := w.path + fmt.Sprintf(".%04d-%02d-%02d.%d", year, month, day, w.id)

		os.Rename(w.path, newpath)

		if w.maxfiles > 0 {
			removed := w.ringPush(w.id, newpath)
			if removed != "" {
				os.Remove(removed)
			}
		}
	}

	return w.reopen(day)
}

func (w *Writer) writeToFile(p []byte) (n int, err error) {
	now := time.Now()
	f := w.f
	year, month, day := now.Date()
	if day != w.day {
		// daily rotate
		w.mu.Lock()
		if day != w.day {
			err = w.rotate(year, int(month), day)
		}
		f = w.f
		w.mu.Unlock()
	}
	if err != nil {
		return
	}

	w.wmu.Lock()
	w.wrote += len(p)
	if w.wrote > w.limit {
		// limit rotate
		w.mu.Lock()
		if w.wrote > w.limit {
			err = w.rotate(year, int(month), day)
		}
		f = w.f
		w.mu.Unlock()
		w.wrote = len(p)
	}
	w.wmu.Unlock()
	if err != nil {
		return
	}

	if f != nil {
		n, err = f.Write(p)
	}
	return
}

// Write 输出 p 内容到文件或 stdout
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.ToConsole {
		n, err = os.Stdout.Write(p)
	}
	if w.ToFile {
		// override os.Stdout.Write result
		n, err = w.writeToFile(p)
	}
	return
}

// Flush 同步文件缓冲
func (w *Writer) Flush() error {
	f := w.f
	if f != nil {
		return f.Sync()
	}
	return nil
}

// NewWriter 创建一个 logwriter.Writer
//
//   path 滚动日志文件
//   limit 单个文件大小
//   maxfiles 最多文件数量, 0 代表不限制文件数量
func NewWriter(path string, limit int, maxfiles int) (*Writer, error) {
	if path == "" || limit <= 0 || maxfiles < 0 {
		return nil, errors.New("invalid argument")
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	filist := collectFiles(dir, base, maxfiles)
	id := 0
	if len(filist) > 0 {
		id = filist[len(filist)-1].id
	}
	return &Writer{
		id:        id,
		ring:      filist[:cap(filist)],
		head:      0,
		tail:      len(filist),
		path:      path,
		dir:       dir,
		limit:     limit,
		maxfiles:  maxfiles,
		ToConsole: false,
		ToFile:    true,
	}, nil
}
