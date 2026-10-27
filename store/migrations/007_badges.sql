-- Badges (spec 2026-10-27): one row per earn; the share link uses token, never id.
CREATE TABLE badges (
  id        INTEGER PRIMARY KEY,
  token     TEXT NOT NULL UNIQUE,
  group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind      TEXT NOT NULL,               -- 'lonely'
  day       TEXT NOT NULL,               -- YYYY-MM-DD the badge is for
  earned_at INTEGER NOT NULL,
  UNIQUE (group_id, user_id, kind, day)
);
CREATE INDEX badges_user ON badges(user_id, group_id);

-- Badges earned after this are "new" for the member.
ALTER TABLE memberships ADD COLUMN badges_seen_at INTEGER NOT NULL DEFAULT 0;
-- Last day awarded; days after it are evaluated once over. No backfill.
ALTER TABLE groups ADD COLUMN badges_through TEXT;

UPDATE memberships SET badges_seen_at = unixepoch();
UPDATE groups SET badges_through = date('now', '-1 day');
