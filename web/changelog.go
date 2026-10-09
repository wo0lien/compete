package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type release struct {
	Version, Date string
	EN            []string `toml:"en"`
	FR            []string `toml:"fr"`
}

// changelog is web/changelog.toml, newest first.
var changelog = loadChangelog()

func loadChangelog() []release {
	b, err := assets.ReadFile("changelog.toml")
	if err != nil {
		panic(err)
	}
	var f struct {
		Release []release `toml:"release"`
	}
	if err := toml.Unmarshal(b, &f); err != nil {
		panic(err)
	}
	return f.Release
}

// versionLess compares "vX.Y.Z" numerically.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

func (s *Server) changelogPage(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Version, Date string
		Lines         []string
	}
	var es []entry
	for _, rel := range changelog {
		lines := rel.EN
		if langFrom(r) == "fr" {
			lines = rel.FR
		}
		es = append(es, entry{rel.Version, rel.Date, lines})
	}
	s.render(w, r, http.StatusOK, "changelog.html", map[string]any{"Releases": es})
}
