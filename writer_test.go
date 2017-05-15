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

func NOTestWrite(t *testing.T) {
	os.RemoveAll("./testdata")
	w := New("./testdata/roll.log", 50, 0)
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}
	w.Sync()
}

func TestWriteMerge(t *testing.T) {
	os.RemoveAll("./testdata")
	w := New("./testdata/roll.log", 50, 0)
	w.WriteMerging()
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}
	w.Sync()
}

func BenchmarkWrite(b *testing.B) {
	os.RemoveAll("./benchdata1")
	w := New("./benchdata1/roll.log", 100*1024*1024, 10)
	log.SetOutput(w)

	for i := 0; i < b.N; i++ {
		log.Println(i)
	}
	w.Sync()
}

func BenchmarkWriteMerge(b *testing.B) {
	os.RemoveAll("./benchdata2")
	w := New("./benchdata2/roll.log", 100*1024*1024, 10)
	log.SetOutput(w)
	w.WriteMerging()

	for i := 0; i < b.N; i++ {
		log.Println(i)
	}
	w.Sync()
}
