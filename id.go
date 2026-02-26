package entigo

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"sync"

	"github.com/bwmarrin/snowflake"
)

// ID is a snowflake-based unique identifier represented as a 64-bit integer.
// It serializes to JSON as a string to preserve precision in JavaScript clients.
type ID int64

var (
	idNode     *snowflake.Node
	idNodeOnce sync.Once
	idInitErr  error
)

// InitIDGenerator initializes the snowflake node with the given nodeID (0-1023).
// It is safe to call multiple times; only the first call takes effect.
// If not called before NewID, the generator auto-initializes with node 0.
func InitIDGenerator(nodeID int64) error {
	idNodeOnce.Do(func() {
		idNode, idInitErr = snowflake.NewNode(nodeID)
	})
	return idInitErr
}

// ensureInitialized auto-initializes with node 0 if InitIDGenerator was never called.
func ensureInitialized() {
	if idNode == nil {
		_ = InitIDGenerator(0)
	}
}

// NewID generates a new unique snowflake ID.
// If the generator has not been initialized, it auto-initializes with node 0.
func NewID() ID {
	ensureInitialized()
	return ID(idNode.Generate().Int64())
}

// ParseID converts various types to an ID.
// Supported input types: ID, int, int64, string.
func ParseID(idAny any) (ID, error) {
	switch v := idAny.(type) {
	case ID:
		return v, nil
	case int:
		return ID(v), nil
	case int64:
		return ID(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse ID from string %q: %w", v, err)
		}
		return ID(parsed), nil
	default:
		return 0, fmt.Errorf("unsupported type %T for ID conversion", idAny)
	}
}

// IsInvalidID reports whether the given value represents an invalid (non-positive) ID.
// Supported input types: ID, int64, string.
func IsInvalidID(id any) bool {
	switch v := id.(type) {
	case ID:
		return v <= 0
	case int64:
		return v <= 0
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return true
		}
		return parsed <= 0
	default:
		return true
	}
}

// String returns the decimal string representation of the ID.
func (id ID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// Int64 returns the ID as an int64.
func (id ID) Int64() int64 {
	return int64(id)
}

// MarshalJSON encodes the ID as a JSON string to avoid precision loss in JavaScript.
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON decodes the ID from either a JSON string or a JSON number.
func (id *ID) UnmarshalJSON(data []byte) error {
	s := string(data)

	// Strip surrounding quotes if present
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	parsed, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to unmarshal ID from %q: %w", string(data), err)
	}
	*id = ID(parsed)
	return nil
}

// Scan implements sql.Scanner for reading ID values from database drivers.
func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*id = ID(v)
		return nil
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return fmt.Errorf("failed to scan ID from bytes: %w", err)
		}
		*id = ID(parsed)
		return nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to scan ID from string: %w", err)
		}
		*id = ID(parsed)
		return nil
	case nil:
		*id = 0
		return nil
	default:
		return fmt.Errorf("unsupported type %T for ID scan", src)
	}
}

// Value implements driver.Valuer for writing ID values to database drivers.
func (id ID) Value() (driver.Value, error) {
	return int64(id), nil
}
