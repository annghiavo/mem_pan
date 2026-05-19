-- name: BumpActivityBucket :exec
-- Called from the study.completed subscriber. Increments the (hour, day_type)
-- bucket for the user.
INSERT INTO user_activity_buckets (user_id, hour_of_day, day_type, review_count, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, hour_of_day, day_type)
DO UPDATE SET
    review_count = user_activity_buckets.review_count + EXCLUDED.review_count,
    updated_at   = CURRENT_TIMESTAMP;

-- name: GetActivityBuckets :many
-- Returns the full distribution for a user (24 hours × 2 day-types = up to 48 rows).
SELECT hour_of_day, day_type, review_count
FROM user_activity_buckets
WHERE user_id = $1;

-- name: SetOptimalHours :exec
UPDATE user_stats SET
    optimal_hour_weekday = $2,
    optimal_hour_weekend = $3,
    updated_at           = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: GetReminderState :one
-- Returns everything the cron needs about a user: streak, last-studied,
-- cached optimal hours, custom reminder time.
SELECT
    user_id,
    current_streak,
    last_studied_date,
    optimal_hour_weekday,
    optimal_hour_weekend,
    reminder_local_time
FROM user_stats
WHERE user_id = $1;

-- name: ListUsersWithActiveStreak :many
-- Returns all users whose streak >= 1. The cron handler then filters by
-- timezone-local time and last_studied_date.
SELECT
    user_id,
    current_streak,
    last_studied_date,
    optimal_hour_weekday,
    optimal_hour_weekend,
    reminder_local_time
FROM user_stats
WHERE current_streak >= 1
ORDER BY user_id;

-- name: ListUsersForStudyReminder :many
-- Returns every user that has been seen by stats-service (has a row).
-- The cron handler iterates and filters by tz + optimal hour.
SELECT
    user_id,
    current_streak,
    last_studied_date,
    optimal_hour_weekday,
    optimal_hour_weekend,
    reminder_local_time
FROM user_stats
ORDER BY user_id;
