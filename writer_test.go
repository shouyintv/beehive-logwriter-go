package logwriter

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestRingPush(t *testing.T) {
	w := Writer{
		ring:     make([]fileinfo, 10),
		maxfiles: 10,
	}
	for i := 0; i < 20; i++ {
		t.Log(w.ringPush(i, "file_"+strconv.Itoa(i)))
	}
}

func TestSyncWrite(t *testing.T) {
	os.RemoveAll("./testdata/sync")
	w, err := NewWriter("./testdata/sync/roll.log", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	w.ToFile = true
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}
}

func TestAsyncWrite(t *testing.T) {
	os.RemoveAll("./testdata/async")
	w, err := NewWriter("./testdata/async/roll.log", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	w.Async()
	w.ToFile = true
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}

	time.Sleep(time.Second)
}

func TestWriteFields(t *testing.T) {
	rt := reflect.TypeOf(Writer{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		t.Logf("%-10s %-4d %4d", f.Name, f.Offset, f.Type.Size())
	}
}
