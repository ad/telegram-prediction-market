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
				GroupID:   1,
				Code:      AchievementEventOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid active organizer achievement",
			achievement: Achievement{
				UserID:    456,
				GroupID:   1,
				Code:      AchievementActiveOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid master organizer achievement",
			achievement: Achievement{
				UserID:    789,
				GroupID:   1,
				Code:      AchievementMasterOrganizer,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid existing achievement - sharpshooter",
			achievement: Achievement{
				UserID:    100,
				GroupID:   1,
				Code:      AchievementSharpshooter,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid existing achievement - veteran",
			achievement: Achievement{
				UserID:    200,
				GroupID:   1,
				Code:      AchievementVeteran,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "invalid achievement code",
			achievement: Achievement{
				UserID:    123,
				GroupID:   1,
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
				GroupID:   1,
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
				GroupID:   1,
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
				GroupID:   1,
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
				GroupID:   1,
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

func TestGroupValidation(t *testing.T) {
	tests := []struct {
		name        string
		group       Group
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid group",
			group: Group{
				ID:             1,
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now(),
				CreatedBy:      123,
			},
			wantErr: false,
		},
		{
			name: "valid group with long name",
			group: Group{
				ID:             2,
				TelegramChatID: -1009876543210,
				Name:           "A Very Long Group Name With Many Characters",
				CreatedAt:      time.Now(),
				CreatedBy:      456,
			},
			wantErr: false,
		},
		{
			name: "invalid telegram chat ID",
			group: Group{
				ID:             1,
				TelegramChatID: 0,
				Name:           "Test Group",
				CreatedAt:      time.Now(),
				CreatedBy:      123,
			},
			wantErr:     true,
			expectedErr: ErrInvalidTelegramChatID,
		},
		{
			name: "empty group name",
			group: Group{
				ID:             1,
				TelegramChatID: -1001234567890,
				Name:           "",
				CreatedAt:      time.Now(),
				CreatedBy:      123,
			},
			wantErr:     true,
			expectedErr: ErrEmptyGroupName,
		},
		{
			name: "invalid creator ID",
			group: Group{
				ID:             1,
				TelegramChatID: -1001234567890,
				Name:           "Test Group",
				CreatedAt:      time.Now(),
				CreatedBy:      0,
			},
			wantErr:     true,
			expectedErr: ErrInvalidCreator,
		},
		{
			name: "multiple invalid fields",
			group: Group{
				ID:             1,
				TelegramChatID: 0,
				Name:           "",
				CreatedAt:      time.Now(),
				CreatedBy:      0,
			},
			wantErr:     true,
			expectedErr: ErrInvalidTelegramChatID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("Group.Validate() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestGroupMembershipValidation(t *testing.T) {
	tests := []struct {
		name        string
		membership  GroupMembership
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid active membership",
			membership: GroupMembership{
				ID:       1,
				GroupID:  100,
				UserID:   200,
				JoinedAt: time.Now(),
				Status:   MembershipStatusActive,
			},
			wantErr: false,
		},
		{
			name: "valid removed membership",
			membership: GroupMembership{
				ID:       2,
				GroupID:  100,
				UserID:   300,
				JoinedAt: time.Now(),
				Status:   MembershipStatusRemoved,
			},
			wantErr: false,
		},
		{
			name: "invalid group ID",
			membership: GroupMembership{
				ID:       1,
				GroupID:  0,
				UserID:   200,
				JoinedAt: time.Now(),
				Status:   MembershipStatusActive,
			},
			wantErr:     true,
			expectedErr: ErrInvalidGroupID,
		},
		{
			name: "invalid user ID",
			membership: GroupMembership{
				ID:       1,
				GroupID:  100,
				UserID:   0,
				JoinedAt: time.Now(),
				Status:   MembershipStatusActive,
			},
			wantErr:     true,
			expectedErr: ErrInvalidUserID,
		},
		{
			name: "empty status",
			membership: GroupMembership{
				ID:       1,
				GroupID:  100,
				UserID:   200,
				JoinedAt: time.Now(),
				Status:   "",
			},
			wantErr:     true,
			expectedErr: ErrInvalidMembershipStatus,
		},
		{
			name: "invalid status",
			membership: GroupMembership{
				ID:       1,
				GroupID:  100,
				UserID:   200,
				JoinedAt: time.Now(),
				Status:   "invalid_status",
			},
			wantErr:     true,
			expectedErr: ErrInvalidMembershipStatus,
		},
		{
			name: "multiple invalid fields",
			membership: GroupMembership{
				ID:       1,
				GroupID:  0,
				UserID:   0,
				JoinedAt: time.Now(),
				Status:   "",
			},
			wantErr:     true,
			expectedErr: ErrInvalidGroupID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.membership.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupMembership.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.expectedErr != nil && err != tt.expectedErr {
				t.Errorf("GroupMembership.Validate() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestMembershipStatusConstants(t *testing.T) {
	// Test that membership status constants are correctly defined
	statuses := []MembershipStatus{
		MembershipStatusActive,
		MembershipStatusRemoved,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			membership := GroupMembership{
				ID:       1,
				GroupID:  100,
				UserID:   200,
				JoinedAt: time.Now(),
				Status:   status,
			}
			err := membership.Validate()
			if err != nil {
				t.Errorf("Expected membership status %s to be valid, got error: %v", status, err)
			}
		})
	}
}
