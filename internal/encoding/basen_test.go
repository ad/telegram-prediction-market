package encoding

import (
	"testing"
)

func TestNewBaseNEncoder(t *testing.T) {
	tests := []struct {
		name      string
		alphabet  string
		wantError bool
	}{
		{
			name:      "valid alphabet",
			alphabet:  "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			wantError: false,
		},
		{
			name:      "short alphabet",
			alphabet:  "01",
			wantError: false,
		},
		{
			name:      "too short alphabet",
			alphabet:  "0",
			wantError: true,
		},
		{
			name:      "duplicate characters",
			alphabet:  "0123456789012",
			wantError: true,
		},
		{
			name:      "empty alphabet",
			alphabet:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder, err := NewBaseNEncoder(tt.alphabet)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if encoder == nil {
					t.Errorf("expected encoder, got nil")
				}
			}
		})
	}
}

func TestBaseNEncoder_Encode(t *testing.T) {
	// Base62 alphabet (like URL shorteners)
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoder, err := NewBaseNEncoder(alphabet)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	tests := []struct {
		name    string
		input   int64
		want    string
		wantErr bool
	}{
		{
			name:    "zero",
			input:   0,
			want:    "0000",
			wantErr: false,
		},
		{
			name:    "one",
			input:   1,
			want:    "0001",
			wantErr: false,
		},
		{
			name:    "base value",
			input:   62,
			want:    "0010",
			wantErr: false,
		},
		{
			name:    "large number",
			input:   123456789,
			want:    "8m0Kx",
			wantErr: false,
		},
		{
			name:    "negative number",
			input:   -1,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encoder.Encode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("Encode(%d) = %s, want %s", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestBaseNEncoder_Decode(t *testing.T) {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoder, err := NewBaseNEncoder(alphabet)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:    "zero with padding",
			input:   "0000",
			want:    0,
			wantErr: false,
		},
		{
			name:    "zero without padding",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "one with padding",
			input:   "0001",
			want:    1,
			wantErr: false,
		},
		{
			name:    "one without padding",
			input:   "1",
			want:    1,
			wantErr: false,
		},
		{
			name:    "base value with padding",
			input:   "0010",
			want:    62,
			wantErr: false,
		},
		{
			name:    "base value without padding",
			input:   "10",
			want:    62,
			wantErr: false,
		},
		{
			name:    "large number",
			input:   "8m0Kx",
			want:    123456789,
			wantErr: false,
		},
		{
			name:    "invalid character",
			input:   "xyz!",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encoder.Decode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("Decode(%s) = %d, want %d", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestBaseNEncoder_RoundTrip(t *testing.T) {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoder, err := NewBaseNEncoder(alphabet)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	testNumbers := []int64{0, 1, 10, 100, 1000, 10000, 100000, 1000000, 123456789}

	for _, num := range testNumbers {
		t.Run("", func(t *testing.T) {
			encoded, err := encoder.Encode(num)
			if err != nil {
				t.Fatalf("Encode(%d) failed: %v", num, err)
			}

			decoded, err := encoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(%s) failed: %v", encoded, err)
			}

			if decoded != num {
				t.Errorf("Round trip failed: %d -> %s -> %d", num, encoded, decoded)
			}
		})
	}
}

func TestBaseNEncoder_MinimumLength(t *testing.T) {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoder, err := NewBaseNEncoder(alphabet)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	tests := []struct {
		name      string
		input     int64
		minLength int
	}{
		{
			name:      "zero",
			input:     0,
			minLength: 4,
		},
		{
			name:      "one",
			input:     1,
			minLength: 4,
		},
		{
			name:      "ten",
			input:     10,
			minLength: 4,
		},
		{
			name:      "hundred",
			input:     100,
			minLength: 4,
		},
		{
			name:      "thousand",
			input:     1000,
			minLength: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encoder.Encode(tt.input)
			if err != nil {
				t.Fatalf("Encode(%d) failed: %v", tt.input, err)
			}

			if len(encoded) < tt.minLength {
				t.Errorf("Encoded length %d is less than minimum %d for input %d (encoded: %s)",
					len(encoded), tt.minLength, tt.input, encoded)
			}

			// Verify round trip still works
			decoded, err := encoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(%s) failed: %v", encoded, err)
			}

			if decoded != tt.input {
				t.Errorf("Round trip failed: %d -> %s -> %d", tt.input, encoded, decoded)
			}
		})
	}
}

func TestBaseNEncoder_PaddingCompatibility(t *testing.T) {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoder, err := NewBaseNEncoder(alphabet)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}

	// Test that both padded and unpadded versions decode to the same value
	tests := []struct {
		padded   string
		unpadded string
		expected int64
	}{
		{
			padded:   "0001",
			unpadded: "1",
			expected: 1,
		},
		{
			padded:   "000a",
			unpadded: "a",
			expected: 10,
		},
		{
			padded:   "0010",
			unpadded: "10",
			expected: 62,
		},
	}

	for _, tt := range tests {
		t.Run(tt.unpadded, func(t *testing.T) {
			decodedPadded, err := encoder.Decode(tt.padded)
			if err != nil {
				t.Fatalf("Decode(%s) failed: %v", tt.padded, err)
			}

			decodedUnpadded, err := encoder.Decode(tt.unpadded)
			if err != nil {
				t.Fatalf("Decode(%s) failed: %v", tt.unpadded, err)
			}

			if decodedPadded != tt.expected {
				t.Errorf("Padded decode failed: expected %d, got %d", tt.expected, decodedPadded)
			}

			if decodedUnpadded != tt.expected {
				t.Errorf("Unpadded decode failed: expected %d, got %d", tt.expected, decodedUnpadded)
			}

			if decodedPadded != decodedUnpadded {
				t.Errorf("Padded and unpadded decode mismatch: %d != %d", decodedPadded, decodedUnpadded)
			}
		})
	}
}
