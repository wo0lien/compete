package web

import (
	"net/http"
	"strings"
)

// platforms are /help's install guides, in chip order.
var platforms = []string{"android-chrome", "android-other", "ios", "desktop"}

// platformOf guesses which install guide /help opens on. A wrong guess costs
// one tap on another chip (an iPad reporting a Mac lands on desktop).
func platformOf(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		return "ios"
	case !strings.Contains(ua, "Android"):
		return "desktop"
	}
	// Chromium-based browsers other than Chrome also say "Chrome/", but have
	// no share target for installed web apps or keep their own install flow.
	for _, other := range []string{"SamsungBrowser", "Firefox", "EdgA", "OPR", "YaBrowser", "UCBrowser"} {
		if strings.Contains(ua, other) {
			return "android-other"
		}
	}
	if strings.Contains(ua, "Chrome/") {
		return "android-chrome"
	}
	return "android-other"
}

func (s *Server) helpPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "help.html", map[string]any{
		"Platform": platformOf(r.UserAgent()), "Platforms": platforms})
}
