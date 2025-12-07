package domain

import (
	"fmt"
)

// GroupCreationContext holds data during group creation flow
type GroupCreationContext struct {
	GroupName      string `json:"group_name"`
	TelegramChatID int64  `json:"telegram_chat_id"`
	MessageIDs     []int  `json:"message_ids"` // All message IDs to delete on error/cancel
	ChatID         int64  `json:"chat_id"`
}

// ToMap converts GroupCreationContext to a map for JSON serialization
func (c *GroupCreationContext) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"group_name":       c.GroupName,
		"telegram_chat_id": c.TelegramChatID,
		"message_ids":      c.MessageIDs,
		"chat_id":          c.ChatID,
	}
}

// FromMap populates GroupCreationContext from a map after JSON deserialization
func (c *GroupCreationContext) FromMap(data map[string]interface{}) error {
	if data == nil {
		return ErrInvalidContextData
	}

	// Parse group_name
	if groupName, ok := data["group_name"].(string); ok {
		c.GroupName = groupName
	}

	// Parse telegram_chat_id (handle both int64 and float64 from JSON)
	if telegramChatID, ok := data["telegram_chat_id"].(float64); ok {
		c.TelegramChatID = int64(telegramChatID)
	} else if telegramChatID, ok := data["telegram_chat_id"].(int64); ok {
		c.TelegramChatID = telegramChatID
	} else if telegramChatID, ok := data["telegram_chat_id"].(int); ok {
		c.TelegramChatID = int64(telegramChatID)
	}

	// Parse message_ids
	if messageIDs, ok := data["message_ids"].([]interface{}); ok {
		c.MessageIDs = make([]int, len(messageIDs))
		for i, msgID := range messageIDs {
			if msgIDFloat, ok := msgID.(float64); ok {
				c.MessageIDs[i] = int(msgIDFloat)
			} else if msgIDInt, ok := msgID.(int); ok {
				c.MessageIDs[i] = msgIDInt
			}
		}
	}

	// Parse chat_id (handle both int64 and float64 from JSON)
	if chatID, ok := data["chat_id"].(float64); ok {
		c.ChatID = int64(chatID)
	} else if chatID, ok := data["chat_id"].(int64); ok {
		c.ChatID = chatID
	} else if chatID, ok := data["chat_id"].(int); ok {
		c.ChatID = int64(chatID)
	}

	return nil
}

// Validate validates the GroupCreationContext for required fields
func (c *GroupCreationContext) Validate() error {
	if c.ChatID == 0 {
		return fmt.Errorf("%w: chat_id", ErrMissingRequiredField)
	}

	return nil
}
