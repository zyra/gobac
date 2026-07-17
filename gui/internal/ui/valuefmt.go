package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zyra/gobac/gui/internal/session"
)

// FormatValue renders a single decoded property value as display text for
// the object browser's property table. The formatting is keyed off the
// concrete Go type session.Value carries for each application tag (see
// session/live.go's toValue), not the Tag field directly, since that type
// is already unambiguous per tag.
func FormatValue(v session.Value) string {
	switch val := v.Value.(type) {
	case nil:
		return ""
	case bool:
		return strconv.FormatBool(val)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	case string:
		return val
	case []bool:
		parts := make([]string, len(val))
		for i, b := range val {
			parts[i] = strconv.FormatBool(b)
		}
		return "{" + strings.Join(parts, " ") + "}"
	case session.ObjectRef:
		return fmt.Sprintf("%d:%d", val.Type, val.Instance)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ParseWriteValue parses text into the Go value session.WriteRequest.Value
// expects for tag, mirroring the CLI's write-property parsing
// (cmd/gobac/actions/writeprop.go:58-82): bool via strconv.ParseBool, ints
// via strconv.Atoi, Real via ParseFloat(text,32) narrowed to float32,
// Double via ParseFloat(text,64), CharacterString as the text unchanged,
// and Null ignoring text and always returning (nil, nil).
func ParseWriteValue(tag uint8, text string) (interface{}, error) {
	switch tag {
	case 0: // Null
		return nil, nil
	case 1: // Boolean
		return strconv.ParseBool(text)
	case 2, 3, 9: // Unsigned, Signed, Enumerated
		return strconv.Atoi(text)
	case 4: // Real
		v, err := strconv.ParseFloat(text, 32)
		if err != nil {
			return nil, err
		}
		return float32(v), nil
	case 5: // Double
		return strconv.ParseFloat(text, 64)
	case 7: // CharacterString
		return text, nil
	default:
		return nil, fmt.Errorf("unsupported data tag %d", tag)
	}
}
