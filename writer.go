// 提供了一个 Writer 实现文件滚动
package logwriter

import (
	"bytes"
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

	logQueueSize = 2048
)

var (
	bufpool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(nil)
		},
	}
)

// Writer 实现了一个支持文件滚动的 io.Writer
type Writer struct {
	f     *os.File
	wmu   sync.Mutex
	wrote int

	mu  sync.Mutex
	day int

	limit     int
	logq      chan *bytes.Buffer
	async     bool
	ToFile    bool
	ToConsole bool

	id   int
	ring []fileinfo
	head int
	tail int

	maxfiles int
	path     string
	dir      string
	once     sync.Once
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
		if day == 1 {
			month--
			if month == 0 {
				month = 12
				year--
			}
		}
		newpath := w.path + fmt.Sprintf(".%04d-%02d-%02d.%d", year, month, w.day, w.id)

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

func (w *Writer) writeUnsafe(p []byte) error {
	var err error
	now := time.Now()
	f := w.f
	year, month, day := now.Date()
	if day != w.day {
		// daily rotate
		if day != w.day {
			err = w.rotate(year, int(month), day)
		}
		f = w.f
	}
	if err != nil {
		return err
	}

	w.wrote += len(p)
	if w.wrote > w.limit {
		// limit rotate
		if w.wrote > w.limit {
			err = w.rotate(year, int(month), day)
		}
		f = w.f
		w.wrote = len(p)
	}
	if err != nil {
		return err
	}

	if f != nil {
		_, err = f.Write(p)
		if err != nil {
			w.day = 0
		}
	}
	return nil
}

func (w *Writer) writeAsync(p []byte) (n int, err error) {
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Write(p)
	n = len(p)
	w.logq <- buf
	return
}

func (w *Writer) writeFile(p []byte) (n int, err error) {
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
		if err != nil {
			w.mu.Lock()
			w.day = 0
			w.mu.Unlock()
		}
	}
	return
}

// Write 输出 p 内容到文件或 stdout
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.ToConsole {
		n, err = os.Stdout.Write(p)
	}
	if w.ToFile {
		if w.async {
			n, err = w.writeAsync(p)
		} else {
			// override os.Stdout.Write result
			n, err = w.writeFile(p)
		}
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

func (w *Writer) asyncWork() {
	for buf := range w.logq {
		err := w.writeUnsafe(buf.Bytes())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		bufpool.Put(buf)
	}
}

func (w *Writer) Async() {
	w.once.Do(func() {
		w.logq = make(chan *bytes.Buffer, logQueueSize)
		go w.asyncWork()
		w.async = true
		w.wmu.Lock()
		w.mu.Lock()
	})
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
