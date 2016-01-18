package spec

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSimpleParse(t *testing.T) {
	var doc Swagger
	f, err := os.Open("testdata/petstore-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&doc); err != nil {
		t.Error(err)
	}
}
