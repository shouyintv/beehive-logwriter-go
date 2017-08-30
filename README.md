# beehive-logwriter-go
支持文件滚动的 Writer

## install
```
go get github.com/shouyintv/beehive-logwriter-go
```

## usage
```go
package main

import (
	logwriter "github.com/shouyintv/beehive-logwriter-go"
)

func main() {
	// create roll.log at log directory.
	// each log file limits 50MB.
	// max 10 files.
	w := logwriter.New("log/roll.log", 50*1024*1024, 10)
	w.Write([]byte("hello world\n"))
	w.Sync()
}
```
