#!/usr/bin/env bash
# demo-study-reminder.sh — bắn 1 push nhắc-học FCM ngay tại chỗ để demo bảo vệ.
#
# Luồng thật trong sản phẩm: Cloud Scheduler tick mỗi 15p -> topic cron-study-reminder
# -> notification-service quyết định gửi cho user nào. Script này chỉ thay phần
# "tick" bằng một message tự publish, KHÔNG sửa code service.
#
# MẸO QUAN TRỌNG: handler đọc thời điểm tick từ trường JSON "now" (decodeTickTime trong
# subscriber/handler.go). Nên ta GIẢ thời điểm = đúng giờ gửi (sendTime) của user test,
# để cửa sổ gửi luôn khớp bất kể giờ thật khi bạn demo.
#
# Điều kiện để 1 user thực sự nhận push (scheduler.go):
#   1) user có >= 1 thẻ tới hạn (CountDueForUser > 0)
#   2) localNow nằm trong [sendTime-15p, sendTime]  (script lo phần này)
#      sendTime = (reminder_hour - 2):45 theo timezone user, KHI optimal_hour chưa tính
#   3) chưa gửi "study_reminder" nào trong hôm nay (dedup theo notification_log)
#
# Yêu cầu trước khi chạy:
#   - gcloud đã auth, đúng project (mempan-cac51)
#   - notification-service đang chạy & đã cấu hình STATS_SERVICE_ADDRESS + STUDY_SERVICE_ADDRESS
#   - user test có device đã đăng ký FCM token, và có thẻ tới hạn
#
# Cách dùng:
#   PROJECT=mempan-cac51 USER_TZ=Asia/Bangkok REMINDER_HHMM=21:00 ./demo-study-reminder.sh
set -euo pipefail

PROJECT="${PROJECT:-mempan-cac51}"
TOPIC="${TOPIC:-cron-study-reminder}"
USER_TZ="${USER_TZ:-Asia/Bangkok}"
REMINDER_HHMM="${REMINDER_HHMM:-21:00}"   # = reminder_local_time của user test trong stats-service

# --- Tính sendTime theo timezone user, rồi đổi sang UTC RFC3339 ---------------
# sendTime(local) = (reminder_hour - 2) giờ : 45 phút, hôm nay.
NOW_ISO="$(python3 - "$USER_TZ" "$REMINDER_HHMM" <<'PY'
import sys, datetime
from zoneinfo import ZoneInfo
tz_name, hhmm = sys.argv[1], sys.argv[2]
tz = ZoneInfo(tz_name)
rh = int(hhmm.split(":")[0])
local_now = datetime.datetime.now(tz)
# optimalHour (fallback) = reminder_hour - 1 ; sendTime = optimalHour:00 - 15p = (rh-2):45
send_hour = (rh - 2) % 24
send_local = local_now.replace(hour=send_hour, minute=45, second=0, microsecond=0)
print(send_local.astimezone(datetime.timezone.utc).isoformat().replace("+00:00", "Z"))
PY
)"

echo "User timezone : $USER_TZ"
echo "reminder_local_time: $REMINDER_HHMM  -> sendTime (UTC giả lập 'now'): $NOW_ISO"

# data field = base64( {"now":"<RFC3339>"} ) ; envelope khớp PushHandler
DATA_B64="$(printf '{"now":"%s"}' "$NOW_ISO" | base64 | tr -d '\n')"
BODY="$(printf '{"event_type":"cron.study_reminder","data":"%s"}' "$DATA_B64")"

echo "Publishing tick -> projects/${PROJECT}/topics/${TOPIC}"
echo "  body: $BODY"
gcloud pubsub topics publish "$TOPIC" --project="$PROJECT" --message="$BODY"

cat <<'TIP'

Đã publish. Nếu push KHÔNG bắn, kiểm tra theo thứ tự:
  • user test có thẻ tới hạn? (due_count > 0) — vào học vài thẻ để tạo due.
  • optimal_hour_weekday/weekend trong user_stats khác NULL? Nếu có, nó sẽ ĐÈ
    lên fallback và sendTime sẽ lệch. Tạm set NULL cho user test:
      UPDATE user_stats
         SET optimal_hour_weekday = NULL,
             optimal_hour_weekend = NULL,
             reminder_local_time  = '21:00'
       WHERE user_id = '<USER_ID>';
  • đã gửi study_reminder hôm nay rồi? (dedup) — xóa log để gửi lại:
      DELETE FROM notification_log
       WHERE user_id = '<USER_ID>'
         AND notification_type = 'study_reminder'
         AND created_at >= date_trunc('day', now());
  • xem log notification-service: dòng "[cron] study_reminder: sent to <uid> (N devices)".
TIP
