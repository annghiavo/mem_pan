CREATE TYPE deck_access_level AS ENUM ('free', 'plus', 'private');
CREATE TYPE deck_plus_status AS ENUM ('none', 'submitted', 'approved', 'rejected', 'suspended');
CREATE TYPE creator_tier AS ENUM ('standard', 'partner');

ALTER TABLE decks
ADD COLUMN access_level deck_access_level NOT NULL DEFAULT 'free',
ADD COLUMN plus_status deck_plus_status NOT NULL DEFAULT 'none',
ADD COLUMN plus_submitted_at TIMESTAMPTZ,
ADD COLUMN plus_approved_at TIMESTAMPTZ,
ADD COLUMN avg_rating NUMERIC(3,2) NOT NULL DEFAULT 0,
ADD COLUMN total_reviews INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_decks_plus_public
    ON decks(access_level, plus_status, status)
    WHERE is_public = TRUE;

CREATE TABLE creator_profiles (
    user_id UUID PRIMARY KEY,
    display_name VARCHAR(100),
    bio TEXT,
    tier creator_tier NOT NULL DEFAULT 'standard',
    follower_count INTEGER NOT NULL DEFAULT 0,
    bank_name TEXT,
    bank_account_number TEXT,
    bank_account_name TEXT,
    bank_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE creator_followers (
    creator_id UUID NOT NULL,
    follower_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (creator_id, follower_id),
    CHECK (creator_id <> follower_id)
);

CREATE INDEX idx_creator_followers_follower ON creator_followers(follower_id);

CREATE TABLE deck_reviews (
    review_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(deck_id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(deck_id, user_id)
);

CREATE INDEX idx_deck_reviews_deck_active
    ON deck_reviews(deck_id)
    WHERE status = 'active';
