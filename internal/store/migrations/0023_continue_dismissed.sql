-- Things a profile removed from Continue Watching (a movie, an episode, or a whole show), and when.
-- They stay out until the profile plays them (or, for a show, any of its episodes) again.
CREATE TABLE continue_dismissed (
    user_id TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id TEXT    NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    at      INTEGER NOT NULL,
    PRIMARY KEY (user_id, item_id)
);
