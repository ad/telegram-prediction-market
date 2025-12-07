package domain

import (
	"fmt"
)

// EventResolutionContext holds data during event resolution flow
type EventResolutionContext struct {
	EventID    int64 `json:"event_id"`
	MessageIDs []int `json:"message_ids"` // All message IDs to delete at the end
	ChatID     int64 `json:"chat_id"`
}

// ToMap converts EventResolutionContext to a map for JSON serialization
func (c *EventResolutionContext) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"event_id":    c.EventID,
		"message_ids": c.MessageIDs,
		"chat_id":     c.ChatID,
	}
}

// FromMap populates EventResolutionContext from a map after JSON deserialization
func (c *EventResolutionContext) FromMap(data map[string]interface{}) error {
	if data == nil {
		return ErrInvalidContextData
	}

	// Parse event_id (handle both int64 and float64 from JSON)
	if eventID, ok := data["event_id"].(float64); ok {
		c.EventID = int64(eventID)
	} else if eventID, ok := data["event_id"].(int64); ok {
		c.EventID = eventID
	} else if eventID, ok := data["event_id"].(int); ok {
		c.EventID = int64(eventID)
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

// Validate validates the EventResolutionContext for required fields
func (c *EventResolutionContext) Validate() error {
	if c.ChatID == 0 {
		return fmt.Errorf("%w: chat_id", ErrMissingRequiredField)
	}

	return nil
}
