-- name: CreateStudySession :one
INSERT INTO study_sessions (user_id, deck_id, total_cards)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetStudySession :one
SELECT * FROM study_sessions WHERE session_id = $1;

-- name: GetOngoingSessionByDeck :one
SELECT * FROM study_sessions
WHERE user_id = $1 AND deck_id = $2 AND status = 'ongoing'
LIMIT 1;

-- name: FinishStudySession :one
UPDATE study_sessions SET
    status           = 'completed',
    finished_at      = NOW(),
    last_accessed_at = NOW()
WHERE session_id = $1
RETURNING *;

-- name: AbandonStudySession :one
UPDATE study_sessions SET
    status           = 'abandoned',
    finished_at      = NOW(),
    last_accessed_at = NOW()
WHERE session_id = $1
RETURNING *;

-- name: IncrementCompletedCards :one
UPDATE study_sessions SET
    completed_cards      = completed_cards + 1,
    last_completed_index = last_completed_index + 1,
    last_accessed_at     = NOW()
WHERE session_id = $1
RETURNING *;
