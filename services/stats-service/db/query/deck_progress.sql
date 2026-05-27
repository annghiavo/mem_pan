-- name: UpsertDeckProgressSnapshot :exec
INSERT INTO deck_progress_snapshots (deck_id, user_id, snapshot_date, new_count, learning_count, review_count, mastered_count)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (deck_id, user_id, snapshot_date) DO UPDATE SET
    new_count      = EXCLUDED.new_count,
    learning_count = EXCLUDED.learning_count,
    review_count   = EXCLUDED.review_count,
    mastered_count = EXCLUDED.mastered_count;

-- name: ListDeckProgressSnapshots :many
SELECT * FROM deck_progress_snapshots
WHERE deck_id = $1
  AND snapshot_date BETWEEN $2 AND $3
ORDER BY snapshot_date ASC;

-- name: DeleteDeckProgressSnapshots :exec
DELETE FROM deck_progress_snapshots WHERE deck_id = $1 AND user_id = $2;
