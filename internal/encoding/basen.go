package encoding

import (
	"errors"
	"math"
	"strings"
)

var (
	ErrInvalidAlphabet = errors.New("alphabet must contain at least 2 unique characters")
	ErrInvalidInput    = errors.New("input contains invalid characters")
	ErrNegativeNumber  = errors.New("cannot encode negative numbers")
)

// BaseNEncoder handles encoding and decoding of integers using a custom alphabet
type BaseNEncoder struct {
	alphabet  string
	base      int
	charMap   map[rune]int
	minLength int // Minimum length of encoded string (padding with first char)
}

// NewBaseNEncoder creates a new encoder with the specified alphabet
// Encoded strings will have a minimum length of 4 characters (padded with the first character)
func NewBaseNEncoder(alphabet string) (*BaseNEncoder, error) {
	if len(alphabet) < 2 {
		return nil, ErrInvalidAlphabet
	}

	// Check for duplicate characters
	seen := make(map[rune]bool)
	for _, char := range alphabet {
		if seen[char] {
			return nil, ErrInvalidAlphabet
		}
		seen[char] = true
	}

	// Build character to index map for decoding
	charMap := make(map[rune]int)
	for i, char := range alphabet {
		charMap[char] = i
	}

	return &BaseNEncoder{
		alphabet:  alphabet,
		base:      len(alphabet),
		charMap:   charMap,
		minLength: 4, // Minimum length for encoded strings
	}, nil
}

// Encode converts an integer to a base-N string using the alphabet
// The result is padded to minLength with the first character of the alphabet
func (e *BaseNEncoder) Encode(num int64) (string, error) {
	if num < 0 {
		return "", ErrNegativeNumber
	}

	if num == 0 {
		// Return minLength padding characters
		return strings.Repeat(string(e.alphabet[0]), e.minLength), nil
	}

	var result strings.Builder
	for num > 0 {
		remainder := num % int64(e.base)
		result.WriteByte(e.alphabet[remainder])
		num = num / int64(e.base)
	}

	// Reverse the string
	encoded := result.String()
	runes := []rune(encoded)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	// Pad to minimum length if needed
	encoded = string(runes)
	if len(encoded) < e.minLength {
		padding := strings.Repeat(string(e.alphabet[0]), e.minLength-len(encoded))
		encoded = padding + encoded
	}

	return encoded, nil
}

// Decode converts a base-N string back to an integer
// Leading padding characters (first char of alphabet) are automatically stripped
func (e *BaseNEncoder) Decode(encoded string) (int64, error) {
	if encoded == "" {
		return 0, ErrInvalidInput
	}

	// Strip leading padding characters (first character of alphabet)
	paddingChar := rune(e.alphabet[0])
	encoded = strings.TrimLeft(encoded, string(paddingChar))

	// If all characters were padding, the number is 0
	if encoded == "" {
		return 0, nil
	}

	var result int64
	for i, char := range encoded {
		value, ok := e.charMap[char]
		if !ok {
			return 0, ErrInvalidInput
		}

		power := len(encoded) - i - 1
		result += int64(value) * int64(math.Pow(float64(e.base), float64(power)))
	}

	return result, nil
}
