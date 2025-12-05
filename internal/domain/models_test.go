package domain

import (
	"testing"
	"time"
)

func TestAchievementValidation(t *testing.T) {
	tests := []struct {
		name        string
		achievement Achievement
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid event organizer achievement",
			achievement: Achievement{
				UserID:    123,
				Code:      AchievementEventOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid active organizer achievement",
			achievement: Achievement{
				UserID:    456,
				Code:      AchievementActiveOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid master organizer achievement",
			achievement: Achievement{
				UserID:    789,
				Code:      AchievementMasterOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid existing achievement - sharpshooter",
			achievement: Achievement{
				UserID:    100,
				Code:      AchievementSharpshooter,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid existing achievement - veteran",
			achievement: Achievement{
				UserID:    200,
				Code:      AchievementVeteran,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "invalid achievement code",
			achievement: Achievement{
				UserID:    123,
				Code:      "invalid_code",
				Timestamp: time.Now(),
			},
			wantErr:     true,
			expectedErr: ErrInvalidAchievementCode,
		},
		{
			name: "empty achievement code",
			achievement: Achievement{
				UserID:    123,
				Code:      "",
				Timestamp: time.Now(),
			},
			wantErr:     true,
			expectedErr: ErrInvalidAchievementCode,
		},
		{
			name: "invalid user ID",
			achievement: Achievement{
				UserID:    0,
				Code:      AchievementEventOrganizer,
				Timestamp: time.Now(),
			},
			wantErr:     true,
			expectedErr: ErrInvalidUserID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.achievement.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Achievement.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Achievement.Validate() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestNewAchievementCodes(t *testing.T) {
	// Test that all new achievement codes are accepted
	newCodes := []AchievementCode{
		AchievementEventOrganizer,
		AchievementActiveOrganizer,
		AchievementMasterOrganizer,
	}

	for _, code := range newCodes {
		t.Run(string(code), func(t *testing.T) {
			achievement := Achievement{
				UserID:    123,
				Code:      code,
				Timestamp: time.Now(),
			}
			err := achievement.Validate()
			if err != nil {
				t.Errorf("Expected new achievement code %s to be valid, got error: %v", code, err)
			}
		})
	}
}

func TestInvalidAchievementCodes(t *testing.T) {
	// Test that invalid codes are rejected
	invalidCodes := []AchievementCode{
		"unknown_achievement",
		"invalid",
		"event_creator",    // Similar but wrong
		"organizer",        // Partial match
		"MASTER_ORGANIZER", // Wrong case
	}

	for _, code := range invalidCodes {
		t.Run(string(code), func(t *testing.T) {
			achievement := Achievement{
				UserID:    123,
				Code:      code,
				Timestamp: time.Now(),
			}
			err := achievement.Validate()
			if err != ErrInvalidAchievementCode {
				t.Errorf("Expected invalid achievement code %s to return ErrInvalidAchievementCode, got: %v", code, err)
			}
		})
	}
}
