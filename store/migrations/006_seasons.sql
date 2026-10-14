-- Seasons of the race (#54): only boundaries are stored, points come from results.
ALTER TABLE groups ADD COLUMN season_days INTEGER NOT NULL DEFAULT 7;

CREATE TABLE seasons (
  id        INTEGER PRIMARY KEY,
  group_id  INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  number    INTEGER NOT NULL,          -- 1, 2… per group
  start_day TEXT NOT NULL,             -- YYYY-MM-DD
  n         INTEGER NOT NULL,          -- finish line, frozen at start
  end_day   TEXT,                      -- NULL while running
  winners   TEXT,                      -- JSON array of user ids, set when decided
  UNIQUE (group_id, number)
);
