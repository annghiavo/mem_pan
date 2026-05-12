-- name: UpsertDailyStats :exec
INSERT INTO daily_stats (user_id, study_date, reviews_count, new_cards_count, study_time_ms, correct_count)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, study_date) DO UPDATE SET
    reviews_count   = daily_stats.reviews_count   + EXCLUDED.reviews_count,
    new_cards_count = daily_stats.new_cards_count + EXCLUDED.new_cards_count,
    study_time_ms   = daily_stats.study_time_ms   + EXCLUDED.study_time_ms,
    correct_count   = daily_stats.correct_count   + EXCLUDED.correct_count;

-- name: ListDailyStats :many
SELECT * FROM daily_stats
WHERE user_id = $1
  AND study_date BETWEEN $2 AND $3
ORDER BY study_date ASC;
