UPDATE decks
SET plus_status = 'approved',
    plus_approved_at = COALESCE(plus_approved_at, updated_at, created_at, now()),
    updated_at = now()
WHERE access_level = 'plus'
  AND plus_status = 'none'
  AND is_public = TRUE
  AND status = 'active';
