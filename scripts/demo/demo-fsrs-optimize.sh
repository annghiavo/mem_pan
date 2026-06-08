#!/usr/bin/env bash
# demo-fsrs-optimize.sh — gọi trực tiếp HTTP POST /internal/fsrs/optimize để demo tối ưu hóa FSRS trên prod.
#
# Cách dùng:
#   ./scripts/demo/demo-fsrs-optimize.sh
# Hoặc truyền tham số tùy chỉnh:
#   STUDY_SERVICE_URL=https://study-service-xxxx.run.app CRON_SECRET=custom-secret ./scripts/demo/demo-fsrs-optimize.sh

set -euo pipefail

# Các giá trị mặc định cho môi trường production hiện tại
STUDY_SERVICE_URL="${STUDY_SERVICE_URL:-https://study-service-272885252422.asia-southeast3.run.app}"
CRON_SECRET="${CRON_SECRET:-6a2289792dba0a6bc2a1082e1457f1d7ff9f46e0c74c20f652b808121da88614}"

URI="${STUDY_SERVICE_URL%/}/internal/fsrs/optimize"

echo "=== DEMO TỐI ƯU HÓA TRỌNG SỐ FSRS (PROD) ==="
echo "Endpoint: $URI"
echo "X-Cron-Secret: $CRON_SECRET"
echo "------------------------------------------"
echo "Đang gửi yêu cầu tối ưu hóa..."

# Gọi POST request, nâng timeout lên 10 phút vì tiến trình huấn luyện FSRS tốn nhiều thời gian
response=$(curl -s -w "\n%{http_code}" -X POST "$URI" \
  -H "X-Cron-Secret: $CRON_SECRET" \
  --max-time 600)

# Tách phần body JSON và HTTP status code
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

echo "------------------------------------------"
echo "HTTP Status Code: $http_code"

if [ "$http_code" -eq 200 ]; then
  echo "Tối ưu hóa THÀNH CÔNG!"
  echo "Chi tiết kết quả (JSON):"
  if command -v jq &> /dev/null; then
    echo "$body" | jq .
  else
    echo "$body"
  fi
  echo ""
  echo "Mẹo kiểm tra sự thay đổi trong Database:"
  echo "Chạy câu lệnh SQL sau để kiểm tra bộ trọng số mới được cập nhật (version tăng lên, is_active=true):"
  echo "  SELECT user_id, version, is_active, trained_on_reviews, training_loss, created_at"
  echo "  FROM user_fsrs_weights"
  echo "  ORDER BY created_at DESC LIMIT 5;"
else
  echo "Tối ưu hóa THẤT BẠI!"
  echo "Chi tiết lỗi:"
  echo "$body"
  echo ""
  echo "Gợi ý kiểm tra:"
  echo "  1. Kiểm tra xem user test đã có đủ số review tối thiểu chưa (FSRS_OPTIMIZE_MIN_REVIEWS)."
  echo "  2. Đảm bảo dịch vụ moderation-fsrs-service đã được bật và kết nối thành công."
fi
