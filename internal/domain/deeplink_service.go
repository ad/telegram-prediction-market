package domain

import (
	"fmt"
	"strings"
)

// IDEncoder defines the interface for encoding and decoding IDs
type IDEncoder interface {
	Encode(num int64) (string, error)
	Decode(encoded string) (int64, error)
}

// DeepLinkService handles generation and parsing of Telegram deep-link URLs for group invitations
type DeepLinkService struct {
	botUsername string
	encoder     IDEncoder
}

// NewDeepLinkService creates a new DeepLinkService with the specified bot username and ID encoder
func NewDeepLinkService(botUsername string, encoder IDEncoder) *DeepLinkService {
	return &DeepLinkService{
		botUsername: botUsername,
		encoder:     encoder,
	}
}

// GenerateGroupInviteLink generates a Telegram deep-link URL for joining a specific group
// Format: https://t.me/{bot_username}?start=group_{encodedGroupID}
func (s *DeepLinkService) GenerateGroupInviteLink(groupID int64) (string, error) {
	encodedID, err := s.encoder.Encode(groupID)
	if err != nil {
		return "", fmt.Errorf("failed to encode group ID: %w", err)
	}
	return fmt.Sprintf("https://t.me/%s?start=group_%s", s.botUsername, encodedID), nil
}

// ParseGroupIDFromStart parses a group ID from a /start command parameter
// Expected format: "group_{encodedGroupID}"
// Returns the group ID and an error if the format is invalid
func (s *DeepLinkService) ParseGroupIDFromStart(startParam string) (int64, error) {
	// Check if the parameter starts with "group_"
	if !strings.HasPrefix(startParam, "group_") {
		return 0, fmt.Errorf("invalid start parameter format: expected 'group_<id>', got '%s'", startParam)
	}

	// Extract the encoded group ID part
	encodedID := strings.TrimPrefix(startParam, "group_")
	if encodedID == "" {
		return 0, fmt.Errorf("invalid start parameter: missing group ID")
	}

	// Decode the group ID
	groupID, err := s.encoder.Decode(encodedID)
	if err != nil {
		return 0, fmt.Errorf("invalid group ID in start parameter: %w", err)
	}

	return groupID, nil
}
