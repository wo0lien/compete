package web

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

// langs are the UI languages; the first one is the fallback.
var langs = []language.Tag{language.English, language.French}

var langMatcher = language.NewMatcher(langs)

// localizers holds one localizer per supported language ("en", "fr").
var localizers = map[string]*i18n.Localizer{}

func init() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	for _, tag := range langs {
		name := tag.String()
		if _, err := bundle.LoadMessageFileFS(assets, "locales/"+name+".toml"); err != nil {
			panic(err)
		}
		localizers[name] = i18n.NewLocalizer(bundle, name)
	}
}

// supported reports whether lang is a UI language code.
func supported(lang string) bool {
	_, ok := localizers[lang]
	return ok
}

// resolveLang picks the UI language: the account's choice, then the lang
// cookie, then the browser's Accept-Language, then English.
func resolveLang(account, cookie, accept string) string {
	for _, l := range []string{account, cookie} {
		if supported(l) {
			return l
		}
	}
	tags, _, err := language.ParseAcceptLanguage(accept)
	if err != nil || len(tags) == 0 {
		return langs[0].String()
	}
	_, i, conf := langMatcher.Match(tags...)
	if conf == language.No {
		return langs[0].String()
	}
	return langs[i].String()
}

type langKey struct{}

// withLang attaches the request's UI language; it runs after withUser.
func withLang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var account, cookie string
		if u, ok := userFrom(r); ok {
			account = u.Lang
		}
		if c, err := r.Cookie("lang"); err == nil {
			cookie = c.Value
		}
		lang := resolveLang(account, cookie, r.Header.Get("Accept-Language"))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), langKey{}, lang)))
	})
}

func langFrom(r *http.Request) string {
	if l, ok := r.Context().Value(langKey{}).(string); ok {
		return l
	}
	return langs[0].String()
}

// tr translates message id; args are key/value pairs for the message's
// template data. A missing message is logged and shows its id.
func tr(lang, id string, args ...any) string {
	data := map[string]any{}
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			data[k] = args[i+1]
		}
	}
	s, err := localizers[lang].Localize(&i18n.LocalizeConfig{MessageID: id, TemplateData: data})
	if err != nil {
		log.Printf("i18n %s: %v", lang, err)
		return id
	}
	return s
}

// Translate is tr for other packages (push texts).
func Translate(lang, id string, args ...any) string { return tr(lang, id, args...) }

// num formats a decimal with one digit, with a decimal comma in French.
func num(lang string, f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	if lang == "fr" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// setLang handles the FR/EN switch: remembers the choice in a cookie and on
// the account, then goes back to the page it came from.
func (s *Server) setLang(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("lang")
	if !supported(lang) {
		http.Error(w, "unsupported language", http.StatusBadRequest)
		return
	}
	if u, ok := userFrom(r); ok {
		if err := s.store.SetLang(u.ID, lang); err != nil {
			s.oops(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "lang", Value: lang, Path: "/", MaxAge: int(365 * 24 * time.Hour / time.Second),
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, safeNext(r.FormValue("back")), http.StatusSeeOther)
}

// errMsgs maps store validation errors (English, for logs and the CLI) to
// message IDs shown to users.
var errMsgs = map[error]string{
	store.ErrUsernameTaken: "err.username_taken",
	store.ErrBadUsername:   "err.bad_username",
	store.ErrWeakPassword:  "err.weak_password",
	store.ErrBadLogin:      "err.bad_login",
	store.ErrBadPawn:       "err.bad_pawn",
	store.ErrBadGroupName:  "err.bad_group_name",
	store.ErrTooManyGroups: "err.too_many_groups",
}

// errText translates a store validation error; anything else gets the
// generic error text.
func errText(r *http.Request, err error) string {
	for e, id := range errMsgs {
		if errors.Is(err, e) {
			return tr(langFrom(r), id, "N", store.MaxGroupsPerUser)
		}
	}
	return tr(langFrom(r), "msg.oops_text")
}

// scoreText shows a score; games keep language-neutral formats ("3/6", "+2"),
// only Travle's failed result is a word.
func scoreText(lang, id string, score *int) string {
	g, _ := games.ByID(id)
	if score == nil && g.Fail == "fail" {
		return tr(lang, "score.fail")
	}
	return g.ShowScore(score)
}

// tiebreakText shows a tiebreak; times ("1:05") are neutral, distances are words.
func tiebreakText(lang, id string, tb *int) string {
	g, _ := games.ByID(id)
	if tb != nil && g.TiebreakKind == "away" {
		return tr(lang, "score.away", "N", *tb)
	}
	return g.ShowTiebreak(tb)
}
