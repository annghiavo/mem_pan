ALTER TABLE user_cards DROP CONSTRAINT IF EXISTS user_cards_user_id_card_id_deck_id_key;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'user_cards_user_id_card_id_key'
          AND conrelid = 'user_cards'::regclass
    ) THEN
        ALTER TABLE user_cards
            ADD CONSTRAINT user_cards_user_id_card_id_key
            UNIQUE (user_id, card_id);
    END IF;
END$$;
