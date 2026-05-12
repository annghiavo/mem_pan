# Study Service – Unit Test Documentation

## Overview

All tests use **gomock** (`go.uber.org/mock`) to mock repository and client dependencies.
No database or network connections are required; tests run in-process and are fast.

Run all tests:

```bash
go test ./internal/...
```

---

## `internal/service` — Business Logic

### `study_service_test.go` (18 tests — original coverage)

| Test | What it covers |
|---|---|
| `TestStartSession_ResumeExisting` | Returns the ongoing session without calling deck-service |
| `TestStartSession_NewSession` | Full new-session path: deck fetch → upsert → select → create |
| `TestStartSession_EmptyDeck` | Deck with zero cards returns `ErrDeckEmpty` |
| `TestGetSession_Success` | Fetches session and card list for the owning user |
| `TestGetSession_Forbidden` | Returns `ErrForbidden` when user ID doesn't match session owner |
| `TestReviewCard_InvalidRating` | Ratings outside 1–4 return `ErrInvalidRating` without touching the DB |
| `TestReviewCard_SessionForbidden` | Rejects review when session belongs to a different user |
| `TestReviewCard_AlreadyFinished` | Rejects review when session status is `completed` |
| `TestReviewCard_CardAlreadyReviewed` | Rejects review when `SessionCard.ReviewedAt` is already set |
| `TestFinishSession_Success` | Marks session completed and returns updated session + cards |
| `TestFinishSession_AlreadyFinished` | Returns `ErrSessionFinished` for an already-completed session |
| `TestFinishSession_Forbidden` | Returns `ErrForbidden` for a different-user session |
| `TestGetDueCards_AllDecks` | Calls `ListDueUserCards` (no deck filter) when deckID is nil |
| `TestGetDueCards_SpecificDeck` | Calls `ListDueUserCardsByDeck` when deckID is provided |
| `TestGetRecentSessionCards_Success` | Returns most-recent session with its card list |
| `TestGetRecentSessionCards_NoSession` | Propagates `ErrSessionNotFound` when no session exists |
| `TestGetRecentDecks_Success` | Maps raw DB rows to `RecentDeck` values |
| `TestGetDeckProgress_Success` | Counts new / learning / review cards correctly |

### `study_service_extra_test.go` (11 additional tests)

| Test | What it covers |
|---|---|
| `TestReviewCard_Success_DefaultWeights` | Full `ReviewCard` happy path using default FSRS weights (weights repo returns error) |
| `TestReviewCard_Success_WithCustomWeights` | Full `ReviewCard` happy path using 21-element custom weights from DB |
| `TestReviewCard_RatingOne_Accepted` | Boundary: rating=1 is the minimum valid value and must be accepted |
| `TestReviewCard_RatingZero_Rejected` | Boundary: rating=0 is below the valid range → `ErrInvalidRating` |
| `TestReviewCard_ElapsedDaysFromLastReview` | `LastReviewDate` is set on the user card; ensures elapsed-day computation proceeds without error |
| `TestStartSession_OnlyNewCards` | Due list is empty but new cards exist; session is created successfully |
| `TestStartSession_NoDueOrNewCards` | Both lists empty after upsert → `ErrDeckEmpty` |
| `TestStartSession_DefaultLimits` | `NewCardsLimit=0` and `ReviewLimit=0` are replaced with package constants (20/200) |
| `TestGetRecentDecks_SortedByRecency` | Verifies the sort-by-recency guarantee with three decks at distinct timestamps |
| `TestGetDeckProgress_RelearningCards` | `relearning` state is counted in `LearnCount`, not `MemorizedCount` |
| `TestGetDeckProgress_EmptyDeck` | Returns zero counts for all categories; `Tags` slice still has three entries |
| `TestGetDeckProgress_TagCardIDs` | Each `ProgressTag` contains the correct card ID list for its state |

### `settings_service_test.go` (10 tests)

