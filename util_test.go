package logwriter

import "testing"

func TestCollectFiles(t *testing.T) {
	t.Log(collectFiles("./testdata", "roll.log", 0))
	t.Log(collectFiles("./testdata", "roll.log", 10))
	t.Log(collectFiles("./testdata", "roll.log", 30))
}
