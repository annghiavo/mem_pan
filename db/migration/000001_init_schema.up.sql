CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE user_role AS ENUM ('user', 'admin', 'moderator');
CREATE TYPE content_status AS ENUM ('active', 'hidden', 'deleted_by_admin');
CREATE TYPE card_state AS ENUM ('new', 'learning', 'review', 'relearning');
CREATE TYPE report_target AS ENUM ('deck', 'user');

CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    full_name VARCHAR(100),
    avatar_url TEXT,
    role user_role DEFAULT 'user',
    is_banned BOOLEAN DEFAULT false,
    streak_count INT DEFAULT 0,
    email_verified BOOLEAN DEFAULT false,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_fsrs_weights (
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    version INT DEFAULT 1,
    weights FLOAT[] NOT NULL DEFAULT '{0.4,0.6,2.4,5.8,4.93,0.94,0.86,0.01,1.49,0.14,0.94,2.18,0.05,0.34,1.26,0.29,2.61,0.5,0.0,0.0}',
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, version)
);

CREATE TABLE folders (
    folder_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE decks (
    deck_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_public BOOLEAN DEFAULT false,
    status content_status DEFAULT 'active',
    settings JSONB DEFAULT '{"quiz_type":"multiple_choice","answer_side":"back","partial_correct":true,"strict_typing":false}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ
);

CREATE TABLE folder_decks (
    folder_id UUID REFERENCES folders(folder_id) ON DELETE CASCADE,
    deck_id UUID REFERENCES decks(deck_id) ON DELETE CASCADE,
    PRIMARY KEY(folder_id, deck_id)
);

CREATE TABLE notes (
    note_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    content_front TEXT NOT NULL,
    content_back TEXT NOT NULL,
    image_url TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cards (
    card_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    note_id UUID NOT NULL REFERENCES notes(note_id) ON DELETE CASCADE,
    deck_id UUID NOT NULL REFERENCES decks(deck_id) ON DELETE CASCADE,
    state card_state DEFAULT 'new',
    stability FLOAT DEFAULT 0,
    difficulty FLOAT DEFAULT 0,
    reps INT DEFAULT 0,
    lapses INT DEFAULT 0,
    scheduled_days INT DEFAULT 0,
    next_review_date TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    t_avg FLOAT DEFAULT 5.0,
    last_review_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE revlogs (
    log_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL REFERENCES cards(card_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    rating INT2 NOT NULL,
    duration_ms INT NOT NULL,
    state card_state NOT NULL,
    elapsed_days INT NOT NULL,
    stability_before FLOAT,
    difficulty_before FLOAT,
    review_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE study_sessions (
    session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    deck_id UUID NOT NULL REFERENCES decks(deck_id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'ongoing',
    last_completed_index INT DEFAULT -1,
    last_accessed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE session_cards (
    session_id UUID REFERENCES study_sessions(session_id) ON DELETE CASCADE,
    card_id UUID REFERENCES cards(card_id),
    position INT,
    PRIMARY KEY(session_id, position)
);

CREATE TABLE reports (
    report_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_type report_target NOT NULL,
    target_id UUID NOT NULL,
    reason_category VARCHAR(50),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    admin_note TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_cards_due
ON cards (next_review_date)
WHERE state != 'new';

CREATE INDEX idx_deck_active
ON decks (is_public, status)
WHERE status = 'active';

CREATE INDEX idx_pending_reports
ON reports (status)
WHERE status = 'pending';

CREATE INDEX idx_folder_decks_deck
ON folder_decks(deck_id);

CREATE INDEX idx_revlog_card
ON revlogs(card_id);

CREATE INDEX idx_revlog_user
ON revlogs(user_id);
