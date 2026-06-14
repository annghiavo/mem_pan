package payos

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
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
		parts = append(parts, fmt.Sprintf("%s=%s", encodeURIComponent(key), encodeURIComponent(value)))
	}

	mac := hmac.New(sha256.New, []byte(checksumKey))
	stringToSign := strings.Join(parts, "&")
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifySignature(data map[string]any, signature, checksumKey string) bool {
	expected, err := CreateSignature(data, checksumKey)
	if err != nil {
		return false
	}
	return strings.EqualFold(expected, signature)
}

func VerifyAnySignature(data any, signature, checksumKey string) bool {
	payload, err := mapFromAny(data)
	if err != nil {
		return false
	}
	return VerifySignature(payload, signature, checksumKey)
}

func encodeURIComponent(str string) string {
	escaped := url.QueryEscape(str)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%21", "!")
	escaped = strings.ReplaceAll(escaped, "%27", "'")
	escaped = strings.ReplaceAll(escaped, "%28", "(")
	escaped = strings.ReplaceAll(escaped, "%29", ")")
	escaped = strings.ReplaceAll(escaped, "%2A", "*")
	return escaped
}

func deepSort(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&out); err != nil {
		return v
	}
	return out
}

func signatureValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	sortedVal := deepSort(value)
	if sortedVal == nil {
		return "", nil
	}

	switch v := sortedVal.(type) {
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

func mapFromAny(value any) (map[string]any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
