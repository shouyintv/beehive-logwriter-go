// Package logwriter 提供了一个 Writer 实现文件滚动
package logwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	dirPerm  os.FileMode = 0755
	filePerm os.FileMode = 0644

	queueSize = 2048
)

// Writer 实现了一个支持文件滚动的 io.Writer
type Writer struct {
	f     *os.File
	wrote int
	day   int

	limit int
	wq    chan []byte

	year  int
	month int
	id    int
	ring  []fileinfo
	head  int
	tail  int

	maxfiles int
	path     string
	dir      string

	mu   sync.Mutex
	cond sync.Cond
	err  error
}

func (w *Writer) push(id int, name string) string {
	var removed string
	if w.maxfiles+w.head == w.tail && w.maxfiles != 0 {
		removed = w.ring[w.head%len(w.ring)].path
		w.head++
	}
	w.ring[w.tail%len(w.ring)] = fileinfo{id: id, path: name}
	w.tail++
	return removed
}

func (w *Writer) reopen(year, month, day int) (err error) {
	if w.f != nil {
		w.f.Close()
	}

	err = os.MkdirAll(w.dir, dirPerm)
	if err != nil {
		return
	}

	w.f, err = os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, filePerm)
	if err == nil {
		w.year = year
		w.month = month
		w.day = day
		info, err := w.f.Stat()
		if err == nil {
			w.wrote = int(info.Size())
		}
		return nil
	}

	w.day = 0
	w.wrote = 0
	return
}

func (w *Writer) rotate(year, month, day int) error {
	if w.f != nil {
		// prefix.yyyy-MM-dd.id
		w.id++
		newpath := w.path + fmt.Sprintf(".%04d-%02d-%02d.%d", w.year, w.month, w.day, w.id)

		if runtime.GOOS == "windows" {
			w.f.Close()
		}
		os.Rename(w.path, newpath)

		if w.maxfiles > 0 {
			removed := w.push(w.id, newpath)
			if removed != "" {
				os.Remove(removed)
			}
		}
	}

	return w.reopen(year, month, day)
}

func (w *Writer) write(p []byte) error {
	var err error
	now := time.Now()
	year, month, day := now.Date()
	if day != w.day {
		// 日期滚动
		err = w.rotate(year, int(month), day)
		if err != nil {
			return err
		}
	}

	w.wrote += len(p)
	if w.wrote > w.limit {
		// 大小滚动
		err = w.rotate(year, int(month), day)
		if err != nil {
			return err
		}
		// 每个文件至少被写一次
		w.wrote = len(p)
	}

	if f := w.f; f != nil {
		_, err = f.Write(p)
		if err != nil {
			w.day = 0
		}
	}
	return nil
}

func (w *Writer) ioloop() {
	for buf := range w.wq {
		if buf == nil {
			// nil 代表 sync 信号
			if w.f != nil {
				err := w.f.Sync()
				if err != nil {
					w.err = err
					w.f.Close()
					w.f = nil
				}
			}
			w.cond.Signal()
			continue
		}

		err := w.write(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

// Write 输出 p 内容到文件或 stdout
func (w *Writer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	buf := make([]byte, len(p))
	n = copy(buf, p)
	w.wq <- buf
	return
}

// Sync 同步文件缓冲
func (w *Writer) Sync() error {
	w.mu.Lock()
	w.wq <- nil
	w.cond.L.Lock()
	w.cond.Wait()
	w.cond.L.Unlock()
	err := w.err
	w.mu.Unlock()
	return err
}

// New 创建一个 logwriter.Writer
//
//   path 滚动日志文件
//   limit 单个文件大小
//   maxfiles 最多文件数量, 0 不限制文件数量
func New(path string, limit int, maxfiles int) *Writer {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	filist, maxid := collectFiles(dir, base, maxfiles)

	w := &Writer{
		limit:    limit,
		wq:       make(chan []byte, queueSize),
		id:       maxid,
		ring:     filist[:cap(filist)],
		head:     0,
		tail:     len(filist),
		maxfiles: maxfiles,
		path:     path,
		dir:      dir,
		cond: sync.Cond{
			L: &sync.Mutex{},
		},
	}

	go w.ioloop()

	return w
}

func NewWriter(path string, limit int, maxfiles int) (*Writer, error) {
	return New(path, limit, maxfiles), nil
}
