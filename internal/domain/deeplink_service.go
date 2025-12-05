package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// DeepLinkService handles generation and parsing of Telegram deep-link URLs for group invitations
type DeepLinkService struct {
	botUsername string
}

// NewDeepLinkService creates a new DeepLinkService with the specified bot username
func NewDeepLinkService(botUsername string) *DeepLinkService {
	return &DeepLinkService{
		botUsername: botUsername,
	}
}

// GenerateGroupInviteLink generates a Telegram deep-link URL for joining a specific group
// Format: https://t.me/{bot_username}?start=group_{groupID}
func (s *DeepLinkService) GenerateGroupInviteLink(groupID int64) string {
	return fmt.Sprintf("https://t.me/%s?start=group_%d", s.botUsername, groupID)
}

// ParseGroupIDFromStart parses a group ID from a /start command parameter
// Expected format: "group_{groupID}"
// Returns the group ID and an error if the format is invalid
func (s *DeepLinkService) ParseGroupIDFromStart(startParam string) (int64, error) {
	// Check if the parameter starts with "group_"
	if !strings.HasPrefix(startParam, "group_") {
		return 0, fmt.Errorf("invalid start parameter format: expected 'group_<id>', got '%s'", startParam)
	}

	// Extract the group ID part
	groupIDStr := strings.TrimPrefix(startParam, "group_")
	if groupIDStr == "" {
		return 0, fmt.Errorf("invalid start parameter: missing group ID")
	}

	// Parse the group ID
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid group ID in start parameter: %w", err)
	}

	return groupID, nil
}
