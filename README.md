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
	"github.com/shouyintv/beehive-logwriter-go"
)

func main() {
	w, err := logwriter.NewWriter("log/roll.log", 50*1024*1024, 10)
	if err != nil {
		panic(err)
	}
	w.ToConsole = true
	w.Write([]byte("hello world\n"))
    w.Sync()
}
```
