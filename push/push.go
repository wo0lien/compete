// Package push sends Web Push notifications: the morning digest, the evening
// reminder and the social pings after a submit.
package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/wo0lien/compete/store"
)

// sendTimeout bounds one delivery: a push service that never answers must not
// stall the timer.
var sendTimeout = 10 * time.Second

// errGone: the push service says the device unsubscribed; its row is deleted.
var errGone = errors.New("subscription gone")

// Message is the JSON payload sw.js shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	// Urgent wakes a dozing Android phone (RFC 8030 "high"): the timed pushes,
	// which are useless late, and the test, which must behave like them.
	// Social pings stay "normal" and may be batched.
	Urgent bool `json:"-"`
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
	urgency := webpush.UrgencyNormal
	if m.Urgent {
		urgency = webpush.UrgencyHigh
	}
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber: n.Subscriber, VAPIDPublicKey: n.publicKey, VAPIDPrivateKey: n.privateKey,
		// short TTL: a push stuck past its moment is dropped, not shown late
		TTL: int((2 * time.Hour).Seconds()), Urgency: urgency,
		HTTPClient: &http.Client{Timeout: sendTimeout},
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	// one line per delivery (push service host, not the endpoint: it is a capability URL)
	host := sub.Endpoint
	if u, err := url.Parse(sub.Endpoint); err == nil {
		host = u.Host
	}
	log.Printf("push user %d via %s: %s", sub.UserID, host, resp.Status)
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		log.Printf("push user %d via %s: subscription gone, removed", sub.UserID, host)
		if err := n.Store.DeleteSubscription(sub.Endpoint); err != nil {
			return err
		}
		return errGone
	case resp.StatusCode >= 400:
		return fmt.Errorf("push service refused: %s", resp.Status)
	}
	return nil
}

// sendAll delivers m to every subscription of r, logging failures, and
// returns how many deliveries a push service accepted.
func (n *Notifier) sendAll(r store.Recipient, m Message) (sent int) {
	for _, sub := range r.Subs {
		switch err := n.Send(sub, m); {
		case err == nil:
			sent++
		case !errors.Is(err, errGone): // already logged by deliver
			log.Printf("push to user %d: %v", r.UserID, err)
		}
	}
	return sent
}

// Test sends the test message to every device of the member, whatever their
// switches, and returns how many got it.
func (n *Notifier) Test(userID int64) int {
	r, err := n.Store.Recipient(userID)
	if err != nil {
		log.Printf("push test: %v", err)
		return 0
	}
	return n.sendAll(r, Message{Title: "compete", Body: n.Text(lang(r), "push.test"), URL: "/account", Urgent: true})
}

// lang is the recipient's language for push texts ("" follows the browser on
// pages; pushes have no browser, so English).
func lang(r store.Recipient) string {
	if r.Lang == "" {
		return "en"
	}
	return r.Lang
}
