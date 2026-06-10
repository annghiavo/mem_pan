package payos

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func CreateSignature(data map[string]any, checksumKey string) (string, error) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value, err := signatureValue(data[key])
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	mac := hmac.New(sha256.New, []byte(checksumKey))
	mac.Write([]byte(strings.Join(parts, "&")))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifySignature(data map[string]any, signature, checksumKey string) bool {
	expected, err := CreateSignature(data, checksumKey)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

func signatureValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.10f", v), "0"), "."), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return fmt.Sprint(v), nil
	}
}
