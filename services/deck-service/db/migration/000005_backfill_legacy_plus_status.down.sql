-- This backfill cannot be safely reversed without a per-row marker. Leaving
-- approved Plus decks intact avoids downgrading legitimate creator/admin state.
SELECT 1;
