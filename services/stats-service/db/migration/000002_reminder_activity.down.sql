ALTER TABLE user_stats
    DROP COLUMN IF EXISTS reminder_local_time,
    DROP COLUMN IF EXISTS optimal_hour_weekend,
    DROP COLUMN IF EXISTS optimal_hour_weekday;

DROP INDEX IF EXISTS idx_activity_buckets_user;
DROP TABLE IF EXISTS user_activity_buckets;
