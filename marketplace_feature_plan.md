# Kế Hoạch Mở Rộng Hệ Thống Flashcard: B2C Subscription Plus

Tài liệu này mô tả kế hoạch nâng cấp hệ thống flashcard hiện tại sang mô hình **B2C Subscription Plus**. Hệ thống không triển khai marketplace C2C mua bán từng deck, không có ví người bán, không có giỏ hàng, và không xử lý giao dịch trực tiếp giữa người học với creator.

Mục tiêu MVP là:

- Người dùng trả phí để mở quyền học các deck Plus.
- Nền tảng tự thu tiền gói Plus qua PayOS.
- Deck Plus được kiểm soát truy cập ở backend.
- Creator đủ điều kiện có thể được ghi nhận doanh thu chia sẻ, nhưng payout và thuế xử lý thủ công trong giai đoạn đầu.

Điểm cần sửa so với bản plan cũ: repo hiện dùng `decks`, `cards`, `notes`, `study_sessions`, `session_cards`, `revlogs`; không có bảng `flashcards`. Vì vậy các thay đổi kỹ thuật phải bám theo schema hiện tại.

---

## I. Mô Hình Kinh Doanh

### 1. Gói Plus

- User mua gói Plus theo tháng/năm.
- Khi Plus còn hiệu lực, user được học toàn bộ deck có `access_level = 'plus'` và `plus_status = 'approved'`.
- Thanh toán MVP chỉ tích hợp PayOS cho chuyển khoản trong nước.

### 2. Creator Revenue Share

- Creator tạo deck chất lượng cao.
- Nền tảng/admin duyệt deck vào thư viện Plus.
- Doanh thu Plus hằng tháng được chia một phần cho creator dựa trên hoạt động học hợp lệ của Plus users.
- MVP chỉ ghi nhận statement và báo cáo doanh thu; chưa tự động chuyển khoản.

## II. Service Ownership Theo Codebase

| Phần việc | Service sở hữu | Ghi chú |
| --- | --- | --- |
| User, role, ban, refresh token | `auth-service` | Không dùng `users.plus_status` làm source of truth duy nhất. |
| Subscription, payment, transaction | Nên tạo `billing-service`; MVP có thể đặt tạm trong `auth-service` | Nếu đặt trong auth, cần giữ module riêng để dễ tách. |
| Deck Plus, preview, card access | `deck-service` | Dùng `decks`, `cards`, `notes`. |
| Study session, FSRS, review logs | `study-service` | Dùng `study_sessions`, `session_cards`, `revlogs`. |
| Dashboard/aggregate metrics | `stats-service` | Nhận event hoặc đọc summary. |
| Duyệt deck, report, appeal | `admin-service` | Mở rộng queue duyệt deck Plus. |
| Search deck Plus | `search-service` | Index thêm access/rating/creator fields. |
| Email/push | `notification-service` | Thông báo payment, deck approved/rejected, statement. |

---

## III. Yêu Cầu Nghiệp Vụ

### 1. Deck Plus Access

Deck cần có trạng thái truy cập rõ ràng:

- `free`: deck public miễn phí.
- `plus`: chỉ Plus user, owner, admin/moderator được học full.
- `private`: chỉ owner.

Quyền học deck Plus phải được kiểm tra ở backend tại các điểm:

- `GET /v1/decks/{deck_id}/cards`
- `GET /v1/cards/{card_id}`
- `POST /v1/study/sessions`
- Các API import/clone/update liên quan đến full content.

Không được chỉ ẩn UI ở frontend rồi vẫn để API trả full card content.

### 2. Deck Metadata Cho User Chưa Có Plus

Với user chưa có Plus, deck-service chỉ trả dữ liệu phục vụ trang giới thiệu deck:

- Non-Plus user xem được metadata deck: tên, mô tả, số card, creator, rating, learner count.
- Không trả full `notes.content_back` hoặc card list đầy đủ cho non-Plus user.

### 3. Verified Rating & Review

