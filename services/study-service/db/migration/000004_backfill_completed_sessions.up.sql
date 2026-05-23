-- Heal "zombie" study sessions: rows whose status is still 'ongoing' even
-- though every card has been graded. Without this backfill, any /review POST
-- against one of these sessions returns 409 ErrCardAlreadyReviewed, leaving
-- the deck unreviewable until the client starts a new session.
--
-- The IncrementCompletedCards query now flips status atomically on the final
-- card (see study_session.sql), so new sessions can't enter this state.
UPDATE study_sessions
SET status      = 'completed',
    finished_at = COALESCE(finished_at, NOW())
WHERE status = 'ongoing'
  AND total_cards > 0
  AND completed_cards >= total_cards;
