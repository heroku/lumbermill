package lpxgen

import (
	"bytes"
	"github.com/bmizerany/lpx"
	"testing"
)

func TestDefaultLog(t *testing.T) {
	d := DefaultLog{}
	s := d.String()

	lp := lpx.NewReader(bytes.NewBufferString(s))
	for lp.Next() {
		if string(lp.Header().Name) != "default" {
			t.Errorf("Expected default: got %q\n", lp.Header().Name)
		}
	}

	if lp.Err() != nil {
		t.Errorf("lpx returned an error: %q", lp.Err())
	}
}