Chỉ cho user review khi:

- User có Plus active tại thời điểm học hoặc từng học deck khi Plus còn active.
- User không phải owner của deck.
- User đã hoàn thành ít nhất `max(5 cards, 10% card_count)`.
- Mỗi user chỉ có một review active cho một deck.

Aggregate rating lưu ở `decks.avg_rating`, `decks.total_reviews`, nhưng dữ liệu gốc nằm ở bảng `deck_reviews`.

### 4. Creator Program

**Plus Partner**

Điều kiện đề xuất:

- Tối thiểu 500 followers.
- Tối thiểu 30.000 lượt review FSRS hợp lệ từ Plus users khác, hoặc ngưỡng giờ học tương đương nếu hệ thống đo được active time đáng tin cậy.
- Rating trung bình các deck Plus >= 4.0.
- Không có moderation strike nghiêm trọng trong 90 ngày.
- Có thông tin nhận tiền cơ bản: tên chủ tài khoản, ngân hàng, số tài khoản.

Quyền lợi:

- Được ghi nhận revenue share hằng tháng.
- Có Creator Dashboard.
- Có badge Partner.

### 5. Tài Khoản Nhận Tiền

- Creator nhập tên chủ tài khoản, ngân hàng, số tài khoản.
- Hệ thống kiểm tra định dạng và có thể dùng API VietQR/Napas nếu khả thi để xác minh tên tài khoản.
- Payout vẫn ở trạng thái thủ công: admin đối soát, chuyển khoản ngoài hệ thống, rồi đánh dấu `paid`.

---

## IV. Phân Chia Doanh Thu

### 1. Đối Tượng Chính

- **User Plus:** người dùng có subscription active.
- **Creator:** user sở hữu deck Plus được duyệt.
- **Deck Plus:** deck có `access_level = 'plus'`, `plus_status = 'approved'`.
- **Study Session hợp lệ:** session học thật, không phải bot, không phải creator tự học deck của mình.

### 2. Session Hợp Lệ

Một session được tính vào revenue share khi:

- User có Plus active tại thời điểm học.
- Deck là Plus và đang approved.
- User không phải owner của deck.
- Deck có ít nhất 10 cards.
- User học tối thiểu 5 cards.
- Thời gian học không quá thấp bất thường.
- Không có gap quá 5 phút trong cùng session.
- Không bị anti-cheat flag.

Repo đã có `study_sessions`, `session_cards`, `revlogs`; không tạo lại bảng `study_sessions`. Nếu cần thêm dữ liệu tính điểm, tạo bảng phụ `study_session_metrics`.

### 3. Weighted Score

Gợi ý trọng số:

- `1.5x`: có FSRS rating hợp lệ trong `revlogs`.
- `1.2x`: review đúng chu kỳ spaced repetition.
- `1.0x`: xem/lật card hợp lệ.
- `0.8x`: học lại cùng deck nhiều lần trong ngày.
- `0.2x`: chỉ scroll/xem rất nông.

Score cuối cùng phải do backend tính, frontend chỉ gửi telemetry thô nếu cần.

### 4. Pool Theo Tháng

Mọi gói Plus được quy đổi theo ngày:

- `daily_value = amount_paid / số ngày chu kỳ`
- `month_value = daily_value * số ngày active trong tháng`
- `creator_pool = tổng month_value của user có session hợp lệ * pool_rate`

`pool_rate` đề xuất ban đầu là 50%, nhưng phải để config, không hard-code.

### 5. Phân Bổ

- Tính weighted score theo từng user và creator trong tháng.
- Chia phần pool của user theo tỷ lệ score.
- Creator chỉ nhận khi có tối thiểu 10 Plus learners hợp lệ trong tháng.
- Mỗi creator bị cap tối đa 20% tổng pool tháng.
- Phần không đủ điều kiện hoặc vượt cap đưa vào platform reserve trong MVP. Không rollover nếu chưa có quyết định kế toán rõ ràng.

