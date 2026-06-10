DROP TABLE IF EXISTS deck_reviews;
DROP TABLE IF EXISTS creator_followers;
DROP TABLE IF EXISTS creator_profiles;

DROP INDEX IF EXISTS idx_decks_plus_public;

ALTER TABLE decks
DROP COLUMN IF EXISTS total_reviews,
DROP COLUMN IF EXISTS avg_rating,
DROP COLUMN IF EXISTS plus_approved_at,
DROP COLUMN IF EXISTS plus_submitted_at,
DROP COLUMN IF EXISTS plus_status,
DROP COLUMN IF EXISTS access_level;

DROP TYPE IF EXISTS creator_tier;
DROP TYPE IF EXISTS deck_plus_status;
DROP TYPE IF EXISTS deck_access_level;
