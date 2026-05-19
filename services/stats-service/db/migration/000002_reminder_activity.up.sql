-- Per-user activity histogram by (hour-of-day, day-type) bucket.
-- Used to compute optimal_hour = argmax(P(user_active | hour, day_type)).
--
-- day_type:
--   0 = weekday (Mon-Fri)
--   1 = weekend (Sat-Sun)
--
-- The bucket is keyed on the *local* hour-of-day in the user's timezone at
-- the time of study, so the histogram is timezone-invariant once written.
CREATE TABLE IF NOT EXISTS user_activity_buckets (
    user_id      UUID    NOT NULL,
    hour_of_day  SMALLINT NOT NULL CHECK (hour_of_day BETWEEN 0 AND 23),
    day_type     SMALLINT NOT NULL CHECK (day_type    BETWEEN 0 AND 1),
    review_count INTEGER NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, hour_of_day, day_type)
);

CREATE INDEX idx_activity_buckets_user ON user_activity_buckets(user_id);

-- Cached optimal hour per user, recomputed nightly by the worker.
-- NULL means "not enough data — fall back to default reminder time".
ALTER TABLE user_stats
    ADD COLUMN optimal_hour_weekday SMALLINT,
    ADD COLUMN optimal_hour_weekend SMALLINT,
    ADD COLUMN reminder_local_time  TIME NOT NULL DEFAULT '21:00:00';  -- streak-warning default
