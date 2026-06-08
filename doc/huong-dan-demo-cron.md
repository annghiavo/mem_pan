# Hướng Dẫn Demo Tính Năng Cron & Tối Ưu Hóa Trọng Số FSRS Khi Bảo Vệ Đồ Án

Tài liệu này hướng dẫn chi tiết cách giả lập và thực hiện demo hai chức năng chạy ngầm (cron jobs) trước Hội đồng:
1. **Thông báo nhắc học & Cảnh báo sắp mất Streak** (qua Pub/Sub và Firebase Cloud Messaging - FCM).
2. **Tối ưu hóa 21 trọng số FSRS** (qua Cloud Run, gRPC và Python FSRS core).

---

## 1. Demo Tính Năng Thông Báo Nhắc Học & Cảnh Báo Streak

### A. Luồng Hoạt Động Thực Tế (Production)
* **Cloud Scheduler** kích hoạt mỗi 15 phút $\rightarrow$ Gửi tín hiệu đến Pub/Sub topic $\rightarrow$ **Notification-service** tiếp nhận $\rightarrow$ Lọc người dùng thỏa mãn điều kiện thời gian và số thẻ cần ôn $\rightarrow$ Gửi push notification qua **FCM**.

---

### B. Các Bước Demo Ngay Lập Tức (Không Cần Chờ 15 Phút)

#### Bước 1: Chuẩn bị dữ liệu tài khoản test (Thực hiện trên DB stats-service và notification-service)
Chạy các câu lệnh SQL dưới đây để đảm bảo tài khoản test của bạn thỏa mãn điều kiện lọc (chưa nhận thông báo hôm nay, giờ học được đặt về mặc định và có thẻ cần ôn):

```sql
-- 1. Reset giờ học về 21:00 và tạm thời tắt giờ học tối ưu (optimal_hour) của user test
UPDATE user_stats
   SET optimal_hour_weekday = NULL,
       optimal_hour_weekend = NULL,
       reminder_local_time  = '21:00'
 WHERE user_id = 'c12adea4-2dc6-4303-be27-4ab1896bb8c8';

-- 2. Xóa lịch sử gửi thông báo nhắc học hôm nay để tránh cơ chế chống gửi trùng (deduplicate)
DELETE FROM notification_log
 WHERE user_id = 'c12adea4-2dc6-4303-be27-4ab1896bb8c8'
   AND notification_type = 'study_reminder'
   AND created_at >= date_trunc('day', now());
```

> [!IMPORTANT]
> **Yêu cầu bắt buộc đối với thiết bị test:**
> 1. Thiết bị giả lập hoặc điện thoại thật phải được mở app và đăng nhập bằng tài khoản test trên để đăng ký thành công **FCM token** với server.
> 2. Tài khoản test cần có **ít nhất 1 thẻ đến hạn** cần ôn tập (`due_count > 0`). Nếu chưa có, hãy tạo hoặc học thử vài thẻ để hệ thống tính toán đến hạn ôn.

#### Bước 2: Chạy Script Giả Lập Trigger
Chạy script giả lập tín hiệu kích hoạt từ máy của bạn. Script này sẽ gửi tín hiệu Pub/Sub lên cloud kèm theo timestamp được giả lập để khớp với múi giờ học 21:00 của bạn:

```bash
# Đảm bảo bạn đang ở thư mục gốc của project mem_pan
cd /Users/annghiavo/Documents/mem_pan

# Chạy script gửi tín hiệu Pub/Sub cho nhắc học
PROJECT=mempan-cac51 USER_TZ=UTC 
REMINDER_HHMM=21:00 ./scripts/demo/demo-study-reminder.sh

```

#### Cách chẩn đoán nếu không nhận được Push:
* Kiểm tra log của `notification-service` trên Cloud Run xem có log: `[cron] study_reminder: sent to <uid> (N devices)` hay không.
* Xác nhận lại ID người dùng và xem bảng `fcm_tokens` trong DB xem thiết bị của bạn đã đăng ký token chưa.

---

## 2. Demo Tính Năng Tối Ưu Hóa Trọng Số FSRS (FSRS Weight Optimization)

### A. Luồng Hoạt Động Thực Tế (Production)
* **Cloud Scheduler** kích hoạt lúc 18:00 UTC hàng ngày $\rightarrow$ Gọi HTTP POST `/internal/fsrs/optimize` của **Study-service** $\rightarrow$ Quét các user có đủ review tích lũy $\rightarrow$ Gọi gRPC sang **Moderation-fsrs-service (Python)** để train lại bộ trọng số $\rightarrow$ Lưu bản ghi trọng số mới với `is_active = true`.

---

### B. Các Bước Demo Chạy Tối Ưu FSRS Ngay Lập Tức

#### Bước 1: Hạ ngưỡng số review tối thiểu trên Cloud Run
Mặc định hệ thống yêu cầu **1000 reviews** mới thực hiện tối ưu. Để demo cho tài khoản test vốn có ít lượt học hơn, hãy hạ ngưỡng xuống thấp (ví dụ: **10 reviews**) bằng cách cập nhật biến môi trường trên Cloud Run:

```bash
# Cập nhật ngưỡng tối thiểu về 10 reviews
gcloud run services update study-service \
  --region=asia-southeast3 \
  --update-env-vars=FSRS_OPTIMIZE_MIN_REVIEWS=10
```

#### Bước 2: Chạy Script Kích Hoạt Tối Ưu Hóa
Chạy script demo có sẵn để gửi request POST bảo mật đến endpoint của `study-service`:

```bash
# Chạy script kích hoạt tiến trình train trọng số
./scripts/demo/demo-fsrs-optimize.sh
```

*Nếu thành công, kết quả trả về sẽ hiển thị thông tin dạng:*
```json
{
  "eligible": 1,
  "optimized": 1,
  "skipped": 0,
  "failed": 0,
  "min_reviews": 10,
  "duration_ms": 11500
}
```

#### Bước 3: Xem Trọng Số Mới Đã Đợc Cập Nhật Trong Database
Chạy câu lệnh SQL sau trên database để trình bày cho Hội đồng thấy phiên bản trọng số mới (`version` tăng lên, `is_active = true`, ghi nhận số reviews được sử dụng để train và độ loss):

```sql
SELECT user_id, version, is_active, trained_on_reviews, training_loss, created_at
FROM user_fsrs_weights
ORDER BY created_at DESC 
LIMIT 5;
```

---

### C. Demo Phần Lõi Thuật Toán (Gọi trực tiếp gRPC sang dịch vụ Python)
Nếu Hội đồng muốn xem cách thức hoạt động thuần túy của bộ tối ưu hóa viết bằng Python (`moderation-fsrs-service`) nhận logs và tính toán trọng số FSRS ra sao:

1. Đảm bảo dịch vụ `moderation-fsrs-service` đang chạy local (gRPC ở port `50051`).
2. Chạy file script Python giả lập việc gửi hàng loạt logs ngẫu nhiên qua gRPC:
   ```bash
   cd services/moderation-fsrs-service
   .venv/bin/python ../../scripts/demo/demo-optimize-weights.py
   ```
*Script sẽ in ra console chi tiết 21 trọng số (`w[0..20]`) và giá trị loss sau khi huấn luyện.*
