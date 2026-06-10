package payos

import "testing"

func TestCreateSignatureSortsFields(t *testing.T) {
	got, err := CreateSignature(map[string]any{
		"returnUrl":   "https://example.com/return",
		"orderCode":   int64(123456),
		"description": "ORDER123",
		"cancelUrl":   "https://example.com/cancel",
		"amount":      int64(50000),
	}, "secret")
	if err != nil {
		t.Fatal(err)
	}

	const want = "b9c208495fda1d5748bae5dc8ebb1ff745aab6cfaa19a9808ffb0e29d808e0e3"
	if got != want {
		t.Fatalf("signature mismatch: got %s, want %s", got, want)
	}
}

func TestVerifySignatureRejectsMismatch(t *testing.T) {
	ok := VerifySignature(map[string]any{"amount": int64(50000)}, "bad-signature", "secret")
	if ok {
		t.Fatal("expected invalid signature")
	}
}
