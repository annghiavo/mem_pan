# Card Image APIs — Tài liệu cho Frontend

> **Base URL (local):** `http://localhost:8000`
> **Auth:** Tất cả API đều yêu cầu header `Authorization: Bearer <access_token>`

---

## Mục lục

1. [Tạo card kèm ảnh](#1-tạo-card-kèm-ảnh)
2. [Cập nhật card kèm ảnh](#2-cập-nhật-card-kèm-ảnh)
3. [Upload ảnh riêng (standalone)](#3-upload-ảnh-riêng-standalone)
4. [Card Object](#4-card-object)
5. [Error Format](#5-error-format)
6. [Lưu ý quan trọng](#6-lưu-ý-quan-trọng)

---

## 1. Tạo card kèm ảnh

```
POST /v1/decks/{deck_id}/cards
Content-Type: multipart/form-data
```

Tạo card mới trong deck, có thể kèm ảnh upload hoặc URL ảnh có sẵn.

### Path Parameters

| Tham số   | Kiểu   | Bắt buộc | Mô tả              |
|-----------|--------|----------|---------------------|
| `deck_id` | string (UUID) | ✅ | ID của deck chứa card |

### Form Fields

| Field          | Kiểu   | Bắt buộc | Mô tả                                                                 |
|----------------|--------|----------|------------------------------------------------------------------------|
| `content_front`| string | ✅       | Nội dung mặt trước của card                                          |
| `content_back` | string | ✅       | Nội dung mặt sau của card                                            |
| `image`        | file   | ❌       | File ảnh upload (sẽ được upload lên Cloudinary). Ưu tiên hơn `image_url` |
| `image_url`    | string | ❌       | URL ảnh có sẵn (chỉ dùng khi không gửi `image` file)                 |
| `position`     | int    | ❌       | Vị trí card trong deck (mặc định: 0)                                 |
| `lang_front`   | string | ❌       | Ngôn ngữ mặt trước (mặc định: `"en"`)                               |
| `lang_back`    | string | ❌       | Ngôn ngữ mặt sau (mặc định: `"en"`)                                 |

### Ví dụ Request

```javascript
const formData = new FormData();
formData.append('content_front', 'Hello');
formData.append('content_back', 'Xin chào');
formData.append('image', imageFile);           // File object từ <input type="file">
formData.append('lang_front', 'en');
formData.append('lang_back', 'vi');

const response = await fetch('/v1/decks/{deck_id}/cards', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    // KHÔNG set Content-Type — browser tự thêm boundary cho multipart
  },
  body: formData,
});
```

### Response `200 OK`

```json
{
  "card": {
    "cardId": "53c1e8d1-c17e-445e-b5b9-c4e5f2bfc9bb",
    "userId": "a1b2c3d4-...",
    "deckId": "d5e6f7a8-...",
    "noteId": "b9c0d1e2-...",
    "position": 0,
    "contentFront": "Hello",
    "contentBack": "Xin chào",
    "imageUrl": "https://res.cloudinary.com/xxx/image/upload/v.../mem_pan/cards/abc123.jpg",
    "langFront": "en",
    "langBack": "vi",
    "createdAt": "2026-05-24T03:01:06Z"
  }
}
```

> **Lưu ý:** Nếu không upload ảnh, field `imageUrl` sẽ là `""` (empty string).

### Các lỗi thường gặp

| Status | Message                                        | Nguyên nhân                     |
|--------|------------------------------------------------|---------------------------------|
| 400    | `content_front and content_back are required`  | Thiếu nội dung bắt buộc        |
| 400    | `invalid deck_id`                              | `deck_id` không phải UUID hợp lệ |
| 400    | `invalid multipart form`                       | Form data bị lỗi               |
| 401    | `unauthorized`                                 | Token không hợp lệ/hết hạn     |
| 404    | `deck not found`                               | Deck không tồn tại             |
| 500    | `failed to upload image`                       | Lỗi upload lên Cloudinary       |

---

## 2. Cập nhật card kèm ảnh

```
PUT /v1/cards/{card_id}
Content-Type: multipart/form-data
```

Cập nhật card hiện có. Chỉ gửi các field cần thay đổi — field không gửi sẽ giữ nguyên giá trị cũ.

### Path Parameters

| Tham số   | Kiểu   | Bắt buộc | Mô tả         |
|-----------|--------|----------|----------------|
| `card_id` | string (UUID) | ✅ | ID của card cần update |

### Form Fields

| Field          | Kiểu   | Bắt buộc | Mô tả                                                                 |
|----------------|--------|----------|------------------------------------------------------------------------|
| `content_front`| string | ❌       | Nội dung mặt trước mới                                               |
| `content_back` | string | ❌       | Nội dung mặt sau mới                                                 |
| `image`        | file   | ❌       | File ảnh mới (upload lên Cloudinary, thay thế ảnh cũ). Ưu tiên hơn `image_url` |
| `image_url`    | string | ❌       | URL ảnh mới (chỉ dùng khi không gửi `image` file)                    |
| `lang_front`   | string | ❌       | Ngôn ngữ mặt trước mới                                              |
| `lang_back`    | string | ❌       | Ngôn ngữ mặt sau mới                                                |

### Ví dụ Request — Chỉ update ảnh

```javascript
const formData = new FormData();
formData.append('image', newImageFile);

const response = await fetch(`/v1/cards/${cardId}`, {
  method: 'PUT',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
  },
  body: formData,
});
```

### Ví dụ Request — Update text + ảnh

```javascript
const formData = new FormData();
formData.append('content_front', 'Updated front');
formData.append('content_back', 'Updated back');
formData.append('image', newImageFile);

const response = await fetch(`/v1/cards/${cardId}`, {
  method: 'PUT',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
  },
  body: formData,
});
```

### Ví dụ Request — Update bằng JSON (không có ảnh file)

Nếu chỉ update text hoặc `image_url` (không upload file), có thể gửi JSON thay vì multipart:

```javascript
const response = await fetch(`/v1/cards/${cardId}`, {
  method: 'PUT',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    content_front: 'Updated front',
    image_url: 'https://example.com/my-image.jpg',
  }),
});
```

### Response `200 OK`

```json
{
  "card": {
    "cardId": "53c1e8d1-c17e-445e-b5b9-c4e5f2bfc9bb",
    "userId": "a1b2c3d4-...",
    "deckId": "d5e6f7a8-...",
    "noteId": "b9c0d1e2-...",
    "position": 0,
    "contentFront": "Updated front",
    "contentBack": "Updated back",
    "imageUrl": "https://res.cloudinary.com/xxx/image/upload/v.../mem_pan/cards/new123.jpg",
    "langFront": "en",
    "langBack": "vi",
    "createdAt": "2026-05-24T03:01:06Z"
  }
}
```

### Các lỗi thường gặp

| Status | Message                  | Nguyên nhân                      |
|--------|--------------------------|----------------------------------|
| 400    | `invalid card_id`        | `card_id` không phải UUID hợp lệ |
| 400    | `invalid multipart form` | Form data bị lỗi                |
| 401    | `unauthorized`           | Token không hợp lệ/hết hạn      |
| 403    | `forbidden`              | Card không thuộc user hiện tại   |
| 404    | `card not found`         | Card không tồn tại              |
| 500    | `failed to upload image` | Lỗi upload lên Cloudinary        |

---

## 3. Upload ảnh riêng (standalone)

```
POST /v1/cards/upload-image
Content-Type: multipart/form-data
```

Upload ảnh lên Cloudinary mà **không gắn vào card nào**. Trả về URL ảnh để frontend dùng sau (ví dụ: gán vào `image_url` khi tạo/update card).

### Form Fields

| Field   | Kiểu | Bắt buộc | Mô tả                          |
|---------|------|----------|---------------------------------|
| `image` | file | ✅       | File ảnh cần upload (max 10 MB) |

### Ví dụ Request

```javascript
const formData = new FormData();
formData.append('image', imageFile);

const response = await fetch('/v1/cards/upload-image', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
  },
  body: formData,
});

const { image_url } = await response.json();
// image_url = "https://res.cloudinary.com/xxx/image/upload/v.../mem_pan/cards/abc123.jpg"
```

### Response `200 OK`

```json
{
  "image_url": "https://res.cloudinary.com/xxx/image/upload/v.../mem_pan/cards/abc123.jpg"
}
```

### Các lỗi thường gặp

| Status | Message                             | Nguyên nhân                          |
|--------|-------------------------------------|--------------------------------------|
| 400    | `image field is required`           | Không gửi file ảnh                   |
| 400    | `request too large or invalid form` | File quá 10MB hoặc form data bị lỗi |
| 401    | `unauthorized`                      | Token không hợp lệ/hết hạn          |
| 500    | `failed to upload image`            | Lỗi upload lên Cloudinary            |
| 503    | `image upload is not configured`    | Server chưa cấu hình Cloudinary      |

---

## 4. Card Object

Response trả về card với **camelCase** (do protojson serialization):

```typescript
interface Card {
  cardId: string;       // UUID
  userId: string;       // UUID — owner
  deckId: string;       // UUID — deck chứa card
  noteId: string;       // UUID — note liên kết
  position: number;     // vị trí trong deck
  contentFront: string; // nội dung mặt trước
  contentBack: string;  // nội dung mặt sau
  imageUrl: string;     // URL ảnh (empty string nếu không có)
  langFront: string;    // ngôn ngữ mặt trước (vd: "en", "vi")
  langBack: string;     // ngôn ngữ mặt sau
  createdAt: string;    // ISO 8601 timestamp
}
```

---

## 5. Error Format

Tất cả lỗi trả về JSON với format:

```json
{
  "message": "mô tả lỗi"
}
```

---

## 6. Lưu ý quan trọng

### Ưu tiên `image` file vs `image_url`

Khi gửi cả `image` (file) và `image_url` (string), **`image` file luôn được ưu tiên**. Server sẽ upload file lên Cloudinary và bỏ qua `image_url`.

Thứ tự ưu tiên:
1. Nếu có `image` file → upload lên Cloudinary → dùng URL trả về
2. Nếu không có `image` file, nhưng có `image_url` → dùng `image_url`
3. Nếu không có cả hai → giữ nguyên (update) hoặc null (create)

### Content-Type tự động

Khi dùng `FormData`, **KHÔNG tự set `Content-Type` header**. Browser sẽ tự thêm `Content-Type: multipart/form-data; boundary=...` với boundary chính xác.

```javascript
// ❌ SAI — không tự set Content-Type
headers: {
  'Content-Type': 'multipart/form-data',
  'Authorization': `Bearer ${token}`,
}

// ✅ ĐÚNG — chỉ set Authorization
headers: {
  'Authorization': `Bearer ${token}`,
}
```

### JSON fallback (Create & Update Card)

API create (`POST /v1/decks/{deck_id}/cards`) và update (`PUT /v1/cards/{card_id}`) hỗ trợ **cả 2 Content-Type**:

| Content-Type          | Khi nào dùng                                     |
|----------------------|--------------------------------------------------|
| `multipart/form-data`| Khi cần upload file ảnh                           |
| `application/json`   | Khi chỉ update text hoặc truyền `image_url` string |

Nếu gửi `Content-Type: application/json`, request sẽ được xử lý qua gRPC gateway với body JSON (snake_case):

```json
{
  "content_front": "Hello",
  "content_back": "Xin chào",
  "image_url": "https://example.com/image.jpg",
  "lang_front": "en",
  "lang_back": "vi"
}
```

### Giới hạn kích thước file

- **Max form size:** 10 MB (áp dụng cho tất cả các API multipart)
- Ảnh được lưu trên **Cloudinary** tại folder `mem_pan/cards/`

### Flow khuyến nghị cho frontend

```
Cách 1: Upload ảnh cùng lúc tạo/update card (đơn giản)
─────────────────────────────────────────────────────────
  FormData { content_front, content_back, image: File }
     ↓
  POST /v1/decks/{deck_id}/cards  hoặc  PUT /v1/cards/{card_id}
     ↓
  Trả về card object đầy đủ (kèm imageUrl)


Cách 2: Upload ảnh trước, rồi tạo/update card (linh hoạt hơn)
──────────────────────────────────────────────────────────────
  Bước 1: POST /v1/cards/upload-image  →  nhận { image_url }
  Bước 2: POST /v1/decks/{deck_id}/cards  với  image_url = URL ở bước 1
```

> **Khuyến nghị dùng Cách 1** cho flow tạo/update card thông thường. Dùng Cách 2 khi cần preview ảnh trước khi save card, hoặc khi tạo nhiều card cùng một ảnh.
