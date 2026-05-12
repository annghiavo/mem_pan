-- name: GetUserStats :one
SELECT * FROM user_stats WHERE user_id = $1;

-- name: CreateUserStats :one
INSERT INTO user_stats (user_id, username, avatar_url)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: IncrementReview :exec
UPDATE user_stats SET
    total_reviews       = total_reviews + $2,
    total_study_time_ms = total_study_time_ms + $3,
    total_correct       = total_correct + $4,
    total_incorrect     = total_incorrect + $5,
    updated_at          = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: UpdateStreak :exec
UPDATE user_stats SET
    current_streak    = $2,
    longest_streak    = GREATEST(longest_streak, $2),
    last_studied_date = $3,
    updated_at        = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: IncrementUserCards :exec
UPDATE user_stats SET
    total_cards = total_cards + 1,
    updated_at  = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: UpdateUserProfile :exec
UPDATE user_stats SET
    username   = $2,
    avatar_url = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1;
