ALTER TABLE user_cards DROP CONSTRAINT user_cards_user_id_card_id_deck_id_key;
ALTER TABLE user_cards ADD CONSTRAINT user_cards_user_id_card_id_key UNIQUE (user_id, card_id);
