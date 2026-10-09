package store

import (
	"database/sql"
	"errors"
	"time"
)

// Subscription is one device's Web Push subscription.
type Subscription struct {
	Endpoint, P256dh, Auth string
	UserID                 int64
}

// Prefs are the notification kinds a member wants.
type Prefs struct{ Digest, Reminder, Social, Friends bool }

// Recipient is everything needed to notify one member.
type Recipient struct {
	UserID int64
	Lang   string
	Prefs  Prefs
	Subs   []Subscription
}

// Setting returns a server setting, "" when unset.
func (s *Store) Setting(key string) (string, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// SaveSubscription stores a device's subscription; a device that subscribes
// again (or under another account) replaces its row.
func (s *Store) SaveSubscription(sub Subscription) error {
	_, err := s.db.Exec(`INSERT INTO push_subscriptions(endpoint, user_id, p256dh, auth) VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth`,
		sub.Endpoint, sub.UserID, sub.P256dh, sub.Auth)
	return err
}

// DeleteSubscription removes a subscription the push service reported gone.
func (s *Store) DeleteSubscription(endpoint string) error {
	_, err := s.db.Exec("DELETE FROM push_subscriptions WHERE endpoint = ?", endpoint)
	return err
}

// DeleteUserSubscription removes one of the user's own subscriptions.
func (s *Store) DeleteUserSubscription(userID int64, endpoint string) error {
	_, err := s.db.Exec("DELETE FROM push_subscriptions WHERE endpoint = ? AND user_id = ?", endpoint, userID)
	return err
}

func (s *Store) NotifyPrefs(userID int64) (Prefs, error) {
	var p Prefs
	err := s.db.QueryRow("SELECT notify_digest, notify_reminder, notify_social, notify_friends FROM users WHERE id = ?", userID).
		Scan(&p.Digest, &p.Reminder, &p.Social, &p.Friends)
	return p, err
}

func (s *Store) SetNotifyPrefs(userID int64, p Prefs) error {
	_, err := s.db.Exec("UPDATE users SET notify_digest = ?, notify_reminder = ?, notify_social = ?, notify_friends = ? WHERE id = ?",
		p.Digest, p.Reminder, p.Social, p.Friends, userID)
	return err
}

// MarkSent records a timed push; false when it was already sent (at most once).
func (s *Store) MarkSent(userID int64, key string) (bool, error) {
	res, err := s.db.Exec("INSERT INTO push_sent(user_id, key) VALUES (?, ?) ON CONFLICT DO NOTHING", userID, key)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) PruneSent(before time.Time) error {
	_, err := s.db.Exec("DELETE FROM push_sent WHERE sent_at < ?", before.Unix())
	return err
}

// PushUsers lists users with at least one subscription.
func (s *Store) PushUsers() ([]int64, error) {
	rows, err := s.db.Query("SELECT DISTINCT user_id FROM push_subscriptions ORDER BY user_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Recipient loads a user's language, prefs and subscriptions.
func (s *Store) Recipient(userID int64) (Recipient, error) {
	r := Recipient{UserID: userID}
	err := s.db.QueryRow("SELECT lang, notify_digest, notify_reminder, notify_social, notify_friends FROM users WHERE id = ?", userID).
		Scan(&r.Lang, &r.Prefs.Digest, &r.Prefs.Reminder, &r.Prefs.Social, &r.Prefs.Friends)
	if err != nil {
		return r, err
	}
	rows, err := s.db.Query("SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = ? ORDER BY rowid", userID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		sub := Subscription{UserID: userID}
		if err := rows.Scan(&sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return r, err
		}
		r.Subs = append(r.Subs, sub)
	}
	return r, rows.Err()
}
