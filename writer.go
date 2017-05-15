// Package logwriter 提供了一个 Writer 实现文件滚动
package logwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const (
	dirPerm  os.FileMode = 0755
	filePerm os.FileMode = 0644

	queueCapacity = 4096
)

type nolock struct{}

func (*nolock) Lock()   {}
func (*nolock) Unlock() {}

// Writer 实现了一个支持文件滚动的 io.Writer
type Writer struct {
	f           *os.File
	wq          chan []byte
	nwrote      int
	limit       int
	writeMerge  bool
	DailyRotate bool

	day   int
	month int
	year  int

	nwm   int
	wmbuf []byte

	id       int
	ring     []fileinfo
	head     int
	tail     int
	maxfiles int

	path string
	dir  string

	mu   sync.Mutex
	cond sync.Cond
}

func (w *Writer) push(id int, name string) string {
	var pop string
	if w.maxfiles+w.head == w.tail {
		pop = w.ring[w.head%len(w.ring)].path
		w.head++
	}
	w.ring[w.tail%len(w.ring)] = fileinfo{id: id, path: name}
	w.tail++
	return pop
}

func (w *Writer) open(year, month, day int) (err error) {
	w.f, err = os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, filePerm)
	if err != nil {
		// 如果创建文件失败重新创建目录
		err = os.MkdirAll(w.dir, dirPerm)
		if err != nil {
			return err
		}
		w.f, err = os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, filePerm)
	}

	if err != nil {
		return err
	}

	w.year = year
	w.month = month
	w.day = day
	w.nwrote = 0

	if fi, err := w.f.Stat(); err == nil {
		w.nwrote = int(fi.Size())
	}

	return nil
}

func (w *Writer) flushwm() (err error) {
	_, err = w.f.Write(w.wmbuf[:w.nwm])
	w.nwm = 0
	return
}

func (w *Writer) rotate(year, month, day int) (err error) {
	if w.nwm > 0 {
		err = w.flushwm()
		if err != nil {
			return err
		}
	}

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
	w.f.Close()

	err = w.open(year, month, day)
	return
}

func (w *Writer) write(p []byte) error {
	year, month, day := time.Now().Date()
	if w.f == nil {
		err := w.open(year, int(month), day)
		if err != nil {
			return err
		}
	}

	if w.DailyRotate && day != w.day {
		// 日期滚动
		err := w.rotate(year, int(month), day)
		if err != nil {
			return err
		}
	}

	w.nwrote += len(p)
	// 如果接下来写的长度溢出, 先滚动再写
	if w.limit > 0 && w.nwrote > w.limit {
		// 单文件大小滚动
		err := w.rotate(year, int(month), day)
		if err != nil {
			return err
		}
		// 每个文件至少被写一次
		w.nwrote = len(p)
	}

	if w.writeMerge {
		if w.nwm != 0 && w.nwm+len(p) > len(w.wmbuf) {
			err := w.flushwm()
			if err != nil {
				return err
			}
		}

		if len(p) <= len(w.wmbuf) {
			w.nwm += copy(w.wmbuf[w.nwm:], p)
			return nil
		}
	}

	_, err := w.f.Write(p)
	return err
}

func (w *Writer) ioproc() {
	for buf := range w.wq {
		if buf == nil {
			// nil 代表 sync 信号
			if w.f != nil {
				var serr error
				if w.nwm > 0 {
					serr = w.flushwm()
				}

				if serr == nil {
					serr = w.f.Sync()
				}

				if serr != nil {
					fmt.Fprintln(os.Stderr, serr)
					w.f.Close()
					w.f = nil
				}
			}
			w.cond.Signal()
			continue
		}

		err := w.write(buf)
		if err == nil {
			if len(w.wq) == 0 {
				err = w.flushwm()
			}
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.f.Close()
			w.f = nil
		}
	}
}

func (w *Writer) WriteMerging() {
	w.wmbuf = make([]byte, syscall.Getpagesize())
	w.nwm = 0
	w.writeMerge = true
}

// Write 输出 p 内容到文件
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
	w.mu.Unlock()
	return nil
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
		wq:          make(chan []byte, queueCapacity),
		limit:       limit,
		DailyRotate: true,
		id:          maxid,
		ring:        filist[:cap(filist)],
		head:        0,
		tail:        len(filist),
		maxfiles:    maxfiles,
		path:        path,
		dir:         dir,
		cond: sync.Cond{
			L: &nolock{},
		},
	}

	go w.ioproc()

	return w
}
