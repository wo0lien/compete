// Package push sends Web Push notifications: the morning digest, the evening
// reminder and the social pings after a submit.
package push

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/wo0lien/compete/store"
)

// sendTimeout bounds one delivery: a push service that never answers must not
// stall the timer.
var sendTimeout = 10 * time.Second

// Message is the JSON payload sw.js shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

type Notifier struct {
	Store      *store.Store
	Text       func(lang, id string, args ...any) string // the web locales
	Subscriber string                                    // VAPID contact: the instance URL
	Send       func(store.Subscription, Message) error   // tests replace it
	Now        func() time.Time

	publicKey, privateKey string
}

// New loads the VAPID keys, creating them on first start so self-hosters
// configure nothing.
func New(st *store.Store, text func(lang, id string, args ...any) string, subscriber string) (*Notifier, error) {
	n := &Notifier{Store: st, Text: text, Subscriber: subscriber, Now: time.Now}
	n.Send = n.deliver
	keys, err := st.Setting("vapid")
	if err != nil {
		return nil, err
	}
	if keys == "" {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, err
		}
		keys = pub + " " + priv
		if err := st.SetSetting("vapid", keys); err != nil {
			return nil, err
		}
	}
	n.publicKey, n.privateKey, _ = strings.Cut(keys, " ")
	return n, nil
}

// PublicKey is the applicationServerKey browsers subscribe with.
func (n *Notifier) PublicKey() string { return n.publicKey }

// deliver sends one message; a subscription the push service says is gone is deleted.
func (n *Notifier) deliver(sub store.Subscription, m Message) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber: n.Subscriber, VAPIDPublicKey: n.publicKey, VAPIDPrivateKey: n.privateKey,
		TTL: int((12 * time.Hour).Seconds()), Urgency: webpush.UrgencyNormal,
		HTTPClient: &http.Client{Timeout: sendTimeout},
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return n.Store.DeleteSubscription(sub.Endpoint)
	case resp.StatusCode >= 400:
		return fmt.Errorf("push %s: %s", sub.Endpoint, resp.Status)
	}
	return nil
}

// sendAll delivers m to every subscription of r, logging failures.
func (n *Notifier) sendAll(r store.Recipient, m Message) {
	for _, sub := range r.Subs {
		if err := n.Send(sub, m); err != nil {
			log.Printf("push to user %d: %v", r.UserID, err)
		}
	}
}

// lang is the recipient's language for push texts ("" follows the browser on
// pages; pushes have no browser, so English).
func lang(r store.Recipient) string {
	if r.Lang == "" {
		return "en"
	}
	return r.Lang
}
