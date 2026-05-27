#!/usr/bin/env python3
"""Import toàn bộ deck từ flashcards.json vào mem_pan qua API Gateway.

Cách dùng:
    1. Sửa EMAIL / PASSWORD bên dưới thành tài khoản muốn import vào.
    2. python3 scripts/import_decks.py

Luồng: login -> lấy access_token -> với mỗi deck: tạo deck rồi bulk-create card.
Chỉ dùng thư viện chuẩn (urllib), không cần pip install.
"""

import json
import random
import sys
import time
import urllib.error
import urllib.request

# ── Cấu hình ──────────────────────────────────────────────────────────────
EMAIL = "antit1616@gmail.com"
PASSWORD = "Dung34@@"

GATEWAY = "https://mempan-gateway-3hd0u0cm.uc.gateway.dev"
JSON_PATH = "/Users/annghiavo/Documents/anki_cao/flashcards.json"
IS_PUBLIC = True         # đặt True nếu muốn deck công khai
BULK_CHUNK = 200           # số card gửi mỗi request bulk

# Bật/tắt từng bước. Deck đã import rồi nên mặc định chỉ tạo lượt học ảo.
DO_IMPORT = False          # True = import deck từ JSON
DO_STUDY = True            # True = tạo lượt học ảo cho các deck đã có

STUDY_MAX_CARDS_PER_DECK = 20   # số card "học" mỗi deck (để hiện tiến độ/hoạt động)
# Phân bố rating ngẫu nhiên: 1=Again 2=Hard 3=Good 4=Easy (đa số Good/Easy cho đẹp)
STUDY_RATINGS = [3, 3, 3, 4, 4, 2]
# ────────────────────────────────────────────────────────────────────────────


def api(method, path, token=None, body=None):
    """Gọi API gateway, trả về dict JSON. Ném lỗi nếu status >= 400."""
    url = GATEWAY + path
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode("utf-8")
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")
        raise RuntimeError(f"{method} {path} -> HTTP {e.code}: {detail}") from None


def login(email, password):
    resp = api("POST", "/v1/auth/login", body={"email": email, "password": password})
    token = resp.get("accessToken") or resp.get("access_token")
    if not token:
        raise RuntimeError(f"Login không trả về access_token: {resp}")
    return token


def create_deck(token, name, description=""):
    resp = api("POST", "/v1/decks", token=token,
               body={"name": name, "description": description, "is_public": IS_PUBLIC})
    deck = resp.get("deck") or {}
    deck_id = deck.get("deckId") or deck.get("deck_id")
    if not deck_id:
        raise RuntimeError(f"Tạo deck thất bại: {resp}")
    return deck_id


def bulk_create_cards(token, deck_id, cards):
    """cards: list các dict {front, back}. Gửi theo chunk."""
    total = 0
    for i in range(0, len(cards), BULK_CHUNK):
        chunk = cards[i:i + BULK_CHUNK]
        items = [{
            "content_front": (c.get("front") or "").strip(),
            "content_back": (c.get("back") or "").strip(),
        } for c in chunk]
        resp = api("POST", f"/v1/decks/{deck_id}/cards/bulk", token=token,
                   body={"cards": items})
        total += int(resp.get("created", len(items)))
    return total


def list_all_decks(token):
    """Lấy toàn bộ deck của user, trả list (deck_id, name)."""
    out, page = [], 1
    while True:
        resp = api("GET", f"/v1/decks?page={page}&page_size=100", token=token)
        batch = resp.get("decks") or []
        if not batch:
            break
        for d in batch:
            did = d.get("deckId") or d.get("deck_id")
            if did:
                out.append((did, d.get("name") or ""))
        if len(batch) < 100:
            break
        page += 1
    return out


def study_deck(token, deck_id, max_cards):
    """Tạo lượt học ảo: start -> review từng card -> finish. Trả số card đã học."""
    resp = api("POST", "/v1/study/sessions", token=token,
               body={"deck_id": deck_id, "new_cards_limit": max_cards, "review_limit": max_cards})
    session = resp.get("session") or {}
    session_id = session.get("sessionId") or session.get("session_id")
    if not session_id:
        raise RuntimeError(f"Không tạo được session: {resp}")

    cards = session.get("cards") or []
    reviewed = 0
    for c in cards[:max_cards]:
        card_id = c.get("cardId") or c.get("card_id")
        if not card_id:
            continue
        api("POST", f"/v1/study/sessions/{session_id}/review", token=token,
            body={"card_id": card_id,
                  "rating": random.choice(STUDY_RATINGS),
                  "duration_ms": random.randint(2000, 9000),
                  "timezone": "Asia/Ho_Chi_Minh"})
        reviewed += 1

    api("POST", f"/v1/study/sessions/{session_id}/finish", token=token, body={})
    return reviewed


def run_import(token):
    with open(JSON_PATH, encoding="utf-8") as f:
        decks = json.load(f)
    print(f"Đã đọc {len(decks)} deck từ {JSON_PATH}")

    total_decks = total_cards = 0
    for idx, deck in enumerate(decks, 1):
        name = (deck.get("deck_name") or f"Deck {idx}").strip()
        cards = deck.get("cards") or []
        try:
            deck_id = create_deck(token, name)
            created = bulk_create_cards(token, deck_id, cards)
            total_decks += 1
            total_cards += created
            print(f"[{idx}/{len(decks)}] ✓ '{name}' — {created} card (deck_id={deck_id})")
        except Exception as e:
            print(f"[{idx}/{len(decks)}] ✗ '{name}' — LỖI: {e}")
        time.sleep(0.1)  # nhẹ tay với gateway

    print(f"\nImport xong: {total_decks}/{len(decks)} deck, tổng {total_cards} card.")


def run_study(token):
    decks = list_all_decks(token)
    print(f"Tạo lượt học ảo cho {len(decks)} deck (tối đa {STUDY_MAX_CARDS_PER_DECK} card/deck) ...")

    ok_decks = total_reviewed = 0
    for idx, (deck_id, name) in enumerate(decks, 1):
        try:
            reviewed = study_deck(token, deck_id, STUDY_MAX_CARDS_PER_DECK)
            ok_decks += 1
            total_reviewed += reviewed
            print(f"[{idx}/{len(decks)}] ✓ '{name}' — học {reviewed} card")
        except Exception as e:
            print(f"[{idx}/{len(decks)}] ✗ '{name}' — LỖI: {e}")
        time.sleep(0.1)

    print(f"\nStudy xong: {ok_decks}/{len(decks)} deck, tổng {total_reviewed} lượt review.")


def main():
    if EMAIL == "your-email@example.com" or PASSWORD == "your-password":
        sys.exit("⚠️  Hãy sửa EMAIL và PASSWORD trong file trước khi chạy.")

    print(f"Đang đăng nhập {EMAIL} ...")
    token = login(EMAIL, PASSWORD)
    print("✓ Đăng nhập thành công")

    if DO_IMPORT:
        run_import(token)
    if DO_STUDY:
        run_study(token)


if __name__ == "__main__":
    main()
