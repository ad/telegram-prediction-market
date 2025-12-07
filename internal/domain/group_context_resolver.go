package domain

import (
	"context"
	"errors"
)

var (
	ErrNoGroupMembership        = errors.New("user has no group memberships")
	ErrMultipleGroupsNeedChoice = errors.New("user has multiple groups, selection required")
)

// GroupRepository interface for group operations
type GroupRepository interface {
	CreateGroup(ctx context.Context, group *Group) error
	GetGroup(ctx context.Context, groupID int64) (*Group, error)
	GetGroupByTelegramChatID(ctx context.Context, telegramChatID int64) (*Group, error)
	GetAllGroups(ctx context.Context) ([]*Group, error)
	GetUserGroups(ctx context.Context, userID int64) ([]*Group, error)
	DeleteGroup(ctx context.Context, groupID int64) error
	UpdateGroupStatus(ctx context.Context, groupID int64, status GroupStatus) error
	UpdateGroupName(ctx context.Context, groupID int64, name string) error
}

// GroupMembershipRepository interface for group membership operations
type GroupMembershipRepository interface {
	CreateMembership(ctx context.Context, membership *GroupMembership) error
	GetMembership(ctx context.Context, groupID int64, userID int64) (*GroupMembership, error)
	GetGroupMembers(ctx context.Context, groupID int64) ([]*GroupMembership, error)
	UpdateMembershipStatus(ctx context.Context, groupID int64, userID int64, status MembershipStatus) error
	HasActiveMembership(ctx context.Context, groupID int64, userID int64) (bool, error)
}

// ForumTopicRepository interface for forum topic operations
type ForumTopicRepository interface {
	CreateForumTopic(ctx context.Context, topic *ForumTopic) error
	GetForumTopic(ctx context.Context, topicID int64) (*ForumTopic, error)
	GetForumTopicByGroupAndThread(ctx context.Context, groupID int64, messageThreadID int) (*ForumTopic, error)
	GetForumTopicsByGroup(ctx context.Context, groupID int64) ([]*ForumTopic, error)
	DeleteForumTopic(ctx context.Context, topicID int64) error
	UpdateForumTopicName(ctx context.Context, topicID int64, name string) error
}

// GroupContextResolver determines the active group context for a user
type GroupContextResolver struct {
	groupRepo GroupRepository
}

// NewGroupContextResolver creates a new GroupContextResolver
func NewGroupContextResolver(groupRepo GroupRepository) *GroupContextResolver {
	return &GroupContextResolver{
		groupRepo: groupRepo,
	}
}

// ResolveGroupForUser determines the active group for a user
// Returns the group ID if the user has exactly one group membership
// Returns ErrNoGroupMembership if the user has no groups
// Returns ErrMultipleGroupsNeedChoice if the user has multiple groups
func (r *GroupContextResolver) ResolveGroupForUser(ctx context.Context, userID int64) (int64, error) {
	groups, err := r.groupRepo.GetUserGroups(ctx, userID)
	if err != nil {
		return 0, err
	}

	if len(groups) == 0 {
		return 0, ErrNoGroupMembership
	}

	if len(groups) == 1 {
		return groups[0].ID, nil
	}

	return 0, ErrMultipleGroupsNeedChoice
}

// GetUserGroupChoices returns all groups where the user has active membership
func (r *GroupContextResolver) GetUserGroupChoices(ctx context.Context, userID int64) ([]*Group, error) {
	return r.groupRepo.GetUserGroups(ctx, userID)
}
