// Package contract — Cross-language determinism contract (ATOM-CL).
// CL1: Go output == Python output (bit-level).
// CL2: No map iteration without sorting.
// CL3: RFC 8785 canonical JSON only.
// This package is the authoritative spec for Go↔Python hash parity.
package contract

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DeterministicContext binds an event to a specific execution point.
// CL1: Enables bit-level Go↔Python parity.
type DeterministicContext struct {
	TraceID  string
	Tick    uint64
	EventID string
}

// Encode canonicalizes a value per RFC 8785: sorted keys, no whitespace, ASCII.
func Encode(v interface{}) ([]byte, error) {
	// Handle maps: sort keys deterministically.
	switch val := v.(type) {
	case map[string]interface{}:
		return encodeMap(val)
	case []interface{}:
		return encodeSlice(val)
	default:
		return json.Marshal(val)
	}
}

func encodeMap(m map[string]interface{}) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		// Key: RFC 8785 requires double-quoted key.
		sb.WriteByte('"')
		sb.WriteString(k)
		sb.WriteByte('"')
		sb.WriteByte(':')
		v := m[k]
		switch child := v.(type) {
		case map[string]interface{}:
			sub, err := encodeMap(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		case []interface{}:
			sub, err := encodeSlice(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		default:
			sub, err := json.Marshal(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		}
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

func encodeSlice(s []interface{}) ([]byte, error) {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range s {
		if i > 0 {
			sb.WriteByte(',')
		}
		switch child := v.(type) {
		case map[string]interface{}:
			sub, err := encodeMap(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		case []interface{}:
			sub, err := encodeSlice(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		default:
			sub, err := json.Marshal(child)
			if err != nil {
				return nil, err
			}
			sb.Write(sub)
		}
	}
	sb.WriteByte(']')
	return []byte(sb.String()), nil
}

// HashPayload returns SHA-256 of RFC 8785 canonical payload.
func HashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum)
}

// HashEvent computes the deterministic event hash.
// Formula: SHA256(traceID ‖ seq ‖ eventType ‖ HashPayload(payload) ‖ prevHash)
// CL1: Must be bit-identical to Python implementation.
func HashEvent(ctx DeterministicContext, seq uint64, eventType string, payload []byte, prevHash string) string {
	payloadHash := HashPayload(payload)
	raw := fmt.Sprintf("%s|%d|%s|%s|%s",
		ctx.TraceID, seq, eventType, payloadHash, prevHash)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// EncodeDict is a convenience for encoding a map[string]any dict.
func EncodeDict(d map[string]interface{}) ([]byte, error) {
	return Encode(d)
}
