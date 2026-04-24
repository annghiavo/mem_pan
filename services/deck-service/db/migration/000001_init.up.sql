-- ============================================
-- deck_db — deck-service
-- ============================================

CREATE TYPE content_status AS ENUM ('active', 'hidden', 'deleted');

-- Folders
CREATE TABLE folders (
    folder_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    name            VARCHAR(100) NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_folders_user_id ON folders(user_id);

-- Decks
CREATE TABLE decks (
    deck_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    is_public       BOOLEAN NOT NULL DEFAULT FALSE,
    status          content_status NOT NULL DEFAULT 'active',
    settings        JSONB NOT NULL DEFAULT '{
        "quiz_type": "multiple_choice",
        "answer_side": "back",
        "strict_typing": false,
        "partial_correct": true,
        "new_cards_per_day": 20,
        "reviews_per_day": 200
    }'::jsonb,
    card_count      INTEGER NOT NULL DEFAULT 0,
    cloned_from     UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_decks_user_id ON decks(user_id);
CREATE INDEX idx_decks_is_public ON decks(is_public, status) WHERE is_public = TRUE AND status = 'active';
CREATE INDEX idx_decks_cloned_from ON decks(cloned_from) WHERE cloned_from IS NOT NULL;

-- Notes
CREATE TABLE notes (
    note_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    content_front   TEXT NOT NULL,
    content_back    TEXT NOT NULL,
    image_url       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notes_user_id ON notes(user_id);

-- Cards
CREATE TABLE cards (
    card_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    deck_id         UUID NOT NULL REFERENCES decks(deck_id) ON DELETE CASCADE,
    note_id         UUID NOT NULL REFERENCES notes(note_id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cards_deck_id ON cards(deck_id);
CREATE INDEX idx_cards_note_id ON cards(note_id);
CREATE INDEX idx_cards_user_id ON cards(user_id);
CREATE UNIQUE INDEX idx_cards_deck_note ON cards(deck_id, note_id);

-- Folder-Deck many-to-many
CREATE TABLE folder_decks (
    folder_id       UUID NOT NULL REFERENCES folders(folder_id) ON DELETE CASCADE,
    deck_id         UUID NOT NULL REFERENCES decks(deck_id) ON DELETE CASCADE,
    added_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (folder_id, deck_id)
);

CREATE INDEX idx_folder_decks_deck_id ON folder_decks(deck_id);
