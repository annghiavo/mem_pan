-- name: CreateDeck :one
INSERT INTO decks (user_id, name, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE deck_id = $1 LIMIT 1;

-- name: ListDecksByUser :many
SELECT * FROM decks
WHERE user_id = $1 AND status != 'deleted'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDecksByUser :one
SELECT COUNT(*) FROM decks WHERE user_id = $1 AND status != 'deleted';

-- name: ListPublicDecks :many
SELECT * FROM decks
WHERE is_public = TRUE
  AND status = 'active'
  AND (access_level != 'plus' OR plus_status = 'approved')
  AND access_level::text = COALESCE(sqlc.narg('access_level')::text, access_level::text)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicDecks :one
SELECT COUNT(*) FROM decks
WHERE is_public = TRUE
  AND status = 'active'
  AND (access_level != 'plus' OR plus_status = 'approved')
  AND access_level::text = COALESCE(sqlc.narg('access_level')::text, access_level::text);

-- name: ListPublicDecksByUser :many
SELECT * FROM decks
WHERE user_id = $1 AND is_public = TRUE AND status = 'active'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPublicDecksByUser :one
SELECT COUNT(*) FROM decks WHERE user_id = $1 AND is_public = TRUE AND status = 'active';

-- name: UpdateDeck :one
UPDATE decks
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE deck_id = sqlc.arg('deck_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: UpdateDeckSettings :one
UPDATE decks
SET settings   = sqlc.arg('settings')::jsonb,
    updated_at = now()
WHERE deck_id = sqlc.arg('deck_id')
RETURNING *;

-- name: UpdateDeckVisibility :one
UPDATE decks
SET is_public  = sqlc.arg('is_public'),
    updated_at = now()
WHERE deck_id = sqlc.arg('deck_id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: SoftDeleteDeck :exec
UPDATE decks
SET status     = 'deleted',
    updated_at = now()
WHERE deck_id = $1 AND user_id = $2;

-- name: AdminUpdateDeckStatus :one
UPDATE decks
SET status     = $2,
    updated_at = now()
WHERE deck_id = $1
RETURNING *;

-- name: UpdateDeckAccessLevel :one
UPDATE decks
SET access_level = $3,
    plus_status = CASE
        WHEN $3::deck_access_level = 'plus' THEN 'submitted'::deck_plus_status
        ELSE 'none'::deck_plus_status
    END,
    plus_submitted_at = CASE
        WHEN $3::deck_access_level = 'plus' THEN now()
        ELSE NULL
    END,
    plus_approved_at = NULL,
    is_public = CASE
        WHEN $3::deck_access_level = 'private' THEN FALSE
        ELSE is_public
    END,
    updated_at = now()
WHERE deck_id = $1 AND user_id = $2
RETURNING *;

-- name: AdminReviewDeckPlus :one
UPDATE decks
SET plus_status = $2,
    plus_approved_at = CASE WHEN $2::deck_plus_status = 'approved' THEN now() ELSE plus_approved_at END,
    access_level = CASE WHEN $2::deck_plus_status = 'approved' THEN 'plus'::deck_access_level ELSE access_level END,
    is_public = CASE WHEN $2::deck_plus_status = 'approved' THEN TRUE ELSE is_public END,
    updated_at = now()
WHERE deck_id = $1
RETURNING *;

-- name: AdminListDecks :many
SELECT * FROM decks
WHERE status::text = COALESCE(sqlc.narg('status_filter')::text, status::text)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountDecks :one
SELECT COUNT(*) FROM decks
WHERE status::text = COALESCE(sqlc.narg('status_filter')::text, status::text);

-- name: IncrementCardCount :exec
UPDATE decks SET card_count = card_count + 1, updated_at = now() WHERE deck_id = $1;

-- name: DecrementCardCount :exec
UPDATE decks SET card_count = GREATEST(card_count - 1, 0), updated_at = now() WHERE deck_id = $1;

-- name: CloneDeck :one
INSERT INTO decks (user_id, name, description, is_public, cloned_from)
VALUES ($1, $2, $3, FALSE, $4)
RETURNING *;

-- name: UpsertCreatorProfile :one
INSERT INTO creator_profiles (
    user_id,
    display_name,
    bio,
    bank_name,
    bank_account_number,
    bank_account_name
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE
SET display_name = COALESCE(EXCLUDED.display_name, creator_profiles.display_name),
    bio = COALESCE(EXCLUDED.bio, creator_profiles.bio),
    bank_name = COALESCE(EXCLUDED.bank_name, creator_profiles.bank_name),
    bank_account_number = COALESCE(EXCLUDED.bank_account_number, creator_profiles.bank_account_number),
    bank_account_name = COALESCE(EXCLUDED.bank_account_name, creator_profiles.bank_account_name),
    updated_at = now()
RETURNING *;

-- name: GetCreatorProfile :one
SELECT * FROM creator_profiles WHERE user_id = $1 LIMIT 1;

-- name: FollowCreator :exec
WITH inserted AS (
    INSERT INTO creator_followers (creator_id, follower_id)
    VALUES ($1, $2)
    ON CONFLICT DO NOTHING
    RETURNING creator_id
)
UPDATE creator_profiles
SET follower_count = follower_count + (SELECT COUNT(*) FROM inserted),
    updated_at = now()
WHERE user_id = $1;

-- name: UnfollowCreator :exec
WITH deleted AS (
    DELETE FROM creator_followers
    WHERE creator_id = $1 AND follower_id = $2
    RETURNING creator_id
)
UPDATE creator_profiles
SET follower_count = GREATEST(follower_count - (SELECT COUNT(*) FROM deleted), 0),
    updated_at = now()
WHERE user_id = $1;

-- name: UpsertDeckReview :one
INSERT INTO deck_reviews (deck_id, user_id, rating, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (deck_id, user_id) DO UPDATE
SET rating = EXCLUDED.rating,
    status = 'active',
    updated_at = now()
RETURNING *;

-- name: RebuildDeckRating :one
UPDATE decks
SET avg_rating = COALESCE((
        SELECT ROUND(AVG(rating)::numeric, 2)
        FROM deck_reviews
        WHERE deck_reviews.deck_id = $1 AND deck_reviews.status = 'active'
    ), 0),
    total_reviews = (
        SELECT COUNT(*)::integer
        FROM deck_reviews
        WHERE deck_reviews.deck_id = $1 AND deck_reviews.status = 'active'
    ),
    updated_at = now()
WHERE deck_id = $1
RETURNING *;

-- name: ListDeckReviews :many
SELECT * FROM deck_reviews
WHERE deck_id = $1 AND status = 'active'
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;
