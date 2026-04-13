DROP INDEX IF EXISTS idx_revlog_user;
DROP INDEX IF EXISTS idx_revlog_card;
DROP INDEX IF EXISTS idx_folder_decks_deck;
DROP INDEX IF EXISTS idx_pending_reports;
DROP INDEX IF EXISTS idx_deck_active;
DROP INDEX IF EXISTS idx_cards_due;

DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS session_cards;
DROP TABLE IF EXISTS study_sessions;
DROP TABLE IF EXISTS revlogs;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS folder_decks;
DROP TABLE IF EXISTS decks;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS user_fsrs_weights;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS report_target;
DROP TYPE IF EXISTS card_state;
DROP TYPE IF EXISTS content_status;
DROP TYPE IF EXISTS user_role;
