# Tài liệu Biểu đồ Quan hệ Thực thể (ERD / ERM) — Dự án mem_pan

> **Phạm vi tài liệu:** Tài liệu này mô tả mô hình dữ liệu vật lý (Physical Data Model) của hệ thống **mem_pan**. Vì dự án áp dụng kiến trúc microservice với mô hình **database-per-service**, tài liệu được tổ chức theo cả hai tầng:
>
> 1. **Một sơ đồ logical tổng quan** — cho thấy bức tranh toàn cảnh và các tham chiếu xuyên service.
> 2. **Sáu sơ đồ vật lý chi tiết** — mỗi sơ đồ đặc tả một cơ sở dữ liệu, đầy đủ kiểu dữ liệu, khoá chính, khoá ngoại, và ràng buộc.
>
> Mọi bảng, cột, ràng buộc và index trong tài liệu này đều được trích xuất nguyên văn từ các tệp migration `services/<svc>/db/migration/*.up.sql`.

---

## Mục lục

1. [Quyết định thiết kế: vẽ một ERD chung hay nhiều ERD theo từng database?](#1-quyết-định-thiết-kế-vẽ-một-erd-chung-hay-nhiều-erd-theo-từng-database)
2. [Quy ước trình bày](#2-quy-ước-trình-bày)
3. [Sơ đồ logical tổng quan](#3-sơ-đồ-logical-tổng-quan)
4. [ERD chi tiết theo từng database](#4-erd-chi-tiết-theo-từng-database)
   - 4.1. `auth_db` — auth-service
   - 4.2. `deck_db` — deck-service
   - 4.3. `study_db` — study-service
   - 4.4. `stats_db` — stats-service
   - 4.5. `notif_db` — notification-service
   - 4.6. `admin_db` — admin-service
5. [Bảng tổng hợp tham chiếu xuyên service](#5-bảng-tổng-hợp-tham-chiếu-xuyên-service)
6. [Quy ước đặt tên và các kiểu enum dùng chung](#6-quy-ước-đặt-tên-và-các-kiểu-enum-dùng-chung)

---

## 1. Quyết định thiết kế: vẽ một ERD chung hay nhiều ERD theo từng database?

Với mô hình **database-per-service**, đây là câu hỏi cần trả lời rõ trước khi vẽ. Trong dự án mem_pan, chúng tôi áp dụng **cả hai cách** cho hai mục đích khác nhau:

### Cách 1 — Một ERD vật lý chung (KHÔNG ÁP DỤNG)

> ❌ **Không khuyến nghị** vẽ một sơ đồ duy nhất với tất cả ~22 bảng nối nhau bằng khoá ngoại.

**Lý do:**

1. **Gây hiểu nhầm về ràng buộc vật lý.** Postgres **không hỗ trợ** khoá ngoại xuyên database. Nếu vẽ mũi tên FK liền nét từ `study_db.user_cards.user_id` sang `auth_db.users.user_id`, người đọc sẽ nghĩ có constraint `ON DELETE CASCADE` — điều không tồn tại. Khi xoá user trong `auth_db`, bản ghi mồ côi (orphan) vẫn ở lại các DB khác cho đến khi service tương ứng consume event `UserDeleted` để dọn dẹp.
2. **Vi phạm nguyên tắc bounded context.** Nếu trong sơ đồ thấy `user_cards.user_id` có FK trỏ tới `users`, lập trình viên có khuynh hướng viết câu `JOIN` xuyên DB — điều này không thể (và không nên) thực hiện trong kiến trúc microservice. Mọi truy vấn xuyên service phải đi qua gRPC.
3. **Mất khả năng truyền đạt.** Một sơ đồ có >20 bảng và >40 đường nối sẽ trở thành "mì spaghetti", không còn giá trị tài liệu.

### Cách 2 — ERD vật lý chi tiết PER database (✅ ÁP DỤNG — phần chính)

> ✅ **Khuyến nghị** mỗi service có một ERD riêng, chỉ thể hiện các bảng và FK **trong cùng database**.

**Lợi ích:**

- Phản ánh trung thực ràng buộc vật lý — chỉ vẽ mũi tên FK khi thực sự có `REFERENCES … ON DELETE CASCADE` trong migration.
- Dễ review khi viết migration mới (chỉ cần xem ERD của service đó).
- Tách bạch trách nhiệm — mỗi sơ đồ là tài liệu chính thức cho một service.

### Cách 3 — Một ERD logical tổng quan (✅ BỔ SUNG — đặt ở mục 3)

> ✅ **Khuyến nghị** bổ sung **một** sơ đồ tổng, dùng **mũi tên đứt nét** cho các tham chiếu xuyên DB (logical reference).

**Vai trò:**

- Cho thấy `user_id` xuất hiện ở cả 6 DB — đây là **logical FK** được duy trì bằng nghiệp vụ (event-driven), không phải constraint vật lý.
- Giúp lập trình viên mới hình dung data flow: khi `auth_db.users` thay đổi, các DB khác đồng bộ qua Pub/Sub event nào.
- Đặt ở **đầu tài liệu** trước các ERD chi tiết, đóng vai trò "table of contents trực quan".

### Kết luận

Tài liệu này được tổ chức theo thứ tự:

```
1. Một sơ đồ logical tổng (mục 3) — bird's-eye view, FK đứt nét
2. Sáu sơ đồ vật lý chi tiết (mục 4.1 – 4.6) — đặc tả chính thức
3. Một bảng tổng hợp các cross-DB reference (mục 5) — checklist cho dev
```

---

## 2. Quy ước trình bày

| Ký hiệu | Ý nghĩa |
|---|---|
| `||--o{` | Quan hệ 1-N (một bản ghi cha có nhiều con). |
| `||--||` | Quan hệ 1-1. |
| `}o--o{` | Quan hệ N-N (qua bảng liên kết). |
| Đường liền nét | Khoá ngoại vật lý trong cùng database (`REFERENCES … ON DELETE …`). |
| Đường đứt nét | **Logical reference xuyên database** — không có FK vật lý, ràng buộc duy trì bởi nghiệp vụ. |
| `PK` | Primary Key. |
| `FK` | Foreign Key vật lý. |
| `UK` | Unique Key. |
| `IDX` | Có index để tăng tốc truy vấn. |

**Lưu ý về kiểu dữ liệu:**
- Tất cả ID đều dùng `UUID` (kiểu `uuid` của Postgres, mặc định `gen_random_uuid()`).
- Tất cả timestamp đều dùng `TIMESTAMPTZ` (lưu kèm timezone, chuẩn UTC).
- `ENUM` là Postgres enum type (định nghĩa bằng `CREATE TYPE`).
- `JSONB` cho phép index và truy vấn theo trường JSON nội bộ.

---

## 3. Sơ đồ logical tổng quan

Sơ đồ này thể hiện **tất cả các thực thể chính** của 6 database, với:
- **Đường liền nét**: FK vật lý trong cùng DB.
- **Đường đứt nét**: logical reference xuyên DB — đồng bộ qua Pub/Sub event.

Mục đích: cho phép nhìn toàn cảnh data flow của hệ thống trong một slide. Để đặc tả vật lý chính xác, xem các sơ đồ chi tiết ở mục 4.

```mermaid
erDiagram
    %% ============== auth_db ==============
    USERS {
        UUID user_id PK
        varchar username UK
        varchar email UK
        text password_hash
        user_role role
        bool is_banned
        bool email_verified
        text timezone
        timestamptz last_login_at
        timestamptz created_at
    }
    REFRESH_TOKENS {
        UUID token_id PK
        UUID user_id FK
        text token_hash UK
        timestamptz expires_at
        timestamptz revoked_at
    }
    VERIFICATION_TOKENS {
        UUID token_id PK
        UUID user_id FK
        text token_hash UK
        verification_token_type type
        timestamptz expires_at
        timestamptz used_at
    }

    %% ============== deck_db ==============
    FOLDERS {
        UUID folder_id PK
        UUID user_id "logical→USERS"
        varchar name
        bool is_public
    }
    DECKS {
        UUID deck_id PK
        UUID user_id "logical→USERS"
        varchar name
        bool is_public
        content_status status
        jsonb settings
        int card_count
        UUID cloned_from
    }
    NOTES {
        UUID note_id PK
        UUID user_id "logical→USERS"
        text content_front
        text content_back
        card_language lang_front
        card_language lang_back
        text image_url
    }
    CARDS {
        UUID card_id PK
        UUID deck_id FK
        UUID note_id FK
        int position
    }
    FOLDER_DECKS {
        UUID folder_id PK,FK
        UUID deck_id PK,FK
    }

    %% ============== study_db ==============
    USER_CARDS {
        UUID user_card_id PK
        UUID user_id "logical→USERS"
        UUID card_id "logical→CARDS"
        UUID deck_id "logical→DECKS"
        card_state state
        float stability
        float difficulty
        int reps
        int lapses
        timestamptz next_review_date
    }
    STUDY_SESSIONS {
        UUID session_id PK
        UUID user_id "logical→USERS"
        UUID deck_id "logical→DECKS"
        session_status status
        int total_cards
        int completed_cards
    }
    SESSION_CARDS {
        UUID session_id PK,FK
        int position PK
        UUID card_id "logical→CARDS"
        UUID user_card_id FK
        smallint rating
    }
    REVLOGS {
        UUID log_id PK
        UUID user_card_id FK
        UUID session_id FK
        smallint rating
        int duration_ms
        card_state state_before
        card_state state_after
    }
    USER_FSRS_WEIGHTS {
        UUID user_id PK "logical→USERS"
        int version PK
        "float[]" weights
        bool is_active
    }
    DECK_STUDY_SETTINGS {
        UUID user_id PK "logical→USERS"
        UUID deck_id PK "logical→DECKS"
        bool shuffle_terms
        varchar strictness_level
    }

    %% ============== stats_db ==============
    USER_STATS {
        UUID user_id PK "logical→USERS"
        int total_reviews
        int current_streak
        int longest_streak
        date last_studied_date
        smallint optimal_hour_weekday
        smallint optimal_hour_weekend
        time reminder_local_time
    }
    DAILY_STATS {
        UUID user_id PK "logical→USERS"
        date study_date PK
        int reviews_count
        bigint study_time_ms
    }
    DECK_STATS {
        UUID deck_id PK "logical→DECKS"
        UUID user_id "logical→USERS"
        int total_cards
        int due_today
    }
    DECK_PROGRESS_SNAPSHOTS {
        UUID deck_id PK "logical→DECKS"
        UUID user_id PK "logical→USERS"
        date snapshot_date PK
        int new_count
        int mastered_count
    }
    USER_ACTIVITY_BUCKETS {
        UUID user_id PK "logical→USERS"
        smallint hour_of_day PK
        smallint day_type PK
        int review_count
    }

    %% ============== notif_db ==============
    FCM_TOKENS {
        UUID id PK
        UUID user_id "logical→USERS"
        text token UK
        text device_name
    }
    NOTIFICATION_LOG {
        UUID id PK
        UUID user_id "logical→USERS"
        text notification_type
        text channel
        text recipient
        text status
    }
    EMAIL_TEMPLATES {
        UUID id PK
        text template_key
        text locale
        text subject
        text html_body
        jsonb variables
    }
    EMAIL_TEMPLATE_VERSIONS {
        UUID id PK
        UUID template_id FK
        int version
    }

    %% ============== admin_db ==============
    REPORTS {
        UUID report_id PK
        UUID reporter_id "logical→USERS"
        report_target_type target_type
        UUID target_id "logical→DECKS/USERS"
        report_category reason_category
        report_status status
        UUID assigned_to "logical→USERS (admin)"
        UUID resolved_by "logical→USERS (admin)"
        timestamptz resolved_at
    }
    MODERATION_LOGS {
        UUID log_id PK
        UUID admin_id "logical→USERS (admin)"
        varchar action
        varchar target_type
        UUID target_id
        jsonb metadata
    }

    %% ============== PHYSICAL FK (cùng DB) ==============
    USERS                ||--o{ REFRESH_TOKENS              : "ON DELETE CASCADE"
    USERS                ||--o{ VERIFICATION_TOKENS         : "ON DELETE CASCADE"
    DECKS                ||--o{ CARDS                       : "ON DELETE CASCADE"
    NOTES                ||--o{ CARDS                       : "ON DELETE CASCADE"
    FOLDERS              ||--o{ FOLDER_DECKS                : "ON DELETE CASCADE"
    DECKS                ||--o{ FOLDER_DECKS                : "ON DELETE CASCADE"
    USER_CARDS           ||--o{ SESSION_CARDS               : ""
    STUDY_SESSIONS       ||--o{ SESSION_CARDS               : "ON DELETE CASCADE"
    USER_CARDS           ||--o{ REVLOGS                     : ""
    STUDY_SESSIONS       ||--o{ REVLOGS                     : ""
    EMAIL_TEMPLATES      ||--o{ EMAIL_TEMPLATE_VERSIONS     : "ON DELETE CASCADE"

    %% ============== LOGICAL REFERENCES (xuyên DB) ==============
    USERS                ||..o{ FOLDERS                     : "logical"
    USERS                ||..o{ DECKS                       : "logical"
    USERS                ||..o{ NOTES                       : "logical"
    USERS                ||..o{ USER_CARDS                  : "logical"
    USERS                ||..o{ STUDY_SESSIONS              : "logical"
    USERS                ||..|| USER_STATS                  : "logical 1-1"
    USERS                ||..o{ DAILY_STATS                 : "logical"
    USERS                ||..o{ USER_ACTIVITY_BUCKETS       : "logical"
    USERS                ||..o{ FCM_TOKENS                  : "logical"
    USERS                ||..o{ NOTIFICATION_LOG            : "logical"
    USERS                ||..o{ REPORTS                     : "logical reporter"
    USERS                ||..o{ MODERATION_LOGS             : "logical admin"
    USERS                ||..o{ USER_FSRS_WEIGHTS           : "logical"
    USERS                ||..o{ DECK_STUDY_SETTINGS         : "logical"
    DECKS                ||..o{ USER_CARDS                  : "logical"
    DECKS                ||..o{ STUDY_SESSIONS              : "logical"
    DECKS                ||..|| DECK_STATS                  : "logical 1-1"
    DECKS                ||..o{ DECK_PROGRESS_SNAPSHOTS     : "logical"
    DECKS                ||..o{ DECK_STUDY_SETTINGS         : "logical"
    CARDS                ||..o{ USER_CARDS                  : "logical"
    CARDS                ||..o{ SESSION_CARDS               : "logical"
```

> ⚠ Lưu ý: Mermaid render các quan hệ `||..o{` (logical) bằng đường nét giống `||--o{` (vật lý) — không có cách phân biệt bằng nét vẽ ở phiên bản hiện tại. Phân biệt được thực hiện qua **chú thích chữ** (`"logical"`, `"ON DELETE CASCADE"`) ở cuối mỗi đường nối, và qua việc các trường tham chiếu được đánh dấu `"logical→USERS"` ngay trong định nghĩa cột thay vì `FK`.

---

## 4. ERD chi tiết theo từng database

### 4.1. `auth_db` — auth-service

**Trách nhiệm:** Quản lý người dùng, phiên đăng nhập, token xác minh.

**Custom types:**
```sql
CREATE TYPE user_role AS ENUM ('user', 'admin', 'moderator');
CREATE TYPE verification_token_type AS ENUM ('email_verification', 'password_reset');
```

```mermaid
erDiagram
    USERS ||--o{ REFRESH_TOKENS : "ON DELETE CASCADE"
    USERS ||--o{ VERIFICATION_TOKENS : "ON DELETE CASCADE"

    USERS {
        uuid user_id PK "default gen_random_uuid()"
        varchar username UK "NOT NULL"
        varchar email UK "NOT NULL"
        text password_hash "NOT NULL — bcrypt cost 10"
        varchar full_name "nullable"
        text avatar_url "nullable — Cloudinary URL"
        user_role role "default 'user'"
        bool is_banned "default FALSE"
        timestamptz banned_at "nullable"
        text banned_reason "nullable"
        bool email_verified "default FALSE"
        text timezone "default 'UTC' — IANA tz"
        timestamptz last_login_at "nullable"
        timestamptz created_at "default NOW"
        timestamptz updated_at "default NOW"
    }

    REFRESH_TOKENS {
        uuid token_id PK
        uuid user_id FK "→ users.user_id"
        text token_hash UK "SHA-256 của raw token"
        text user_agent "nullable"
        inet ip_address "nullable"
        timestamptz expires_at "NOT NULL — 168h"
        timestamptz revoked_at "nullable"
        timestamptz created_at
    }

    VERIFICATION_TOKENS {
        uuid token_id PK
        uuid user_id FK "→ users.user_id"
        text token_hash UK "SHA-256"
        verification_token_type type "email_verification|password_reset"
        timestamptz expires_at "24h hoặc 1h"
        timestamptz used_at "nullable — sau khi consume"
        timestamptz created_at
    }
```

**Index đáng chú ý:**
- `idx_refresh_tokens_user_id` — phục vụ "danh sách thiết bị đang đăng nhập".
- `idx_refresh_tokens_token_hash` — lookup khi refresh.
- `idx_users_timezone` — phục vụ cron nhắc học tính giờ địa phương.

**Nguyên tắc bảo mật:**
- **Không lưu raw token** — chỉ lưu hash. Nếu DB bị rò rỉ, attacker không thể tái sử dụng token.
- Refresh token bị **revoke** khi user đăng xuất (`UPDATE … SET revoked_at = NOW()`) thay vì xoá, để giữ audit trail.

### 4.2. `deck_db` — deck-service

**Trách nhiệm:** Quản lý folder, deck, note, card, và quan hệ folder–deck.

**Custom types:**
```sql
CREATE TYPE content_status AS ENUM ('active', 'hidden', 'deleted');
CREATE TYPE card_language AS ENUM (
    'vi','en','es','fr','it','de','ru','ja','ja_romaji',
    'zh_hans','zh_hant','zh_pinyin','ko'
);
```

```mermaid
erDiagram
    FOLDERS      ||--o{ FOLDER_DECKS : "ON DELETE CASCADE"
    DECKS        ||--o{ FOLDER_DECKS : "ON DELETE CASCADE"
    DECKS        ||--o{ CARDS        : "ON DELETE CASCADE"
    NOTES        ||--o{ CARDS        : "ON DELETE CASCADE"

    FOLDERS {
        uuid folder_id PK
        uuid user_id "logical → auth_db.users"
        varchar name "max 100"
        text description "nullable"
        bool is_public "default FALSE"
        timestamptz created_at
        timestamptz updated_at
    }

    DECKS {
        uuid deck_id PK
        uuid user_id "logical → auth_db.users"
        varchar name "max 200"
        text description "nullable"
        bool is_public "default FALSE"
        content_status status "default 'active'"
        jsonb settings "quiz_type,answer_side,strict_typing,..."
        int card_count "denormalized count"
        uuid cloned_from "nullable — deck nguồn nếu clone"
        timestamptz created_at
        timestamptz updated_at
    }

    NOTES {
        uuid note_id PK
        uuid user_id "logical → auth_db.users"
        text content_front "NOT NULL"
        text content_back "NOT NULL"
        text image_url "nullable — Cloudinary URL"
        card_language lang_front "default 'en'"
        card_language lang_back "default 'en'"
        timestamptz created_at
        timestamptz updated_at
    }

    CARDS {
        uuid card_id PK
        uuid user_id "logical → auth_db.users"
        uuid deck_id FK "→ decks.deck_id"
        uuid note_id FK "→ notes.note_id"
        int position "default 0"
        timestamptz created_at
    }

    FOLDER_DECKS {
        uuid folder_id PK,FK "→ folders.folder_id"
        uuid deck_id PK,FK "→ decks.deck_id"
        timestamptz added_at
    }
```

**Index đáng chú ý:**
- `idx_decks_is_public` — **partial index** `WHERE is_public=TRUE AND status='active'`. Chỉ index các deck công khai → giảm dung lượng index, tăng tốc query "khám phá deck".
- `idx_decks_cloned_from` — partial index để truy vết các deck đã được clone.
- `idx_folders_is_public` — partial.
- `idx_cards_deck_note` — unique `(deck_id, note_id)` đảm bảo một note không bị thêm trùng vào cùng một deck.

**Quyết định thiết kế đáng chú ý:**
- **Tách `notes` khỏi `cards`.** Một `note` (cặp mặt trước/sau) có thể được tham chiếu bởi nhiều `card` ở các deck khác nhau khi user clone deck → tránh nhân bản nội dung.
- **`settings` là JSONB**, mặc định:
  ```json
  {
    "quiz_type": "multiple_choice",
    "answer_side": "back",
    "strict_typing": false,
    "partial_correct": true,
    "new_cards_per_day": 20,
    "reviews_per_day": 200
  }
  ```
  Cho phép thêm tuỳ chỉnh học bài mà không cần migration.
- **`card_count` denormalized** — tránh COUNT(*) đắt tiền mỗi lần list deck.

### 4.3. `study_db` — study-service

**Trách nhiệm:** Lưu trạng thái ôn tập của từng thẻ theo từng user (FSRS), phiên học, lịch sử review, trọng số FSRS cá nhân hoá.

**Custom types:**
```sql
CREATE TYPE card_state    AS ENUM ('new', 'learning', 'review', 'relearning');
CREATE TYPE session_status AS ENUM ('ongoing', 'completed', 'abandoned');
```

```mermaid
erDiagram
    USER_CARDS     ||--o{ SESSION_CARDS : ""
    USER_CARDS     ||--o{ REVLOGS       : ""
    STUDY_SESSIONS ||--o{ SESSION_CARDS : "ON DELETE CASCADE"
    STUDY_SESSIONS ||--o{ REVLOGS       : ""

    USER_CARDS {
        uuid user_card_id PK
        uuid user_id "logical → auth_db.users"
        uuid card_id "logical → deck_db.cards"
        uuid deck_id "logical → deck_db.decks"
        card_state state "default 'new'"
        float stability "default 0"
        float difficulty "default 0"
        int reps "default 0"
        int lapses "default 0"
        int scheduled_days "default 0"
        float t_avg "default 5.0 — avg thinking time"
        timestamptz next_review_date "default NOW"
        timestamptz last_review_date "nullable"
        timestamptz created_at
        timestamptz updated_at
        _ unique_constraint "(user_id, card_id, deck_id)"
    }

    STUDY_SESSIONS {
        uuid session_id PK
        uuid user_id "logical → auth_db.users"
        uuid deck_id "logical → deck_db.decks"
        session_status status "default 'ongoing'"
        int total_cards "default 0"
        int completed_cards "default 0"
        int last_completed_index "default -1"
        timestamptz started_at
        timestamptz finished_at "nullable"
        timestamptz last_accessed_at
    }

    SESSION_CARDS {
        uuid session_id PK,FK "→ study_sessions"
        int position PK
        uuid card_id "logical → deck_db.cards"
        uuid user_card_id FK "→ user_cards"
        timestamptz reviewed_at "nullable"
        smallint rating "nullable — 1-4"
    }

    REVLOGS {
        uuid log_id PK
        uuid user_id "logical → auth_db.users"
        uuid card_id "logical → deck_db.cards"
        uuid user_card_id FK "→ user_cards"
        uuid session_id FK "→ study_sessions nullable"
        smallint rating "CHECK BETWEEN 1 AND 4"
        int duration_ms "NOT NULL"
        card_state state_before "trước Schedule()"
        float stability_before
        float difficulty_before
        int elapsed_days
        int scheduled_days
        card_state state_after "sau Schedule()"
        float stability_after
        float difficulty_after
        timestamptz review_time "default NOW"
    }

    USER_FSRS_WEIGHTS {
        uuid user_id PK "logical → auth_db.users"
        int version PK "default 1"
        "float[]" weights "21 phần tử — default = community benchmark"
        bool is_active "default TRUE"
        int trained_on_reviews "nullable"
        float training_loss "nullable"
        timestamptz created_at
    }

    DECK_STUDY_SETTINGS {
        uuid user_id PK "logical → auth_db.users"
        uuid deck_id PK "logical → deck_db.decks"
        bool shuffle_terms "default FALSE"
        bool text_to_speech "default FALSE"
        bool answer_with_term "default TRUE"
        bool answer_with_definition "default TRUE"
        bool question_type_flashcards "default FALSE"
        bool question_type_multiple_choice "default TRUE"
        bool question_type_written "default TRUE"
        varchar strictness_level "default 'flexible' — CHECK in (flexible,strict)"
        bool require_retyping_correct_answer "default FALSE"
        timestamptz created_at
        timestamptz updated_at
    }
```

**Index đáng chú ý:**
- `idx_user_cards_due` — **partial index** `(user_id, next_review_date) WHERE state != 'new'`. Cực kỳ quan trọng — query "lấy thẻ đến hạn" chạy mỗi khi user mở deck, partial index loại trừ thẻ chưa từng học để giảm kích thước index 30–50%.
- `idx_user_cards_state` — phục vụ thống kê theo state.
- `idx_revlogs_user_time` — phục vụ huấn luyện FSRS optimizer (lấy revlog theo user, sắp theo thời gian).
- `idx_fsrs_weights_one_active` — **unique partial index** `WHERE is_active = TRUE` → đảm bảo mỗi user có **đúng một** bản trọng số đang dùng tại mọi thời điểm.

**Quyết định thiết kế:**
- **`unique (user_id, card_id, deck_id)`** chứ không phải `(user_id, card_id)` — vì một card có thể nằm trong nhiều deck (sau clone), tiến độ học của user trên mỗi deck độc lập.
- **`revlogs` lưu cả trạng thái trước và sau** — là nguồn dữ liệu duy nhất để huấn luyện lại trọng số FSRS. Không bao giờ xoá bản ghi của `revlogs`.
- **`user_fsrs_weights` lưu dạng `double precision[]`** thay vì 21 cột riêng — tận dụng kiểu array native của Postgres, dễ insert/select cả vector.

### 4.4. `stats_db` — stats-service

**Trách nhiệm:** Tổng hợp thống kê người dùng và deck (streak, heatmap, tiến độ) bằng cách consume Pub/Sub event từ các service khác.

```mermaid
erDiagram
    USER_STATS {
        uuid user_id PK "logical → auth_db.users"
        int total_cards "default 0"
        int total_reviews "default 0"
        bigint total_study_time_ms "default 0"
        int current_streak "default 0"
        int longest_streak "default 0"
        date last_studied_date "nullable"
        int total_correct "default 0"
        int total_incorrect "default 0"
        varchar username "denormalized từ auth_db"
        text avatar_url "denormalized từ auth_db"
        smallint optimal_hour_weekday "nullable — học AI"
        smallint optimal_hour_weekend "nullable"
        time reminder_local_time "default '21:00:00'"
        timestamptz updated_at
    }

    DAILY_STATS {
        uuid user_id PK "logical → auth_db.users"
        date study_date PK
        int reviews_count "default 0"
        int new_cards_count "default 0"
        bigint study_time_ms "default 0"
        int correct_count "default 0"
    }

    DECK_STATS {
        uuid deck_id PK "logical → deck_db.decks"
        uuid user_id "logical → auth_db.users"
        int total_cards "default 0"
        int new_cards "default 0"
        int learning_cards "default 0"
        int review_cards "default 0"
        int mastered_cards "default 0"
        int due_today "default 0"
        varchar deck_name "denormalized từ deck_db"
        timestamptz updated_at
    }

    DECK_PROGRESS_SNAPSHOTS {
        uuid deck_id PK "logical → deck_db.decks"
        uuid user_id PK "logical → auth_db.users"
        date snapshot_date PK
        int new_count
        int learning_count
        int review_count
        int mastered_count
    }

    USER_ACTIVITY_BUCKETS {
        uuid user_id PK "logical → auth_db.users"
        smallint hour_of_day PK "CHECK 0-23"
        smallint day_type PK "CHECK 0-1 (0=weekday,1=weekend)"
        int review_count "default 0"
        timestamptz updated_at
    }
```

**Quyết định thiết kế:**
- **Denormalize `username`, `avatar_url`, `deck_name`** vào `user_stats` và `deck_stats`. Lý do: bảng xếp hạng và dashboard cần hiển thị tên ngay, không thể `JOIN` xuyên DB. Khi `auth_db.users` đổi username, sự kiện `UserUpdated` đồng bộ trường này.
- **`user_activity_buckets`** lưu histogram theo (giờ, kiểu ngày) — dùng để tính `optimal_hour_weekday/weekend` cho gợi ý giờ học cá nhân hoá (`optimal_hour = argmax(P(user_active | hour, day_type))`).
- **`reminder_local_time`** mặc định `21:00:00` — giờ địa phương để gửi cảnh báo streak.

**Không có FK vật lý nào trong DB này** — tất cả đều là logical reference, vì dữ liệu được derive từ event của các service khác.

### 4.5. `notif_db` — notification-service

**Trách nhiệm:** Lưu FCM token thiết bị, log mọi lần gửi notification, và quản lý template email.

```mermaid
erDiagram
    EMAIL_TEMPLATES ||--o{ EMAIL_TEMPLATE_VERSIONS : "ON DELETE CASCADE"

    FCM_TOKENS {
        uuid id PK
        uuid user_id "logical → auth_db.users"
        text token UK "FCM registration token"
        text device_name "default ''"
        timestamptz created_at
        timestamptz updated_at
    }

    NOTIFICATION_LOG {
        uuid id PK
        uuid user_id "logical → auth_db.users — nullable"
        text notification_type "study_reminder|deck_moderation|welcome|..."
        text channel "fcm | email"
        text recipient "FCM token hoặc email"
        text status "default 'sent' | 'failed'"
        text error_message "nullable"
        timestamptz created_at
    }

    EMAIL_TEMPLATES {
        uuid id PK
        text template_key "welcome,email_verification,password_reset,..."
        text locale "default 'en'"
        text subject "Go template syntax"
        text html_body "Go template syntax"
        text text_body "Go template — plaintext fallback"
        jsonb variables "danh sách tên biến required"
        bool is_active "default TRUE"
        int version "default 1"
        uuid updated_by "logical → auth_db.users — admin"
        timestamptz created_at
        timestamptz updated_at
        _ unique_constraint "(template_key, locale)"
    }

    EMAIL_TEMPLATE_VERSIONS {
        uuid id PK
        uuid template_id FK "→ email_templates"
        int version
        text subject
        text html_body
        text text_body
        uuid updated_by "logical → auth_db.users"
        timestamptz created_at
        _ unique_constraint "(template_id, version)"
    }
```

**Quyết định thiết kế:**
- **`email_templates` không hardcode trong code** — admin có thể sửa template qua admin web mà không cần redeploy. Mỗi lần update sẽ snapshot vào `email_template_versions` để rollback.
- **`fcm_tokens.token UNIQUE`** — cùng một token không được đăng ký cho hai user (FCM đảm bảo token duy nhất per device per app).
- **`notification_log` có `status` + `error_message`** — phục vụ audit và debug, ví dụ khi FCM trả `InvalidRegistration` thì service tự xoá token khỏi `fcm_tokens`.
- **Các template seeded** (qua migration `000002`, `000003`, `000004`): `welcome`, `email_verification`, `password_reset`, `study_reminder`, `report_resolved`, `deck_moderation`.

### 4.6. `admin_db` — admin-service

**Trách nhiệm:** Quản lý báo cáo vi phạm và audit log các hành động kiểm duyệt.

**Custom types:**
```sql
CREATE TYPE report_target_type AS ENUM ('deck', 'user', 'note');
CREATE TYPE report_status      AS ENUM ('pending', 'reviewing', 'resolved', 'dismissed');
CREATE TYPE report_category    AS ENUM (
    'inappropriate_content','copyright_violation','spam',
    'harassment','misinformation','other'
);
```

```mermaid
erDiagram
    REPORTS {
        uuid report_id PK
        uuid reporter_id "logical → auth_db.users"
        report_target_type target_type "deck|user|note"
        uuid target_id "logical → deck_db.decks | auth_db.users | deck_db.notes"
        report_category reason_category
        text description "nullable"
        report_status status "default 'pending'"
        uuid assigned_to "logical → auth_db.users (admin) — nullable"
        text admin_note "nullable"
        varchar resolution "banned|deleted|warned|no_action — nullable"
        uuid resolved_by "logical → auth_db.users (admin) — nullable"
        timestamptz resolved_at "nullable"
        timestamptz created_at
        timestamptz updated_at
    }

    MODERATION_LOGS {
        uuid log_id PK
        uuid admin_id "logical → auth_db.users (admin)"
        varchar action "ban_user|unban_user|hide_deck|resolve_report|..."
        varchar target_type "user|deck|report"
        uuid target_id
        text reason "nullable"
        jsonb metadata "thông tin bổ sung"
        timestamptz created_at
    }
```

**Index đáng chú ý:**
- `idx_reports_status` — `(status, created_at DESC)` để liệt kê queue report cho moderator.
- `idx_reports_assigned` — **partial index** `WHERE status IN ('pending','reviewing')` — chỉ index các report đang xử lý.
- `idx_reports_target` — lookup nhanh "có bao nhiêu report về target này".
- `idx_moderation_logs_admin`, `idx_moderation_logs_target` — phục vụ audit.

**Quyết định thiết kế:**
- **`target_id` không có FK vật lý** vì target có thể trỏ tới `deck_db.decks`, `auth_db.users`, hoặc `deck_db.notes` tuỳ `target_type` — là pattern polymorphic association.
- **`moderation_logs` là append-only** — không bao giờ UPDATE/DELETE, đảm bảo audit trail bất biến.
- **`metadata JSONB`** linh hoạt cho mọi loại action (ví dụ `ban_user` có thể có `{"ban_duration_days": 30}`, `hide_deck` có thể có `{"moderation_score": 0.93, "model_version": "v2"}`).

---

## 5. Bảng tổng hợp tham chiếu xuyên service

Đây là **checklist quan trọng** cho lập trình viên: mỗi khi `auth_db.users` thay đổi (đổi tên, ban, xoá), các bảng nào ở DB khác cần đồng bộ?

### 5.1. Tham chiếu tới `auth_db.users.user_id`

| Database | Bảng | Cột | Vai trò | Đồng bộ qua event |
|---|---|---|---|---|
| deck_db | folders | user_id | Chủ sở hữu folder | `UserDeleted` → soft delete folder |
| deck_db | decks | user_id | Chủ sở hữu deck | `UserDeleted` → cascade xoá |
| deck_db | notes | user_id | Tác giả note | `UserDeleted` → giữ lại (anonymize) |
| deck_db | cards | user_id | Tác giả card | `UserDeleted` → cascade |
| study_db | user_cards | user_id | Tiến độ học | `UserDeleted` → xoá |
| study_db | study_sessions | user_id | Phiên học | `UserDeleted` → xoá |
| study_db | revlogs | user_id | Lịch sử review | `UserDeleted` → giữ lại (anonymize) |
| study_db | user_fsrs_weights | user_id | Trọng số FSRS | `UserDeleted` → xoá |
| study_db | deck_study_settings | user_id | Cấu hình học | `UserDeleted` → xoá |
| stats_db | user_stats | user_id | Thống kê tổng | `UserUpdated` → cập nhật username/avatar; `UserDeleted` → xoá |
| stats_db | daily_stats | user_id | Heatmap | `UserDeleted` → xoá |
| stats_db | user_activity_buckets | user_id | Histogram hoạt động | `UserDeleted` → xoá |
| stats_db | deck_stats | user_id | Thống kê deck | `UserDeleted` → xoá |
| notif_db | fcm_tokens | user_id | Token thiết bị | `UserDeleted` → xoá |
| notif_db | notification_log | user_id | Audit log | `UserDeleted` → giữ lại (anonymize) |
| admin_db | reports | reporter_id, assigned_to, resolved_by | Người báo cáo / xử lý | `UserDeleted` → giữ lại (anonymize) |
| admin_db | moderation_logs | admin_id | Admin thực hiện | KHÔNG xoá (audit trail bất biến) |

### 5.2. Tham chiếu tới `deck_db.decks.deck_id`

| Database | Bảng | Cột | Vai trò | Đồng bộ qua event |
|---|---|---|---|---|
| deck_db | folder_decks | deck_id | (cùng DB — FK vật lý) | `ON DELETE CASCADE` |
| deck_db | cards | deck_id | (cùng DB — FK vật lý) | `ON DELETE CASCADE` |
| study_db | user_cards | deck_id | Tiến độ theo deck | `DeckDeleted` → xoá |
| study_db | study_sessions | deck_id | Phiên học | `DeckDeleted` → mark abandoned |
| study_db | deck_study_settings | deck_id | Cấu hình | `DeckDeleted` → xoá |
| stats_db | deck_stats | deck_id | Thống kê deck | `DeckUpdated` → cập nhật tên; `DeckDeleted` → xoá |
| stats_db | deck_progress_snapshots | deck_id | Lịch sử tiến độ | `DeckDeleted` → giữ lại |

### 5.3. Tham chiếu tới `deck_db.cards.card_id`

| Database | Bảng | Cột | Vai trò | Đồng bộ qua event |
|---|---|---|---|---|
| deck_db | (cùng DB) | — | — | — |
| study_db | user_cards | card_id | Tiến độ thẻ | `CardDeleted` → xoá |
| study_db | session_cards | card_id | Thẻ trong phiên | `CardDeleted` → để session bỏ qua |
| study_db | revlogs | card_id | Lịch sử review | `CardDeleted` → giữ lại |

### 5.4. Mẹo vận hành

> 💡 Khi thiết kế nghiệp vụ xoá user, **không cố gắng xoá đồng bộ tất cả các DB trong một transaction**. Thay vào đó:
> 1. `auth-service` đánh dấu `is_banned=TRUE` (soft delete) ngay lập tức để chặn đăng nhập.
> 2. Publish event `UserDeleted` lên topic `user-events`.
> 3. Mỗi service subscriber tự dọn dữ liệu trong DB của mình theo bảng trên — eventual consistency.
> 4. Nếu một subscriber fail tạm thời, Pub/Sub retry; cuối cùng hệ thống đạt trạng thái nhất quán.

---

## 6. Quy ước đặt tên và các kiểu enum dùng chung

### 6.1. Quy ước cột chuẩn

Tất cả bảng đều có 4 cột chuẩn (trừ bảng append-only như `revlogs`, `moderation_logs`):

| Cột | Kiểu | Vai trò |
|---|---|---|
| `<entity>_id` | `UUID` | Primary key, default `gen_random_uuid()`. Không bao giờ dùng `SERIAL`/`BIGSERIAL` (tránh leak metadata về quy mô). |
| `created_at` | `TIMESTAMPTZ` | Default `CURRENT_TIMESTAMP` / `NOW()`. |
| `updated_at` | `TIMESTAMPTZ` | Default `NOW()`, được cập nhật bằng tay trong query `UPDATE`. |

### 6.2. Tất cả custom enum trong hệ thống

| Database | Enum | Giá trị |
|---|---|---|
| auth_db | `user_role` | `user`, `admin`, `moderator` |
| auth_db | `verification_token_type` | `email_verification`, `password_reset` |
| deck_db | `content_status` | `active`, `hidden`, `deleted` |
| deck_db | `card_language` | `vi`, `en`, `es`, `fr`, `it`, `de`, `ru`, `ja`, `ja_romaji`, `zh_hans`, `zh_hant`, `zh_pinyin`, `ko` |
| study_db | `card_state` | `new`, `learning`, `review`, `relearning` |
| study_db | `session_status` | `ongoing`, `completed`, `abandoned` |
| admin_db | `report_target_type` | `deck`, `user`, `note` |
| admin_db | `report_status` | `pending`, `reviewing`, `resolved`, `dismissed` |
| admin_db | `report_category` | `inappropriate_content`, `copyright_violation`, `spam`, `harassment`, `misinformation`, `other` |

### 6.3. Quy ước index

| Prefix | Ý nghĩa | Ví dụ |
|---|---|---|
| `idx_<table>_<col>` | Index thường | `idx_decks_user_id` |
| `idx_<table>_<col>_<col>` | Composite index | `idx_user_cards_user_deck` |
| `idx_<table>_<col>` + `WHERE …` | Partial index | `idx_decks_is_public WHERE is_public=TRUE AND status='active'` |
| `idx_<table>_one_<col>` + `UNIQUE WHERE` | Unique partial | `idx_fsrs_weights_one_active WHERE is_active=TRUE` |

### 6.4. Quản lý migration

- Công cụ: `golang-migrate/migrate v4`.
- Quy ước file: `NNNNNN_<description>.{up,down}.sql` (ví dụ `000003_folder_visibility.up.sql`).
- Chạy migrate **dùng direct endpoint của Neon** (không có `-pooler`) — xem `doc/tech-stack-report.md` mục 3.4.3.
- Mỗi service có thư mục `db/migration/` riêng và quản lý version độc lập.

---

*Tài liệu lập ngày 2026-05-25. Tham chiếu chéo:*
- *`doc/tech-stack-report.md` — tổng quan công nghệ và lý do chọn Postgres / Neon.*
- *`doc/uml-diagrams.md` — class diagram của domain layer (đối tượng tương ứng các bảng).*
- *`services/<svc>/db/migration/` — nguồn chân lý cho schema của từng database.*
