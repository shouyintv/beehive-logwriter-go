package logwriter

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestWriteFields(t *testing.T) {
	rt := reflect.TypeOf(Writer{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		t.Logf("%-10s %-4d %4d", f.Name, f.Offset, f.Type.Size())
	}
}

func TestRingPush(t *testing.T) {
	w := Writer{
		ring:     make([]fileinfo, 10),
		maxfiles: 10,
	}
	for i := 0; i < 20; i++ {
		t.Log(w.push(i, "file_"+strconv.Itoa(i)))
	}
}

func TestWrite(t *testing.T) {
	os.RemoveAll("./testdata")
	w := New("./testdata/roll.log", 50, 0)
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}
	w.Sync()
}
