# Bài Tập Thực Hành: Microservices Communication

Tài liệu này chứa 2 bài tập giúp bạn làm quen với giao tiếp đồng bộ (gRPC) và bất đồng bộ (Pub/Sub) trong hệ thống MemPan.

## 🛠 Chuẩn Bị
Tạo một nhánh mới để thoải mái thử nghiệm mà không sợ hỏng code chính:
```bash
git checkout main
git pull
git checkout -b practice-grpc-pubsub
```

---

## Bài 1: Giao tiếp Đồng Bộ (gRPC)

**Tình huống:** `stats-service` cần lấy tên của một bộ thẻ (Deck Name) từ `deck-service` để hiển thị trong mục "Bộ thẻ học nhiều nhất".

**Yêu cầu:** Viết một hàm gRPC `GetDeckBasicInfo` ở `deck-service` và gọi nó từ `stats-service`.

### Các bước thực hiện:

#### Bước 1.1: Định nghĩa Protobuf
1. Mở file `proto/deck/deck.proto` (hoặc tạo file tương tự nếu cần).
2. Thêm vào các message và rpc sau:
```protobuf
message GetDeckBasicInfoRequest {
  string deck_id = 1;
}

message GetDeckBasicInfoResponse {
  string deck_name = 1;
  int32 card_count = 2;
}

// Thêm rpc này vào block service DeckService { ... }
// rpc GetDeckBasicInfo(GetDeckBasicInfoRequest) returns (GetDeckBasicInfoResponse);
```
3. Chạy lệnh để biên dịch Protobuf từ thư mục gốc dự án (ví dụ: `make proto`).

#### Bước 1.2: Implement phía Server (`deck-service`)
1. Mở thư mục `services/deck-service/internal/gapi/`.
2. Viết hàm xử lý API `GetDeckBasicInfo`.
```go
func (s *Server) GetDeckBasicInfo(ctx context.Context, req *pb.GetDeckBasicInfoRequest) (*pb.GetDeckBasicInfoResponse, error) {
    // 1. Gọi database để lấy thông tin Deck theo req.GetDeckId()
    // 2. Trả về kết quả pb.GetDeckBasicInfoResponse
    // (Bạn có thể hardcode dữ liệu giả để test cho nhanh)
    return &pb.GetDeckBasicInfoResponse{
        DeckName: "Tên bộ thẻ giả",
        CardCount: 10,
    }, nil
}
```

#### Bước 1.3: Cấu hình kết nối Docker
1. Mở `deploy/docker-compose.yml`.
2. Tìm đến block của `stats-service`. Thêm biến môi trường:
```yaml
    environment:
      - AUTH_SERVICE_ADDRESS=auth-service:9090
      - DECK_SERVICE_ADDRESS=deck-service:9091 # Thêm dòng này
```

#### Bước 1.4: Gọi gRPC từ Client (`stats-service`)
1. Ở `stats-service`, viết một gRPC Client để gọi `deck-service` (bạn có thể tham khảo cách cấu hình của `authclient` trong source code).
2. Gọi hàm Client này trong một API hiện có của `stats-service` và in kết quả ra console log.

---

## Bài 2: Giao tiếp Bất Đồng Bộ (Pub/Sub)

**Tình huống:** Khi một người dùng đạt mốc "Học 100 thẻ" trong `study-service`, hệ thống cần gửi thông báo đẩy qua `notification-service`.

**Yêu cầu:** Bắn một sự kiện `study.milestone_reached` lên Pub/Sub và xử lý nó ở `notification-service`.

### Các bước thực hiện:

#### Bước 2.1: Bắn sự kiện (Publisher - `study-service`)
1. Mở `services/study-service/internal/publisher/publisher.go`.
2. Định nghĩa Struct cho Event:
```go
type MilestoneReachedEvent struct {
    UserID        string `json:"user_id"`
    MilestoneType string `json:"milestone_type"`
    Value         int    `json:"value"`
}
```
3. Thêm hàm `PublishMilestoneReached` và gọi `p.publish(ctx, "study.milestone_reached", event)`.
4. Tìm một chỗ thích hợp trong logic (ví dụ API logic xử lý thẻ/review) để tạo thử Event này và gọi hàm Publish bạn vừa tạo.

#### Bước 2.2: Cấu hình Topic & Subscription
1. Mở file `deploy/pubsub-setup/init.sh`.
2. Tìm phần comment `# notification-service subscriptions`.
3. Thêm dòng sau để thông báo cho Pub/Sub rằng `notification-service` muốn nhận các sự kiện từ topic `study-events`:
```bash
declare_sub study-events notification-study-events-sub "${NOTIFICATION_PUSH_URL}"
```

#### Bước 2.3: Lắng nghe và xử lý (Subscriber - `notification-service`)
1. Mở thư mục `services/notification-service/internal/subscriber/` (tìm file chứa hàm `Dispatch` hoặc `handler`).
2. Định nghĩa lại Struct `MilestoneReachedEvent`.
3. Trong hàm `Dispatch(ctx, eventType, rawData)`, bổ sung một `case`:
```go
    case "study.milestone_reached":
        var event MilestoneReachedEvent
        if err := json.Unmarshal(rawData, &event); err != nil {
            return err
        }
        log.Printf("🎉🎉🎉 CHÚC MỪNG USER %s ĐÃ ĐẠT MỐC %d %s!", event.UserID, event.Value, event.MilestoneType)
        return nil
```

### 💡 Cách kiểm tra (Test)
- Gõ lệnh `docker compose -f deploy/docker-compose.yml up --build` để chạy hệ thống ở local.
- Bắn thử một request vào API review thẻ ở `study-service` (hoặc logic bạn cài đặt).
- Mở terminal thứ 2 và xem log của notification-service bằng lệnh: `docker compose -f deploy/docker-compose.yml logs -f notification-service`.
- Nếu bạn thấy dòng chữ `🎉🎉🎉 CHÚC MỪNG...` hiện lên, bạn đã thành công xuất sắc!

--- 
*Chúc bạn vọc vạch vui vẻ! Nếu gặp lỗi, bạn có thể gửi log lỗi lên đây để tôi hỗ trợ gỡ rối nhé.*
