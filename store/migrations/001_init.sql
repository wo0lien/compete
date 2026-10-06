CREATE TABLE users (
  id         INTEGER PRIMARY KEY,
  username   TEXT NOT NULL UNIQUE COLLATE NOCASE,
  pass_hash  TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL
);

CREATE TABLE password_resets (
  token_hash BLOB PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL
);

CREATE TABLE groups (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  invite_code TEXT NOT NULL UNIQUE,
  created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE memberships (
  group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role      TEXT NOT NULL CHECK (role IN ('owner', 'member')),
  joined_at INTEGER NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX memberships_user ON memberships(user_id);

CREATE TABLE results (
  id           INTEGER PRIMARY KEY,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game         TEXT NOT NULL,
  variant      TEXT NOT NULL DEFAULT '',
  puzzle_id    INTEGER NOT NULL,
  score        INTEGER,          -- lower is better, NULL = failed
  tiebreak     INTEGER,          -- lower is better, NULL = none
  detail       TEXT,             -- JSON
  raw          TEXT NOT NULL,
  submitted_at INTEGER NOT NULL DEFAULT (unixepoch()),
  UNIQUE (user_id, game, variant, puzzle_id)
);
CREATE INDEX results_puzzle ON results(game, variant, puzzle_id);
