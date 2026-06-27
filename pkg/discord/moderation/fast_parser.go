package moderation

import (
	"bytes"
	"unsafe"

	"github.com/buger/jsonparser"
)

// b2s converts byte slice to a string without memory allocation.
func b2s(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// extractStringFast navega pelos bytes do JSON puro e extrai o valor.
// Funciona em tempo O(N) (sendo N a profundidade da chave) sem criar structs na Heap.
func extractStringFast(payload []byte, keys ...string) string {
	val, typ, _, err := jsonparser.Get(payload, keys...)
	if err != nil {
		return ""
	}
	if typ == jsonparser.String {
		// If the string contains escape sequences, Get returns the raw bytes including escapes.
		// To be perfectly safe we could unescape, but for IDs/Snowflakes and simple names it's raw.
		// Actually jsonparser.ParseString can be used but it might allocate.
		// For our strict zero-alloc requirement, we assume snowflakes and simple strings don't have escapes.
		return b2s(val)
	}
	return b2s(val)
}

func extractCommandNameFast(payload []byte) string {
	return extractStringFast(payload, "data", "name")
}

func extractOptionString(payload []byte, targetOptionName string) string {
	var result []byte
	targetBytes := []byte(targetOptionName)

	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _, _, _ := jsonparser.Get(value, "name")
		if bytes.Equal(name, targetBytes) {
			result, _, _, _ = jsonparser.Get(value, "value")
		}
	}, "data", "options")

	return b2s(result)
}

func extractOptionInt(payload []byte, targetOptionName string) int {
	var result int64
	targetBytes := []byte(targetOptionName)

	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _, _, _ := jsonparser.Get(value, "name")
		if bytes.Equal(name, targetBytes) {
			result, _ = jsonparser.GetInt(value, "value")
		}
	}, "data", "options")

	return int(result)
}
