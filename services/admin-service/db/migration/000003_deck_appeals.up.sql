-- Deck appeals: lets a user contest a deck deletion (manual or auto-moderation).
-- A row is created when the deck is deleted (status='pending'); the user submits
-- the appeal via a one-time token in their deletion email (status='submitted');
-- an admin then approves or rejects.

CREATE TYPE appeal_status AS ENUM ('pending', 'submitted', 'approved', 'rejected');

CREATE TABLE deck_appeals (
    appeal_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token              VARCHAR(80) NOT NULL UNIQUE,
    deck_id            UUID NOT NULL UNIQUE,            -- one appeal per deck
    user_id            UUID NOT NULL,                   -- deck owner
    deck_name          TEXT NOT NULL,
    moderation_reason  TEXT NOT NULL DEFAULT '',
    status             appeal_status NOT NULL DEFAULT 'pending',

    -- Filled when the user submits the appeal.
    user_message       TEXT,
    submitted_at       TIMESTAMPTZ,

    -- Filled when an admin decides on the submitted appeal.
    decided_by         UUID,
    decision_note      TEXT,
    decided_at         TIMESTAMPTZ,

    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_deck_appeals_status ON deck_appeals(status, created_at DESC);
CREATE INDEX idx_deck_appeals_user   ON deck_appeals(user_id);
