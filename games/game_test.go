package games

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	got := Normalize("  ⬛\uFE0F⬜\uFE0F\r\nline2\r\n ")
	if want := "⬛⬜\nline2"; got != want {
		t.Fatalf("Normalize = %q, want %q", got, want)
	}
}

func TestGridOf(t *testing.T) {
	got := gridOf("TUSMO #70 3/6\n\n🟥🟥⬛\n🟥🟥🟥\n\nhttps://www.tusmo.xyz")
	if want := "🟥🟥⬛\n🟥🟥🟥"; got != want {
		t.Fatalf("gridOf = %q, want %q", got, want)
	}
}

func TestParseUnknown(t *testing.T) {
	if got := Parse("hello world"); got != nil {
		t.Fatalf("Parse(unknown) = %v, want nil", got)
	}
}

// checkFixture parses testdata/<file> through the full registry, so it also
// proves no other game's parser claims the text first.
func checkFixture(t *testing.T, file string, want []Result) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if got := Parse(string(b)); !reflect.DeepEqual(got, want) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		t.Errorf("%s:\n got  %s\n want %s", file, g, w)
	}
}
