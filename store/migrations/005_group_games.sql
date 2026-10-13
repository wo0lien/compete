-- The games a group plays: they drive Home's "To play today" and push.
CREATE TABLE group_games (
  group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  game     TEXT NOT NULL,
  PRIMARY KEY (group_id, game)
);

-- Existing groups: the games their current members played in the last 14 days.
INSERT OR IGNORE INTO group_games
  SELECT DISTINCT m.group_id, r.game
  FROM memberships m JOIN results r ON r.user_id = m.user_id
  WHERE r.submitted_at > unixepoch() - 14*86400;

-- Groups still without a list: every game (ids frozen as of this migration).
INSERT INTO group_games
  SELECT g.id, x.game FROM groups g,
    (SELECT 'tusmo' AS game UNION SELECT 'songless' UNION SELECT 'travle') x
  WHERE g.id NOT IN (SELECT group_id FROM group_games);
