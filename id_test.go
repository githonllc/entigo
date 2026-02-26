package entigo

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// resetIDGenerator resets the global ID generator state so that each test
// can initialize it independently. This is necessary because sync.Once
// prevents re-initialization within the same process.
func resetIDGenerator() {
	idNode = nil
	idNodeOnce = sync.Once{}
	idInitErr = nil
}

func TestInitIDGenerator(t *testing.T) {
	resetIDGenerator()

	err := InitIDGenerator(1)
	assert.NoError(t, err, "InitIDGenerator with valid node ID should succeed")
	assert.NotNil(t, idNode, "idNode should be initialized after InitIDGenerator")

	// Second call should be a no-op due to sync.Once
	err = InitIDGenerator(2)
	assert.NoError(t, err, "second InitIDGenerator call should not return error")
}

func TestNewID(t *testing.T) {
	resetIDGenerator()

	// NewID should auto-initialize and generate a valid positive ID
	id1 := NewID()
	assert.True(t, id1 > 0, "generated ID should be positive")

	id2 := NewID()
	assert.True(t, id2 > 0, "generated ID should be positive")
	assert.NotEqual(t, id1, id2, "two generated IDs should be unique")

	// Generate a batch of IDs and verify uniqueness
	seen := make(map[ID]bool)
	for i := 0; i < 1000; i++ {
		id := NewID()
		assert.False(t, seen[id], "ID should be unique, got duplicate: %d", id)
		seen[id] = true
	}
}

func TestParseID(t *testing.T) {
	resetIDGenerator()

	tests := []struct {
		name    string
		input   any
		want    ID
		wantErr bool
	}{
		{
			name:  "parse from ID type",
			input: ID(12345),
			want:  ID(12345),
		},
		{
			name:  "parse from int",
			input: 42,
			want:  ID(42),
		},
		{
			name:  "parse from int64",
			input: int64(99999),
			want:  ID(99999),
		},
		{
			name:  "parse from valid string",
			input: "67890",
			want:  ID(67890),
		},
		{
			name:    "parse from invalid string",
			input:   "not_a_number",
			wantErr: true,
		},
		{
			name:    "parse from unsupported type (float64)",
			input:   float64(1.5),
			wantErr: true,
		},
		{
			name:    "parse from unsupported type (bool)",
			input:   true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsInvalidID(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{
			name:  "valid ID type",
			input: ID(12345),
			want:  false,
		},
		{
			name:  "zero ID type",
			input: ID(0),
			want:  true,
		},
		{
			name:  "negative ID type",
			input: ID(-1),
			want:  true,
		},
		{
			name:  "valid int64",
			input: int64(100),
			want:  false,
		},
		{
			name:  "zero int64",
			input: int64(0),
			want:  true,
		},
		{
			name:  "negative int64",
			input: int64(-42),
			want:  true,
		},
		{
			name:  "valid string",
			input: "12345",
			want:  false,
		},
		{
			name:  "zero string",
			input: "0",
			want:  true,
		},
		{
			name:  "invalid string",
			input: "abc",
			want:  true,
		},
		{
			name:  "empty string",
			input: "",
			want:  true,
		},
		{
			name:  "unsupported type (float64)",
			input: float64(1.0),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInvalidID(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIDMarshalJSON(t *testing.T) {
	id := ID(1234567890)
	data, err := json.Marshal(id)
	assert.NoError(t, err)
	// ID should be marshaled as a JSON string to avoid JavaScript precision loss
	assert.Equal(t, `"1234567890"`, string(data))
}

func TestIDUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    ID
		wantErr bool
	}{
		{
			name: "from JSON string",
			json: `"1234567890"`,
			want: ID(1234567890),
		},
		{
			name: "from JSON number",
			json: `9876543210`,
			want: ID(9876543210),
		},
		{
			name:    "from invalid JSON string",
			json:    `"not_a_number"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id ID
			err := json.Unmarshal([]byte(tt.json), &id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, id)
			}
		})
	}

	// Round-trip test: marshal then unmarshal
	t.Run("round trip", func(t *testing.T) {
		original := ID(9876543210)
		data, err := json.Marshal(original)
		assert.NoError(t, err)

		var decoded ID
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, original, decoded)
	})
}

func TestIDScanValue(t *testing.T) {
	// Test Scan from int64
	t.Run("scan from int64", func(t *testing.T) {
		var id ID
		err := id.Scan(int64(12345))
		assert.NoError(t, err)
		assert.Equal(t, ID(12345), id)
	})

	// Test Scan from []byte
	t.Run("scan from bytes", func(t *testing.T) {
		var id ID
		err := id.Scan([]byte("67890"))
		assert.NoError(t, err)
		assert.Equal(t, ID(67890), id)
	})

	// Test Scan from string
	t.Run("scan from string", func(t *testing.T) {
		var id ID
		err := id.Scan("99999")
		assert.NoError(t, err)
		assert.Equal(t, ID(99999), id)
	})

	// Test Scan from nil
	t.Run("scan from nil", func(t *testing.T) {
		var id ID
		err := id.Scan(nil)
		assert.NoError(t, err)
		assert.Equal(t, ID(0), id)
	})

	// Test Scan from unsupported type
	t.Run("scan from unsupported type", func(t *testing.T) {
		var id ID
		err := id.Scan(float64(1.0))
		assert.Error(t, err)
	})

	// Test Scan from invalid byte string
	t.Run("scan from invalid bytes", func(t *testing.T) {
		var id ID
		err := id.Scan([]byte("abc"))
		assert.Error(t, err)
	})

	// Test Value (driver.Valuer interface)
	t.Run("value round trip", func(t *testing.T) {
		original := ID(54321)
		val, err := original.Value()
		assert.NoError(t, err)
		assert.Equal(t, int64(54321), val)

		// Scan the value back
		var scanned ID
		err = scanned.Scan(val)
		assert.NoError(t, err)
		assert.Equal(t, original, scanned)
	})
}

func TestIDStringConversions(t *testing.T) {
	id := ID(42)
	assert.Equal(t, "42", id.String())
	assert.Equal(t, int64(42), id.Int64())
}
