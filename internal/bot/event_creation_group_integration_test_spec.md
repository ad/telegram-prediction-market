# Event Creation with Group Selection - Integration Test Specification

This document specifies the integration tests that should be implemented for task 8.4 once the domain interfaces are updated to support group_id parameters.

## Test Cases

### Test 1: Event Creation with Single Group Membership

**Setup:**
- Create a test user with membership in exactly one group
- Initialize FSM for event creation

**Expected Behavior:**
- FSM should automatically select the single group (no prompt)
- Event creation should proceed directly to question input
- Created event should have the correct group_id

**Validation:**
- Verify no group selection prompt is shown
- Verify event.GroupID matches the user's single group
- Verify event is stored in database with correct group_id

### Test 2: Event Creation with Multiple Group Memberships

**Setup:**
- Create a test user with membership in 3 different groups
- Initialize FSM for event creation

**Expected Behavior:**
- FSM should prompt user to select a group
- User should see inline keyboard with all 3 groups
- After selection, event creation should proceed to question input
- Created event should have the selected group_id

**Validation:**
- Verify group selection prompt is shown
- Verify inline keyboard contains all user's groups
- Verify event.GroupID matches the selected group
- Verify event is stored in database with correct group_id

### Test 3: Event Association Verification

**Setup:**
- Create multiple groups and users
- Create events in different groups

**Expected Behavior:**
- Each event should be associated with exactly one group
- Events should be retrievable by group_id

**Validation:**
- Query events by group_id
- Verify only events from that group are returned
- Verify group_id is persisted correctly in database

## Implementation Notes

These tests should be implemented in `internal/bot/event_creation_integration_test.go` after:
1. Domain interfaces are updated to support group_id parameters (task 12)
2. EventManager is updated to handle group context (task 6)
3. GroupContextResolver is fully integrated (task 7)

## Requirements Validated

- Requirement 4.1: Group selection list for multi-group users
- Requirement 4.2: Automatic selection for single-group users
- Requirement 4.3: Event association with selected group
- Requirement 4.4: Group_id persistence in event data
