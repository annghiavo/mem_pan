-- Stores the user's IANA timezone (e.g. "Asia/Ho_Chi_Minh", "America/Los_Angeles").
-- Used by the reminder cron jobs to compute the correct local time for
-- (a) the streak day boundary and (b) the optimal study-reminder send time.
ALTER TABLE users
    ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

CREATE INDEX idx_users_timezone ON users(timezone);
