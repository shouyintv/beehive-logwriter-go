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

	queueCapacity = 2048
)

// Writer 实现了一个支持文件滚动的 io.Writer
type Writer struct {
	f     *os.File
	wrote int
	day   int

	limit int
	logq  chan []byte

	ToConsole   bool
	DailyRotate bool
	LimitRotate bool

	id   int
	ring []fileinfo
	head int
	tail int

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

// rotate [unsafe]
func (w *Writer) rotate(year, month, day int) error {
	if w.f != nil {
		// prefix.yyyy-MM-dd.id
		w.id++
		if day == 1 {
			month--
			if month == 0 {
				month = 12
				year--
			}
		}

		if runtime.GOOS == "windows" {
			w.f.Close()
		}

		newpath := w.path + fmt.Sprintf(".%04d-%02d-%02d.%d", year, month, w.day, w.id)
		os.Rename(w.path, newpath)

		if w.maxfiles > 0 {
			removed := w.push(w.id, newpath)
			if removed != "" {
				os.Remove(removed)
			}
		}
	}

	return w.reopen(day)
}

// write [unsafe]
func (w *Writer) write(p []byte) error {
	var err error
	f := w.f
	now := time.Now()
	year, month, day := now.Date()
	if w.DailyRotate && day != w.day {
		// 日期滚动
		if day != w.day {
			err = w.rotate(year, int(month), day)
			if err != nil {
				return err
			}
		}
		f = w.f
	}

	w.wrote += len(p)
	if w.LimitRotate && w.wrote > w.limit {
		// 大小滚动
		err = w.rotate(year, int(month), day)
		if err != nil {
			return err
		}
		// 每个文件至少被写一次
		f = w.f
		w.wrote = len(p)
	}

	if f != nil {
		_, err = f.Write(p)
		if err != nil {
			w.day = 0
		}
	}
	return nil
}

func (w *Writer) ioloop() {
	for buf := range w.logq {
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

	if w.ToConsole {
		os.Stdout.Write(p)
	}

	buf := make([]byte, len(p))
	n = copy(buf, p)
	w.logq <- buf
	return
}

// Sync 同步文件缓冲
func (w *Writer) Sync() error {
	w.mu.Lock()
	w.logq <- nil
	w.cond.Wait()
	err := w.err
	w.mu.Unlock()
	return err
}

// NewWriter 创建一个 logwriter.Writer
//
//   path 滚动日志文件
//   limit 单个文件大小
//   maxfiles 最多文件数量, 0 不限制文件数量
func NewWriter(path string, limit int, maxfiles int) *Writer {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	filist, maxid := collectFiles(dir, base, maxfiles)

	w := &Writer{
		limit:       limit,
		logq:        make(chan []byte, queueCapacity),
		ToConsole:   false,
		DailyRotate: true,
		LimitRotate: true,
		id:          maxid,
		ring:        filist[:cap(filist)],
		head:        0,
		tail:        len(filist),
		maxfiles:    maxfiles,
		path:        path,
		dir:         dir,
	}
	w.cond.L = &w.mu

	go w.ioloop()
	return w
}
