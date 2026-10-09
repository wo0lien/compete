CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);

CREATE TABLE push_subscriptions (
  endpoint   TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  p256dh     TEXT NOT NULL,
  auth       TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX push_subscriptions_user ON push_subscriptions(user_id);

CREATE TABLE push_sent (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  sent_at INTEGER NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (user_id, key)
);

ALTER TABLE users ADD COLUMN notify_digest   INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN notify_reminder INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN notify_social   INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN notify_friends  INTEGER NOT NULL DEFAULT 0;