| Test | What it covers |
|---|---|
| `TestGetDeckSettings_Success` | Returns settings for an existing deck/user pair |
| `TestGetDeckSettings_NotFound` | Propagates `ErrSettingsNotFound` |
| `TestUpsertDeckSettings_Success_Flexible` | Saves settings with `flexible` strictness |
| `TestUpsertDeckSettings_Success_Strict` | Saves settings with `strict` strictness |
| `TestUpsertDeckSettings_InvalidStrictness` | Rejects unknown strictness values with `ErrInvalidStrictness` |
| `TestUpsertDeckSettings_EmptyStrictness` | Empty string is also rejected with `ErrInvalidStrictness` |
| `TestCheckAnswer_SettingsNotFound_FallsBackToFlexible` | When settings are absent the service defaults to flexible grading |
| `TestCheckAnswer_StrictSettings_Correct` | Exact match passes in strict mode |
| `TestCheckAnswer_StrictSettings_Incorrect` | Punctuation difference fails in strict mode; score is 0.0 |
| `TestCheckAnswer_FlexibleSettings_Typo` | One-edit typo passes for a 5-char word in flexible mode |

---

## `internal/grading` — Answer Grading Logic

### `grading_test.go` (24 tests — pure functions, no mocks)

#### Strict mode (`checkStrict`)

| Test | Scenario |
|---|---|
| `TestCheckAnswer_Strict_ExactMatch` | Identical strings pass |
| `TestCheckAnswer_Strict_CaseInsensitive` | Different case passes |
| `TestCheckAnswer_Strict_LeadingTrailingSpaces` | Trimmed whitespace passes |
| `TestCheckAnswer_Strict_InternalSpaceMismatch` | Internal space mismatch fails |
| `TestCheckAnswer_Strict_PunctuationMismatch` | Trailing punctuation fails |
| `TestCheckAnswer_Strict_WrongAnswer` | Completely wrong answer fails |

#### Flexible mode (`checkFlexible`)

| Test | Scenario |
|---|---|
| `TestCheckAnswer_Flexible_ExactMatch` | Identical strings pass |
| `TestCheckAnswer_Flexible_CaseInsensitive` | Upper-case input passes |
| `TestCheckAnswer_Flexible_PunctuationIgnored` | Trailing `!` is ignored |
| `TestCheckAnswer_Flexible_NormalizedWhitespace` | Multiple spaces collapse to one |
| `TestCheckAnswer_Flexible_ShortWord_ExactRequired` | Words ≤4 chars require exact normalised match (threshold=0) |
| `TestCheckAnswer_Flexible_ShortWord_MatchesExactly` | Short words still pass when correct |
| `TestCheckAnswer_Flexible_MediumWord_OneEditAllowed` | 5–8 char words allow one edit (threshold=1) |
| `TestCheckAnswer_Flexible_MediumWord_TwoEditsFail` | Two edits fail for 5–8 char words |
| `TestCheckAnswer_Flexible_LongWord_TwoEditsAllowed` | ≥9 char words allow two edits (threshold=2) |
| `TestCheckAnswer_Flexible_LongWord_ThreeEditsFail` | Three edits fail for long words |
| `TestCheckAnswer_Flexible_UnknownStrictnessFallsToFlexible` | Any unrecognised strictness string falls through to flexible |

#### Internal helpers

| Test | What it tests |
|---|---|
| `TestNormalize` | Table-driven: lowercase, strip punctuation, collapse whitespace, trim edges |
| `TestLevenshteinThreshold` | All threshold breakpoints: ≤4→0, 5–8→1, ≥9→2 |
| `TestLevenshtein` | Classic pairs: empty strings, identical, one-edit, "kitten"/"sitting", "saturday"/"sunday" |

---

## Mocks

| Mock file | Interface |
|---|---|
| `internal/mock/user_card_repo.go` | `repository.UserCardRepository` |
| `internal/mock/study_session_repo.go` | `repository.StudySessionRepository` |
| `internal/mock/session_card_repo.go` | `repository.SessionCardRepository` |
| `internal/mock/revlog_repo.go` | `repository.RevlogRepository` |
| `internal/mock/fsrs_weights_repo.go` | `repository.FsrsWeightsRepository` |
| `internal/mock/deck_client.go` | `deckclient.Client` |
| `internal/mock/deck_settings_repo.go` | `repository.DeckSettingsRepository` |

The `deck_settings_repo` mock was added alongside the settings service tests.