### 6. Payout MVP

MVP chỉ làm statement, không tự động chuyển tiền.

- Tạo monthly statement.
- Creator xem số dư finalized.
- Creator gửi yêu cầu rút khi đủ ngưỡng, đề xuất 100.000 VND.
- Admin review, chuyển khoản ngoài hệ thống, đánh dấu `paid`.
- Xuất CSV đối soát.

### 7. Đối Soát

Hệ thống lưu dữ liệu phục vụ đối soát:

- Lưu lịch sử payment user.
- Lưu statement creator.
- Export báo cáo tổng hợp theo tháng.
- Lưu trạng thái payout thủ công.

---

## V. Thiết Kế Dữ Liệu Đề Xuất

### 1. Billing

Nên tạo `billing-service`. Nếu muốn nhanh, có thể đặt tạm trong `auth-service` nhưng tách package/module rõ ràng.

```sql
CREATE TYPE subscription_status AS ENUM ('pending', 'active', 'cancelled', 'expired');
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'cancelled', 'refunded');

CREATE TABLE subscriptions (
    subscription_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    plan_code VARCHAR(50) NOT NULL,
    status subscription_status NOT NULL DEFAULT 'pending',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    subscription_id UUID REFERENCES subscriptions(subscription_id),
    provider VARCHAR(20) NOT NULL DEFAULT 'payos',
    provider_payment_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status payment_status NOT NULL DEFAULT 'pending',
    paid_at TIMESTAMPTZ,
    raw_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_payment_id),
    UNIQUE(idempotency_key)
);
```

### 2. Deck Service

```sql
CREATE TYPE deck_access_level AS ENUM ('free', 'plus', 'private');
CREATE TYPE deck_plus_status AS ENUM ('none', 'submitted', 'approved', 'rejected', 'suspended');

ALTER TABLE decks
ADD COLUMN access_level deck_access_level NOT NULL DEFAULT 'free',
ADD COLUMN plus_status deck_plus_status NOT NULL DEFAULT 'none',
ADD COLUMN plus_submitted_at TIMESTAMPTZ,
ADD COLUMN plus_approved_at TIMESTAMPTZ,
ADD COLUMN avg_rating NUMERIC(3,2) NOT NULL DEFAULT 0,
ADD COLUMN total_reviews INTEGER NOT NULL DEFAULT 0;
```

### 3. Creator

```sql
CREATE TYPE creator_tier AS ENUM ('standard', 'partner');

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
```

### 4. Reviews

```sql
CREATE TABLE deck_reviews (
    review_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL,
    user_id UUID NOT NULL,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(deck_id, user_id)
);
```

### 5. Revenue Share

```sql
CREATE TABLE study_session_metrics (
    session_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    deck_id UUID NOT NULL,
    card_views INTEGER NOT NULL DEFAULT 0,
    reviewed_cards INTEGER NOT NULL DEFAULT 0,
    total_active_ms BIGINT NOT NULL DEFAULT 0,
    max_gap_ms INTEGER NOT NULL DEFAULT 0,
    weighted_score NUMERIC(12,4) NOT NULL DEFAULT 0,
    is_revshare_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    invalid_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE monthly_revenue_pools (
    pool_month DATE PRIMARY KEY,
    gross_amount_vnd BIGINT NOT NULL,
    creator_pool_amount_vnd BIGINT NOT NULL,
    platform_amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    finalized_at TIMESTAMPTZ
);

CREATE TABLE creator_earnings (
    earning_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_month DATE NOT NULL,
    creator_id UUID NOT NULL,
    eligible_learners INTEGER NOT NULL,
    weighted_score NUMERIC(14,4) NOT NULL,
    amount_vnd BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pool_month, creator_id)
);
```

---

## VI. API & Job Cần Thêm

### 1. Billing API

- `POST /v1/billing/checkout`: tạo PayOS payment link.
- `POST /v1/billing/webhooks/payos`: nhận webhook, verify chữ ký, xử lý idempotent.
- `GET /v1/billing/subscription/me`: trả trạng thái Plus hiện tại.
- Internal RPC `CheckPlusAccess(user_id)` cho deck/study-service.

