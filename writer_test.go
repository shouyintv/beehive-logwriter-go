package logwriter

import (
	"log"
	"os"
	"strconv"
	"testing"
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

func TestWrite(t *testing.T) {
	os.RemoveAll("./testdata/")
	w, err := NewWriter("./testdata/roll.log", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	w.ToFile = true
	log.SetOutput(w)

	for i := 0; i < 30; i++ {
		log.Println(i)
	}
}
