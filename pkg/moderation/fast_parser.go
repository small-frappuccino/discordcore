package moderation

import (
	"strconv"
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

func parseBanCommand(payload []byte) (targetID uint64, reason string, deleteDays int) {
	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _, _, _ := jsonparser.Get(value, "name")
		if len(name) == 0 {
			return
		}

		switch b2s(name) {
		case "target_id":
			val, _, _, _ := jsonparser.Get(value, "value")
			targetID, _ = strconv.ParseUint(b2s(val), 10, 64)
		case "reason":
			val, _, _, _ := jsonparser.Get(value, "value")
			reason = b2s(val)
		case "delete_days":
			valInt, _ := jsonparser.GetInt(value, "value")
			deleteDays = int(valInt)
		}
	}, "data", "options")
	return
}

func parseKickCommand(payload []byte) (targetID uint64, reason string) {
	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _, _, _ := jsonparser.Get(value, "name")
		if len(name) == 0 {
			return
		}

		switch b2s(name) {
		case "target_id":
			val, _, _, _ := jsonparser.Get(value, "value")
			targetID, _ = strconv.ParseUint(b2s(val), 10, 64)
		case "reason":
			val, _, _, _ := jsonparser.Get(value, "value")
			reason = b2s(val)
		}
	}, "data", "options")
	return
}

func parseMassBanCommand(payload []byte) (targetIDsStr string, reason string, deleteDays int) {
	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _, _, _ := jsonparser.Get(value, "name")
		if len(name) == 0 {
			return
		}

		switch b2s(name) {
		case "target_ids":
			val, _, _, _ := jsonparser.Get(value, "value")
			targetIDsStr = b2s(val)
		case "reason":
			val, _, _, _ := jsonparser.Get(value, "value")
			reason = b2s(val)
		case "delete_days":
			valInt, _ := jsonparser.GetInt(value, "value")
			deleteDays = int(valInt)
		}
	}, "data", "options")
	return
}
