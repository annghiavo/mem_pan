-- ============================================
-- study_db — study-service
-- ============================================

CREATE TYPE card_state AS ENUM ('new', 'learning', 'review', 'relearning');
CREATE TYPE session_status AS ENUM ('ongoing', 'completed', 'abandoned');

CREATE TABLE user_cards (
    user_card_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL,
    card_id             UUID NOT NULL,
    deck_id             UUID NOT NULL,

    state               card_state NOT NULL DEFAULT 'new',
    stability           DOUBLE PRECISION NOT NULL DEFAULT 0,
    difficulty          DOUBLE PRECISION NOT NULL DEFAULT 0,
    reps                INTEGER NOT NULL DEFAULT 0,
    lapses              INTEGER NOT NULL DEFAULT 0,
    scheduled_days      INTEGER NOT NULL DEFAULT 0,
    t_avg               DOUBLE PRECISION NOT NULL DEFAULT 5.0,

    next_review_date    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_review_date    TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (user_id, card_id)
);

CREATE INDEX idx_user_cards_user_id  ON user_cards(user_id);
CREATE INDEX idx_user_cards_deck_id  ON user_cards(deck_id);
CREATE INDEX idx_user_cards_user_deck ON user_cards(user_id, deck_id);
CREATE INDEX idx_user_cards_due      ON user_cards(user_id, next_review_date) WHERE state != 'new';
CREATE INDEX idx_user_cards_state    ON user_cards(user_id, state);

CREATE TABLE study_sessions (
    session_id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID NOT NULL,
    deck_id                 UUID NOT NULL,
    status                  session_status NOT NULL DEFAULT 'ongoing',
    total_cards             INTEGER NOT NULL DEFAULT 0,
    completed_cards         INTEGER NOT NULL DEFAULT 0,
    last_completed_index    INTEGER NOT NULL DEFAULT -1,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at             TIMESTAMPTZ,
    last_accessed_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_study_sessions_user_id     ON study_sessions(user_id);
CREATE INDEX idx_study_sessions_user_status ON study_sessions(user_id, status);
CREATE INDEX idx_study_sessions_deck_id     ON study_sessions(deck_id);

CREATE TABLE session_cards (
    session_id      UUID NOT NULL REFERENCES study_sessions(session_id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    card_id         UUID NOT NULL,
    user_card_id    UUID NOT NULL REFERENCES user_cards(user_card_id),
    reviewed_at     TIMESTAMPTZ,
    rating          SMALLINT,
    PRIMARY KEY (session_id, position)
);

CREATE INDEX idx_session_cards_card_id ON session_cards(card_id);

CREATE TABLE revlogs (
    log_id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL,
    card_id             UUID NOT NULL,
    user_card_id        UUID NOT NULL REFERENCES user_cards(user_card_id),
    session_id          UUID REFERENCES study_sessions(session_id),

    rating              SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 4),
    duration_ms         INTEGER NOT NULL,

    state_before        card_state NOT NULL,
    stability_before    DOUBLE PRECISION NOT NULL,
    difficulty_before   DOUBLE PRECISION NOT NULL,
    elapsed_days        INTEGER NOT NULL,
    scheduled_days      INTEGER NOT NULL,

    state_after         card_state NOT NULL,
    stability_after     DOUBLE PRECISION NOT NULL,
    difficulty_after    DOUBLE PRECISION NOT NULL,

    review_time         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_revlogs_user_id    ON revlogs(user_id);
CREATE INDEX idx_revlogs_card_id    ON revlogs(card_id);
CREATE INDEX idx_revlogs_user_card  ON revlogs(user_card_id, review_time);
CREATE INDEX idx_revlogs_user_time  ON revlogs(user_id, review_time);
CREATE INDEX idx_revlogs_session_id ON revlogs(session_id);

CREATE TABLE user_fsrs_weights (
    user_id         UUID NOT NULL,
    version         INTEGER NOT NULL DEFAULT 1,
    weights         DOUBLE PRECISION[] NOT NULL DEFAULT
        '{0.212,1.2931,2.3065,8.2956,6.4133,0.8334,3.0194,0.001,1.8722,0.1666,0.796,1.4835,0.0614,0.2629,1.6483,0.6014,1.8729,0.5425,0.0912,0.0658,0.1542}'::double precision[],
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    trained_on_reviews INTEGER,
    training_loss   DOUBLE PRECISION,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, version)
);

CREATE INDEX idx_fsrs_weights_active ON user_fsrs_weights(user_id) WHERE is_active = TRUE;
CREATE UNIQUE INDEX idx_fsrs_weights_one_active ON user_fsrs_weights(user_id) WHERE is_active = TRUE;
