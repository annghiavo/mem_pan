ALTER TABLE deck_study_settings
DROP COLUMN IF EXISTS new_card_limit,
DROP COLUMN IF EXISTS review_card_limit;
