package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
)

// assetHashes maps each embedded static file ("app.css", "fonts/nunito.woff2")
// to a short hash of its content. Files only change with a new build.
var assetHashes = hashAssets()

func hashAssets() map[string]string {
	hashes := map[string]string{}
	err := fs.WalkDir(assets, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := assets.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		hashes[strings.TrimPrefix(p, "static/")] = hex.EncodeToString(sum[:8])
		return nil
	})
	if err != nil {
		panic(err)
	}
	return hashes
}

// assetURL is the cache-busting URL of a static file, for templates.
func assetURL(name string) string {
	return "/static/" + name + "?v=" + assetHashes[name]
}

// cacheStatic serves files under /static/ with the content hash as ETag. The
// exact URL from assetURL never changes content, so it is cached for a year;
// any other request (fonts from the CSS, old hashes) revalidates and gets a 304.
// Anything that is not a file, directories included, is a 404.
func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash, ok := assetHashes[strings.TrimPrefix(r.URL.Path, "/static/")]
		if !ok { // directories included: no listings
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", `"`+hash+`"`)
		if r.URL.Query().Get("v") == hash {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.ServeHTTP(w, r)
	})
}
