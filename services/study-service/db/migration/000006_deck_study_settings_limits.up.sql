ALTER TABLE deck_study_settings
ADD COLUMN new_card_limit INT NOT NULL DEFAULT 20,
ADD COLUMN review_card_limit INT NOT NULL DEFAULT 200;
