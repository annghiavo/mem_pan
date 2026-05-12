-- ============================================
-- stats_db — stats-service
-- ============================================

-- Stats tổng per user
CREATE TABLE user_stats (
    user_id                 UUID PRIMARY KEY,
    total_cards             INTEGER NOT NULL DEFAULT 0,
    total_reviews           INTEGER NOT NULL DEFAULT 0,
    total_study_time_ms     BIGINT NOT NULL DEFAULT 0,

    current_streak          INTEGER NOT NULL DEFAULT 0,
    longest_streak          INTEGER NOT NULL DEFAULT 0,
    last_studied_date       DATE,

    total_correct           INTEGER NOT NULL DEFAULT 0,
    total_incorrect         INTEGER NOT NULL DEFAULT 0,

    username                VARCHAR(50),
    avatar_url              TEXT,

    updated_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Daily heatmap data
CREATE TABLE daily_stats (
    user_id             UUID NOT NULL,
    study_date          DATE NOT NULL,
    reviews_count       INTEGER NOT NULL DEFAULT 0,
    new_cards_count     INTEGER NOT NULL DEFAULT 0,
    study_time_ms       BIGINT NOT NULL DEFAULT 0,
    correct_count       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, study_date)
);

CREATE INDEX idx_daily_stats_user_date ON daily_stats(user_id, study_date DESC);

-- Stats per deck
CREATE TABLE deck_stats (
    deck_id             UUID PRIMARY KEY,
    user_id             UUID NOT NULL,
    total_cards         INTEGER NOT NULL DEFAULT 0,
    new_cards           INTEGER NOT NULL DEFAULT 0,
    learning_cards      INTEGER NOT NULL DEFAULT 0,
    review_cards        INTEGER NOT NULL DEFAULT 0,
    mastered_cards      INTEGER NOT NULL DEFAULT 0,
    due_today           INTEGER NOT NULL DEFAULT 0,

    deck_name           VARCHAR(200),

    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_deck_stats_user_id ON deck_stats(user_id);

-- Deck progress timeline
CREATE TABLE deck_progress_snapshots (
    deck_id             UUID NOT NULL,
    user_id             UUID NOT NULL,
    snapshot_date       DATE NOT NULL,
    new_count           INTEGER NOT NULL,
    learning_count      INTEGER NOT NULL,
    review_count        INTEGER NOT NULL,
    mastered_count      INTEGER NOT NULL,
    PRIMARY KEY (deck_id, user_id, snapshot_date)
);

CREATE INDEX idx_deck_progress_deck_date ON deck_progress_snapshots(deck_id, snapshot_date DESC);