### 2. Deck API

- Thêm field `access_level`, `plus_status`, `avg_rating`, `total_reviews` vào `Deck`.
- `ListPublicDecks` hỗ trợ filter Plus/free.
- `GetDeck`, `ListDeckCards`, `GetCard` check quyền Plus.
- Admin API để approve/reject/suspend deck Plus.

### 3. Study API

- `StartSession` check Plus access trước khi tạo session.
- `FinishSession` tính `study_session_metrics`.
- Tận dụng `revlogs.duration_ms` cho review hợp lệ, nhưng không tin tuyệt đối dữ liệu thời gian từ client.

### 4. Cron/Job

- `ExpireSubscriptionsJob`: hết hạn gói Plus.
- `CalculateMonthlyRevShareJob`: tính pool và creator earnings.
- `CreatorPartnerEligibilityJob`: kiểm tra điều kiện lên Partner.
- `DeckRatingAggregateJob`: rebuild rating aggregate khi cần.

---

## VII. Chống Gian Lận & Bảo Mật

### 1. Chống Scraping

- Không trả full card content cho non-Plus user.
- Rate limit theo user/token/IP.
- Batch size thấp khi học deck Plus.
- Log request access để điều tra abuse.
- Search result không expose `notes.content_back` của deck Plus.

### 2. Chống Share Tài Khoản

Repo hiện có `refresh_tokens`, có thể dùng trước thay vì tạo `user_sessions` riêng:

- Lưu user agent, IP, last used.
- Giới hạn số refresh token active cho Plus user.
- Khi vượt limit, revoke token cũ nhất.
- Sau này nếu cần mới tách bảng device/session riêng.

### 3. Chống Creator Farming

- Creator tự học deck của mình không tính.
- Account Plus mới dưới 3 ngày giảm score hoặc không tính.
- Cap contribution mỗi user cho mỗi creator mỗi tháng.
- Nhiều account cùng IP/device/payment fingerprint học cùng creator bị flag.
- Session quá nhanh, đều bất thường, hoặc lặp máy móc bị loại.
- Deck đang bị copyright/report nghiêm trọng có thể tạm giữ earning.

---

## VIII. Roadmap

### Phase 1: Plus Payment MVP

- Thêm billing tables.
- Tích hợp PayOS checkout.
- Webhook idempotent.
- API xem trạng thái subscription.
- Cron expire subscription.

### Phase 2: Plus Deck Access

- Thêm `access_level`, `plus_status` vào `decks`.
- Khóa full card API cho non-Plus.
- `StartSession` check quyền Plus.
- Public deck detail chỉ hiển thị metadata.

### Phase 3: Rating & Review

- Tạo `deck_reviews`.
- Verified review dựa trên Plus + progress/revlogs.
- Aggregate rating vào `decks`.

### Phase 4: Partner Creator

- Tạo `creator_profiles`, `creator_followers`.
- Job kiểm tra điều kiện Partner.
- Dashboard milestone Partner.

### Phase 5: Revenue Share Statement

- Tạo `study_session_metrics`, `monthly_revenue_pools`, `creator_earnings`.
- Tính weighted score.
- Chạy monthly statement.
- Creator xem doanh thu finalized.
- Admin export CSV và mark paid thủ công.

---

## IX. Test Plan Tối Thiểu

- Webhook PayOS duplicate không tạo nhiều subscription.
- Webhook sai signature không activate Plus.
- Subscription hết hạn không học được deck Plus.
- Owner/admin vẫn xem được deck của mình.
- Non-Plus không fetch được full card Plus.
- Plus user tạo study session được.
- Review chỉ tạo được khi đủ điều kiện học.
- Revshare không tính creator tự học deck mình.
- Revshare áp dụng min learners và cap creator.
- Export statement đúng số liệu.

---