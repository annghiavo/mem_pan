CREATE TABLE deck_study_settings (
    user_id                         UUID NOT NULL,
    deck_id                         UUID NOT NULL,

    shuffle_terms                   BOOLEAN NOT NULL DEFAULT FALSE,
    text_to_speech                  BOOLEAN NOT NULL DEFAULT FALSE,

    answer_with_term                BOOLEAN NOT NULL DEFAULT TRUE,
    answer_with_definition          BOOLEAN NOT NULL DEFAULT TRUE,

    question_type_flashcards        BOOLEAN NOT NULL DEFAULT FALSE,
    question_type_multiple_choice   BOOLEAN NOT NULL DEFAULT TRUE,
    question_type_written           BOOLEAN NOT NULL DEFAULT TRUE,

    strictness_level                VARCHAR(20) NOT NULL DEFAULT 'flexible',
    require_retyping_correct_answer BOOLEAN NOT NULL DEFAULT FALSE,

    created_at                      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (user_id, deck_id),
    CONSTRAINT strictness_level_check CHECK (strictness_level IN ('flexible', 'strict'))
);
