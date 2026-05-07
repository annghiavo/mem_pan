-- name: GetDeckStudySettings :one
SELECT * FROM deck_study_settings
WHERE user_id = $1 AND deck_id = $2;

-- name: UpsertDeckStudySettings :one
INSERT INTO deck_study_settings (
    user_id,
    deck_id,
    shuffle_terms,
    text_to_speech,
    answer_with_term,
    answer_with_definition,
    question_type_flashcards,
    question_type_multiple_choice,
    question_type_written,
    strictness_level,
    require_retyping_correct_answer
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id, deck_id) DO UPDATE SET
    shuffle_terms                   = EXCLUDED.shuffle_terms,
    text_to_speech                  = EXCLUDED.text_to_speech,
    answer_with_term                = EXCLUDED.answer_with_term,
    answer_with_definition          = EXCLUDED.answer_with_definition,
    question_type_flashcards        = EXCLUDED.question_type_flashcards,
    question_type_multiple_choice   = EXCLUDED.question_type_multiple_choice,
    question_type_written           = EXCLUDED.question_type_written,
    strictness_level                = EXCLUDED.strictness_level,
    require_retyping_correct_answer = EXCLUDED.require_retyping_correct_answer,
    updated_at                      = CURRENT_TIMESTAMP
RETURNING *;
