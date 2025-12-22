package locale

// Message key constants for localization
// All user-facing messages should use these constants to ensure consistency

const (
	BotStarted                    = "BotStarted"
	StartingTelegramPredictionBot = "StartingBot"

	// ============================================================================
	// COMMANDS AND HELP
	// ============================================================================

	// Help command messages
	HelpTitle                = "HelpTitle"
	HelpUserCommandsSection  = "HelpUserCommandsSection"
	HelpAdminCommandsSection = "HelpAdminCommandsSection"

	// User commands
	HelpCommandHelp   = "HelpCommandHelp"
	HelpCommandRating = "HelpCommandRating"
	HelpCommandMy     = "HelpCommandMy"
	HelpCommandEvents = "HelpCommandEvents"
	HelpCommandGroups = "HelpCommandGroups"

	// Admin commands
	HelpCommandCreateGroup  = "HelpCommandCreateGroup"
	HelpCommandListGroups   = "HelpCommandListGroups"
	HelpCommandGroupMembers = "HelpCommandGroupMembers"
	HelpCommandRemoveMember = "HelpCommandRemoveMember"
	HelpCommandCreateEvent  = "HelpCommandCreateEvent"
	HelpCommandResolveEvent = "HelpCommandResolveEvent"
	HelpCommandEditEvent    = "HelpCommandEditEvent"
	HelpListGroupsHint      = "HelpListGroupsHint"

	// Rules and scoring
	HelpScoringRulesTitle      = "HelpScoringRulesTitle"
	HelpScoringCorrectTitle    = "HelpScoringCorrectTitle"
	HelpScoringBinary          = "HelpScoringBinary"
	HelpScoringMultiOption     = "HelpScoringMultiOption"
	HelpScoringProbability     = "HelpScoringProbability"
	HelpScoringBonusesTitle    = "HelpScoringBonusesTitle"
	HelpScoringMinority        = "HelpScoringMinority"
	HelpScoringEarlyVote       = "HelpScoringEarlyVote"
	HelpScoringParticipation   = "HelpScoringParticipation"
	HelpScoringPenaltiesTitle  = "HelpScoringPenaltiesTitle"
	HelpScoringWrongPrediction = "HelpScoringWrongPrediction"

	// Achievements
	HelpAchievementsTitle        = "HelpAchievementsTitle"
	HelpAchievementSharpshooter  = "HelpAchievementSharpshooter"
	HelpAchievementProphet       = "HelpAchievementProphet"
	HelpAchievementRiskTaker     = "HelpAchievementRiskTaker"
	HelpAchievementWeeklyAnalyst = "HelpAchievementWeeklyAnalyst"
	HelpAchievementVeteran       = "HelpAchievementVeteran"

	// Event types
	HelpEventTypesTitle       = "HelpEventTypesTitle"
	HelpEventTypeBinary       = "HelpEventTypeBinary"
	HelpEventTypeMultiOption  = "HelpEventTypeMultiOption"
	HelpEventTypeProbability  = "HelpEventTypeProbability"
	HelpEventVoteReminder     = "HelpEventVoteReminder"
	HelpEventDeadlineReminder = "HelpEventDeadlineReminder"

	// ============================================================================
	// EVENT CREATION FSM
	// ============================================================================

	// Event creation prompts
	EventCreationTitle       = "EventCreationTitle"
	EventCreationAskQuestion = "EventCreationAskQuestion"
	EventCreationSelectType  = "EventCreationSelectType"
	EventCreationAskOptions  = "EventCreationAskOptions"
	EventCreationAskDeadline = "EventCreationAskDeadline"
	EventCreationConfirm     = "EventCreationConfirm"
	EventCreationSelectGroup = "EventCreationSelectGroup"

	// Event type buttons
	EventTypeBinaryButton      = "EventTypeBinaryButton"
	EventTypeMultiOptionButton = "EventTypeMultiOptionButton"
	EventTypeProbabilityButton = "EventTypeProbabilityButton"

	// Event type labels
	EventTypeBinaryLabel      = "EventTypeBinaryLabel"
	EventTypeMultiOptionLabel = "EventTypeMultiOptionLabel"
	EventTypeProbabilityLabel = "EventTypeProbabilityLabel"

	// Event type icons
	EventTypeBinaryIcon      = "EventTypeBinaryIcon"
	EventTypeMultiOptionIcon = "EventTypeMultiOptionIcon"
	EventTypeProbabilityIcon = "EventTypeProbabilityIcon"

	// Event creation confirmations
	EventCreationTypeBinarySelected      = "EventCreationTypeBinarySelected"
	EventCreationTypeMultiOptionSelected = "EventCreationTypeMultiOptionSelected"
	EventCreationTypeProbabilitySelected = "EventCreationTypeProbabilitySelected"
	EventCreationQuestionSaved           = "EventCreationQuestionSaved"
	EventCreationOptionsSaved            = "EventCreationOptionsSaved"
	EventCreationDeadlineSaved           = "EventCreationDeadlineSaved"

	// Event creation success
	EventCreationSuccess       = "EventCreationSuccess"
	EventCreationPollPublished = "EventCreationPollPublished"
	EventCreationPollReference = "EventCreationPollReference"

	// Event creation errors
	EventCreationErrorInvalidQuestion       = "EventCreationErrorInvalidQuestion"
	EventCreationErrorInvalidOptions        = "EventCreationErrorInvalidOptions"
	EventCreationErrorInvalidDeadline       = "EventCreationErrorInvalidDeadline"
	EventCreationErrorTooFewOptions         = "EventCreationErrorTooFewOptions"
	EventCreationErrorTooManyOptions        = "EventCreationErrorTooManyOptions"
	EventCreationErrorNoGroupMembership     = "EventCreationErrorNoGroupMembership"
	EventCreationErrorNoGroupMembershipHelp = "EventCreationErrorNoGroupMembershipHelp"

	// Event creation permission notification
	EventCreationPermissionGranted      = "EventCreationPermissionGranted"
	EventCreationPermissionInstructions = "EventCreationPermissionInstructions"

	// Deadline prompt messages
	DeadlinePromptMessage          = "DeadlinePromptMessage"
	DeadlineFormatExamples         = "DeadlineFormatExamples"
	DeadlineFormatRelative         = "DeadlineFormatRelative"
	DeadlineFormatAbsolute         = "DeadlineFormatAbsolute"
	DeadlineFormatAbsoluteWithTime = "DeadlineFormatAbsoluteWithTime"

	// Session expiration messages
	SessionExpiredShort = "SessionExpiredShort"
	SessionExpiredLong  = "SessionExpiredLong"

	// Group selection messages
	EventCreationNoGroupsAvailable = "EventCreationNoGroupsAvailable"

	// Deadline preset buttons
	DeadlinePreset1Day    = "DeadlinePreset1Day"
	DeadlinePreset3Days   = "DeadlinePreset3Days"
	DeadlinePreset1Week   = "DeadlinePreset1Week"
	DeadlinePreset2Weeks  = "DeadlinePreset2Weeks"
	DeadlinePreset1Month  = "DeadlinePreset1Month"
	DeadlinePreset3Months = "DeadlinePreset3Months"
	DeadlinePreset6Months = "DeadlinePreset6Months"
	DeadlinePreset1Year   = "DeadlinePreset1Year"

	// Event summary labels
	EventSummaryTitle    = "EventSummaryTitle"
	EventSummaryQuestion = "EventSummaryQuestion"
	EventSummaryType     = "EventSummaryType"
	EventSummaryOptions  = "EventSummaryOptions"
	EventSummaryDeadline = "EventSummaryDeadline"

	// Final event summary
	EventFinalSummaryTitle = "EventFinalSummaryTitle"
	EventFinalSummaryID    = "EventFinalSummaryID"

	// Confirmation buttons
	ConfirmButtonYes = "ConfirmButtonYes"
	ConfirmButtonNo  = "ConfirmButtonNo"

	// Event creation cancellation
	EventCreationCancelled = "EventCreationCancelled"

	// Event creation system errors
	EventCreationErrorGeneric     = "EventCreationErrorGeneric"
	EventCreationErrorGroupInfo   = "EventCreationErrorGroupInfo"
	EventCreationErrorPollPublish = "EventCreationErrorPollPublish"

	// Action buttons
	ActionButtonEdit    = "ActionButtonEdit"
	ActionButtonResolve = "ActionButtonResolve"

	// Achievement notification (in group context)
	AchievementNotificationUser  = "AchievementNotificationUser"
	AchievementNotificationGroup = "AchievementNotificationGroup"

	// Achievement names for event organizers
	AchievementEventOrganizerName  = "AchievementEventOrganizerName"
	AchievementActiveOrganizerName = "AchievementActiveOrganizerName"
	AchievementMasterOrganizerName = "AchievementMasterOrganizerName"

	// Deadline error messages
	EventCreationErrorDeadlineFormat = "EventCreationErrorDeadlineFormat"
	EventCreationErrorDeadlinePast   = "EventCreationErrorDeadlinePast"

	// Options count validation
	EventCreationErrorOptionsCount = "EventCreationErrorOptionsCount"

	// Default options for event types
	EventOptionYes = "EventOptionYes"
	EventOptionNo  = "EventOptionNo"

	// Probability range options
	EventOptionProbability0to25   = "EventOptionProbability0to25"
	EventOptionProbability25to50  = "EventOptionProbability25to50"
	EventOptionProbability50to75  = "EventOptionProbability50to75"
	EventOptionProbability75to100 = "EventOptionProbability75to100"

	// ============================================================================
	// EVENT RESOLUTION FSM
	// ============================================================================

	// Event resolution prompts
	EventResolutionTitle                  = "EventResolutionTitle"
	EventResolutionSelectEvent            = "EventResolutionSelectEvent"
	EventResolutionSelectOption           = "EventResolutionSelectOption"
	EventResolutionSelectCorrectAnswer    = "EventResolutionSelectCorrectAnswer"
	EventResolutionConfirm                = "EventResolutionConfirm"
	EventResolutionPermissionGranted      = "EventResolutionPermissionGranted"
	EventResolutionPermissionInstructions = "EventResolutionPermissionInstructions"

	// Event resolution success
	EventResolutionSuccess        = "EventResolutionSuccess"
	EventResolutionEventCompleted = "EventResolutionEventCompleted"

	// Event resolution errors
	EventResolutionErrorNoEvents             = "EventResolutionErrorNoEvents"
	EventResolutionErrorInvalidEvent         = "EventResolutionErrorInvalidEvent"
	EventResolutionErrorInvalidOption        = "EventResolutionErrorInvalidOption"
	EventResolutionErrorPermissionCheck      = "EventResolutionErrorPermissionCheck"
	EventResolutionErrorPermissionCheckRetry = "EventResolutionErrorPermissionCheckRetry"
	EventResolutionErrorUnauthorized         = "EventResolutionErrorUnauthorized"
	EventResolutionErrorGetEvent             = "EventResolutionErrorGetEvent"
	EventResolutionErrorResolve              = "EventResolutionErrorResolve"
	EventResolutionAchievementNotification   = "EventResolutionAchievementNotification"

	// ============================================================================
	// GROUP CREATION FSM
	// ============================================================================

	// Group creation prompts
	GroupCreationTitle                   = "GroupCreationTitle"
	GroupCreationAskName                 = "GroupCreationAskName"
	GroupCreationAskChatID               = "GroupCreationAskChatID"
	GroupCreationAskChatIDHelp           = "GroupCreationAskChatIDHelp"
	GroupCreationAskChatIDInstructions   = "GroupCreationAskChatIDInstructions"
	GroupCreationAskIsForum              = "GroupCreationAskIsForum"
	GroupCreationAskForumTopicID         = "GroupCreationAskForumTopicID"
	GroupCreationAskForumTopicIDHelp     = "GroupCreationAskForumTopicIDHelp"
	GroupCreationAskThreadIDInstructions = "GroupCreationAskThreadIDInstructions"

	// Group creation confirmations
	GroupCreationNameSaved   = "GroupCreationNameSaved"
	GroupCreationChatIDSaved = "GroupCreationChatIDSaved"

	// Group creation success
	GroupCreationSuccess                = "GroupCreationSuccess"
	GroupCreationSuccessExisting        = "GroupCreationSuccessExisting"
	GroupCreationSuccessNew             = "GroupCreationSuccessNew"
	GroupCreationSuccessDetails         = "GroupCreationSuccessDetails"
	GroupCreationSuccessForumType       = "GroupCreationSuccessForumType"
	GroupCreationSuccessRegularType     = "GroupCreationSuccessRegularType"
	GroupCreationSuccessThreadID        = "GroupCreationSuccessThreadID"
	GroupCreationSuccessTopicRegistered = "GroupCreationSuccessTopicRegistered"
	GroupCreationInviteLink             = "GroupCreationInviteLink"
	GroupCreationInviteLinkInstructions = "GroupCreationInviteLinkInstructions"
	GroupCreationAdminNotification      = "GroupCreationAdminNotification"

	// Group creation errors
	GroupCreationErrorInvalidName       = "GroupCreationErrorInvalidName"
	GroupCreationErrorInvalidChatID     = "GroupCreationErrorInvalidChatID"
	GroupCreationErrorInvalidChatIDHelp = "GroupCreationErrorInvalidChatIDHelp"
	GroupCreationErrorInvalidTopicID    = "GroupCreationErrorInvalidTopicID"
	GroupCreationErrorCheckExisting     = "GroupCreationErrorCheckExisting"
	GroupCreationErrorValidation        = "GroupCreationErrorValidation"
	GroupCreationErrorCreate            = "GroupCreationErrorCreate"
	GroupCreationErrorInviteLink        = "GroupCreationErrorInviteLink"

	// Group creation buttons
	GroupCreationButtonForum   = "GroupCreationButtonForum"
	GroupCreationButtonRegular = "GroupCreationButtonRegular"

	// ============================================================================
	// EVENT EDIT FSM
	// ============================================================================

	// Event edit prompts
	EventEditTitle              = "EventEditTitle"
	EventEditSelectEvent        = "EventEditSelectEvent"
	EventEditSelectField        = "EventEditSelectField"
	EventEditAskNewValue        = "EventEditAskNewValue"
	EventEditCurrentQuestion    = "EventEditCurrentQuestion"
	EventEditCurrentOptions     = "EventEditCurrentOptions"
	EventEditCurrentDeadline    = "EventEditCurrentDeadline"
	EventEditPromptQuestion     = "EventEditPromptQuestion"
	EventEditPromptOptions      = "EventEditPromptOptions"
	EventEditPromptOptionsHelp  = "EventEditPromptOptionsHelp"
	EventEditPromptDeadline     = "EventEditPromptDeadline"
	EventEditPromptDeadlineHelp = "EventEditPromptDeadlineHelp"
	EventEditSelectFieldPrompt  = "EventEditSelectFieldPrompt"

	// Event edit buttons
	EventEditButtonQuestion = "EventEditButtonQuestion"
	EventEditButtonOptions  = "EventEditButtonOptions"
	EventEditButtonDeadline = "EventEditButtonDeadline"
	EventEditButtonSave     = "EventEditButtonSave"
	EventEditButtonCancel   = "EventEditButtonCancel"

	// Event edit success
	EventEditSuccess        = "EventEditSuccess"
	EventEditSuccessUpdated = "EventEditSuccessUpdated"
	EventEditCancelled      = "EventEditCancelled"

	// Event edit errors
	EventEditErrorNoEvents        = "EventEditErrorNoEvents"
	EventEditErrorInvalidEvent    = "EventEditErrorInvalidEvent"
	EventEditErrorInvalidField    = "EventEditErrorInvalidField"
	EventEditErrorEmptyQuestion   = "EventEditErrorEmptyQuestion"
	EventEditErrorEmptyOptions    = "EventEditErrorEmptyOptions"
	EventEditErrorOptionsCount    = "EventEditErrorOptionsCount"
	EventEditErrorInvalidDeadline = "EventEditErrorInvalidDeadline"
	EventEditErrorDeadlinePast    = "EventEditErrorDeadlinePast"
	EventEditErrorGetEvent        = "EventEditErrorGetEvent"
	EventEditErrorHasVotes        = "EventEditErrorHasVotes"
	EventEditErrorSave            = "EventEditErrorSave"

	// ============================================================================
	// RENAME FSM
	// ============================================================================

	// Rename prompts
	RenameTitle   = "RenameTitle"
	RenameAskName = "RenameAskName"

	// Rename success
	RenameGroupSuccess = "RenameGroupSuccess"
	RenameTopicSuccess = "RenameTopicSuccess"

	// Rename errors
	RenameErrorInvalidName = "RenameErrorInvalidName"
	RenameErrorNameTooLong = "RenameErrorNameTooLong"
	RenameErrorEmptyName   = "RenameErrorEmptyName"
	RenameErrorGetContext  = "RenameErrorGetContext"
	RenameErrorUpdateGroup = "RenameErrorUpdateGroup"
	RenameErrorUpdateTopic = "RenameErrorUpdateTopic"

	// ============================================================================
	// NOTIFICATIONS
	// ============================================================================

	// New event notification
	NotificationNewEventTitle    = "NotificationNewEventTitle"
	NotificationNewEventQuestion = "NotificationNewEventQuestion"
	NotificationNewEventType     = "NotificationNewEventType"
	NotificationNewEventOptions  = "NotificationNewEventOptions"
	NotificationNewEventDeadline = "NotificationNewEventDeadline"
	NotificationNewEventCTA      = "NotificationNewEventCTA"

	// Achievement notification
	NotificationAchievementCongrats     = "NotificationAchievementCongrats"
	NotificationAchievementAnnouncement = "NotificationAchievementAnnouncement"

	// Achievement names
	AchievementSharpshooterName  = "AchievementSharpshooterName"
	AchievementProphetName       = "AchievementProphetName"
	AchievementRiskTakerName     = "AchievementRiskTakerName"
	AchievementWeeklyAnalystName = "AchievementWeeklyAnalystName"
	AchievementVeteranName       = "AchievementVeteranName"

	// Event results notification
	NotificationResultsTitle         = "NotificationResultsTitle"
	NotificationResultsQuestion      = "NotificationResultsQuestion"
	NotificationResultsCorrectAnswer = "NotificationResultsCorrectAnswer"
	NotificationResultsStats         = "NotificationResultsStats"
	NotificationResultsTopTitle      = "NotificationResultsTopTitle"

	// Deadline reminder
	NotificationReminderTitle    = "NotificationReminderTitle"
	NotificationReminderTime     = "NotificationReminderTime"
	NotificationReminderQuestion = "NotificationReminderQuestion"
	NotificationReminderCTA      = "NotificationReminderCTA"

	// Event expired notification to organizer
	NotificationEventExpiredTitle      = "NotificationEventExpiredTitle"
	NotificationEventExpiredQuestion   = "NotificationEventExpiredQuestion"
	NotificationEventExpiredStats      = "NotificationEventExpiredStats"
	NotificationEventExpiredCTA        = "NotificationEventExpiredCTA"
	NotificationEventExpiredButtonText = "NotificationEventExpiredButtonText"

	// Deadline formatting
	DeadlineExpired     = "DeadlineExpired"
	DeadlineDaysHours   = "DeadlineDaysHours"
	DeadlineHoursOnly   = "DeadlineHoursOnly"
	DeadlineLabel       = "DeadlineLabel"
	DeadlineIconAndText = "DeadlineIconAndText"

	// Option formatting
	OptionListItem = "OptionListItem"

	// Top ratings formatting
	RatingTopEntry = "RatingTopEntry"
	RatingMedals   = "RatingMedals"

	// ============================================================================
	// RATING AND STATISTICS
	// ============================================================================

	// Rating command
	RatingTitle      = "RatingTitle"
	RatingGroupLabel = "RatingGroupLabel"
	RatingTopTitle   = "RatingTopTitle"
	RatingEmpty      = "RatingEmpty"
	RatingAccuracy   = "RatingAccuracy"
	RatingStreak     = "RatingStreak"
	RatingCorrect    = "RatingCorrect"
	RatingWrong      = "RatingWrong"
	RatingPoints     = "RatingPoints"

	// My stats command
	MyStatsTitle             = "MyStatsTitle"
	MyStatsGroupLabel        = "MyStatsGroupLabel"
	MyStatsPoints            = "MyStatsPoints"
	MyStatsCorrect           = "MyStatsCorrect"
	MyStatsWrong             = "MyStatsWrong"
	MyStatsAccuracy          = "MyStatsAccuracy"
	MyStatsStreak            = "MyStatsStreak"
	MyStatsTotalPredictions  = "MyStatsTotalPredictions"
	MyStatsAchievementsTitle = "MyStatsAchievementsTitle"
	MyStatsNoAchievements    = "MyStatsNoAchievements"

	// ============================================================================
	// EVENTS LIST
	// ============================================================================

	// Events command
	EventsTitle           = "EventsTitle"
	EventsEmpty           = "EventsEmpty"
	EventsGroupLabel      = "EventsGroupLabel"
	EventsTypeLabel       = "EventsTypeLabel"
	EventsOptionsTitle    = "EventsOptionsTitle"
	EventsTotalVotes      = "EventsTotalVotes"
	EventsDeadlineLabel   = "EventsDeadlineLabel"
	EventsDeadlineExpired = "EventsDeadlineExpired"
	EventsDeadlineDays    = "EventsDeadlineDays"
	EventsDeadlineHours   = "EventsDeadlineHours"
	EventsDeadlineMinutes = "EventsDeadlineMinutes"

	// ============================================================================
	// GROUPS
	// ============================================================================

	// Group join
	GroupJoinWelcome       = "GroupJoinWelcome"
	GroupJoinWelcomeBack   = "GroupJoinWelcomeBack"
	GroupJoinAlreadyMember = "GroupJoinAlreadyMember"
	GroupJoinInstructions  = "GroupJoinInstructions"

	// Group errors
	GroupErrorInvalidLink = "GroupErrorInvalidLink"
	GroupErrorNotFound    = "GroupErrorNotFound"
	GroupErrorCheckFailed = "GroupErrorCheckFailed"
	GroupErrorJoinFailed  = "GroupErrorJoinFailed"

	// Group context
	GroupContextNoMembership     = "GroupContextNoMembership"
	GroupContextMultipleGroups   = "GroupContextMultipleGroups"
	GroupContextJoinInstructions = "GroupContextJoinInstructions"

	// ============================================================================
	// ERRORS
	// ============================================================================

	// Authorization errors
	ErrorUnauthorized = "ErrorUnauthorized"
	ErrorNotAdmin     = "ErrorNotAdmin"

	// Validation errors
	ErrorInvalidFormat  = "ErrorInvalidFormat"
	ErrorInvalidInput   = "ErrorInvalidInput"
	ErrorInvalidCommand = "ErrorInvalidCommand"

	// System errors
	ErrorGeneric            = "ErrorGeneric"
	ErrorDatabaseFailed     = "ErrorDatabaseFailed"
	ErrorNotificationFailed = "ErrorNotificationFailed"

	// Session errors
	ErrorSessionConflict = "ErrorSessionConflict"
	ErrorSessionNotFound = "ErrorSessionNotFound"

	// ============================================================================
	// CONFIRMATIONS AND ACTIONS
	// ============================================================================

	// Generic confirmations
	ConfirmYes    = "ConfirmYes"
	ConfirmNo     = "ConfirmNo"
	ConfirmCancel = "ConfirmCancel"

	// Session conflict
	SessionConflictMessage   = "SessionConflictMessage"
	SessionConflictContinue  = "SessionConflictContinue"
	SessionConflictRestart   = "SessionConflictRestart"
	SessionContinueMessage   = "SessionContinueMessage"
	SessionRestartMessage    = "SessionRestartMessage"
	SessionErrorUnknownType  = "SessionErrorUnknownType"
	SessionErrorDeleteFailed = "SessionErrorDeleteFailed"

	// Session type names
	SessionTypeEventCreation   = "SessionTypeEventCreation"
	SessionTypeGroupCreation   = "SessionTypeGroupCreation"
	SessionTypeEventResolution = "SessionTypeEventResolution"

	// Group reference
	GroupReferenceDefault = "GroupReferenceDefault"
	GroupReferenceNamed   = "GroupReferenceNamed"

	// ============================================================================
	// FORMATTING AND LABELS
	// ============================================================================

	// Common labels
	LabelGroup    = "LabelGroup"
	LabelType     = "LabelType"
	LabelQuestion = "LabelQuestion"
	LabelOptions  = "LabelOptions"
	LabelDeadline = "LabelDeadline"
	LabelStatus   = "LabelStatus"
	LabelPoints   = "LabelPoints"
	LabelAccuracy = "LabelAccuracy"
	LabelStreak   = "LabelStreak"

	// User display
	UserDisplayFormat = "UserDisplayFormat"
	UserIDFormat      = "UserIDFormat"

	// Date/time formatting
	DateFormatShort = "DateFormatShort"
	DateFormatLong  = "DateFormatLong"
	TimeRemaining   = "TimeRemaining"

	// ============================================================================
	// ADMIN COMMANDS
	// ============================================================================

	// Admin notifications
	AdminNotificationNewEvent      = "AdminNotificationNewEvent"
	AdminNotificationEventResolved = "AdminNotificationEventResolved"

	// Admin actions
	AdminActionLogged = "AdminActionLogged"

	// ============================================================================
	// HANDLER COMMANDS
	// ============================================================================

	// Help command
	HelpBotTitle         = "HelpBotTitle"
	HelpUserCommands     = "HelpUserCommands"
	HelpAdminCommands    = "HelpAdminCommands"
	HelpScoringRules     = "HelpScoringRules"
	HelpScoringCorrect   = "HelpScoringCorrect"
	HelpScoringBonuses   = "HelpScoringBonuses"
	HelpScoringPenalties = "HelpScoringPenalties"
	HelpAchievements     = "HelpAchievements"
	HelpEventTypes       = "HelpEventTypes"
	HelpVoteReminder     = "HelpVoteReminder"
	HelpDeadlineReminder = "HelpDeadlineReminder"

	// Rating command
	RatingTop10Title   = "RatingTop10Title"
	RatingGroupName    = "RatingGroupName"
	RatingMedalFirst   = "RatingMedalFirst"
	RatingMedalSecond  = "RatingMedalSecond"
	RatingMedalThird   = "RatingMedalThird"
	RatingPosition     = "RatingPosition"
	RatingUserPoints   = "RatingUserPoints"
	RatingUserAccuracy = "RatingUserAccuracy"
	RatingUserStreak   = "RatingUserStreak"
	RatingUserCorrect  = "RatingUserCorrect"
	RatingUserWrong    = "RatingUserWrong"

	// My stats command
	MyStatsTitle2          = "MyStatsTitle2"
	MyStatsGroupName       = "MyStatsGroupName"
	MyStatsPoints2         = "MyStatsPoints2"
	MyStatsCorrect2        = "MyStatsCorrect2"
	MyStatsWrong2          = "MyStatsWrong2"
	MyStatsAccuracy2       = "MyStatsAccuracy2"
	MyStatsCurrentStreak   = "MyStatsCurrentStreak"
	MyStatsTotalPreds      = "MyStatsTotalPreds"
	MyStatsAchievements    = "MyStatsAchievements"
	MyStatsNoAchievements2 = "MyStatsNoAchievements2"

	// Events command
	EventsActiveTitle              = "EventsActiveTitle"
	EventsNoActive                 = "EventsNoActive"
	EventsItemNumber               = "EventsItemNumber"
	EventsItemGroup                = "EventsItemGroup"
	EventsItemType                 = "EventsItemType"
	EventsItemOptions              = "EventsItemOptions"
	EventsItemVotes                = "EventsItemVotes"
	EventsItemTimeRemaining        = "EventsItemTimeRemaining"
	EventsItemTimeRemainingDays    = "EventsItemTimeRemainingDays"
	EventsItemTimeRemainingHours   = "EventsItemTimeRemainingHours"
	EventsItemTimeRemainingMinutes = "EventsItemTimeRemainingMinutes"
	EventsItemDeadlineExpired      = "EventsItemDeadlineExpired"
	EventsItemDeadlineFormat       = "EventsItemDeadlineFormat"

	// Groups command
	GroupsYourGroups       = "GroupsYourGroups"
	GroupsNoGroups         = "GroupsNoGroups"
	GroupsJoinInstructions = "GroupsJoinInstructions"
	GroupsItemNumber       = "GroupsItemNumber"
	GroupsItemMembers      = "GroupsItemMembers"
	GroupsItemJoined       = "GroupsItemJoined"

	// Deep link join
	DeepLinkInvalidLink     = "DeepLinkInvalidLink"
	DeepLinkGroupNotFound   = "DeepLinkGroupNotFound"
	DeepLinkAlreadyMember   = "DeepLinkAlreadyMember"
	DeepLinkWelcomeBack     = "DeepLinkWelcomeBack"
	DeepLinkWelcome         = "DeepLinkWelcome"
	DeepLinkErrorCheck      = "DeepLinkErrorCheck"
	DeepLinkErrorMembership = "DeepLinkErrorMembership"
	DeepLinkErrorReactivate = "DeepLinkErrorReactivate"
	DeepLinkErrorValidation = "DeepLinkErrorValidation"
	DeepLinkErrorCreate     = "DeepLinkErrorCreate"

	// Session conflict
	SessionConflictWarning        = "SessionConflictWarning"
	SessionConflictContinueButton = "SessionConflictContinueButton"
	SessionConflictRestartButton  = "SessionConflictRestartButton"
	SessionContinuePrevious       = "SessionContinuePrevious"
	SessionErrorDelete            = "SessionErrorDelete"
	SessionErrorUnknown           = "SessionErrorUnknown"

	// Event creation permission
	EventCreationPermissionDenied  = "EventCreationPermissionDenied"
	EventCreationErrorNoGroups     = "EventCreationErrorNoGroups"
	EventCreationErrorNoGroupsHelp = "EventCreationErrorNoGroupsHelp"
	EventCreationErrorStart        = "EventCreationErrorStart"

	// Event resolution
	EventResolutionTitle2       = "EventResolutionTitle2"
	EventResolutionSelectPrompt = "EventResolutionSelectPrompt"
	EventResolutionNoEvents     = "EventResolutionNoEvents"
	EventResolutionNoPermission = "EventResolutionNoPermission"
	EventResolutionErrorStart   = "EventResolutionErrorStart"
	EventResolutionErrorGroups  = "EventResolutionErrorGroups"

	// Edit event
	EditEventUnavailable = "EditEventUnavailable"

	// Create group
	CreateGroupForumDetected = "CreateGroupForumDetected"
	CreateGroupForumThreadID = "CreateGroupForumThreadID"
	CreateGroupPromptName    = "CreateGroupPromptName"
	CreateGroupErrorStart    = "CreateGroupErrorStart"
	CreateGroupErrorPrompt   = "CreateGroupErrorPrompt"

	// List groups
	ListGroupsTitle             = "ListGroupsTitle"
	ListGroupsEmpty             = "ListGroupsEmpty"
	ListGroupsItemNumber        = "ListGroupsItemNumber"
	ListGroupsItemMembers       = "ListGroupsItemMembers"
	ListGroupsItemLink          = "ListGroupsItemLink"
	ListGroupsItemID            = "ListGroupsItemID"
	ListGroupsItemType          = "ListGroupsItemType"
	ListGroupsItemForum         = "ListGroupsItemForum"
	ListGroupsItemTopics        = "ListGroupsItemTopics"
	ListGroupsItemNoTopics      = "ListGroupsItemNoTopics"
	ListGroupsItemDeleted       = "ListGroupsItemDeleted"
	ListGroupsButtonRenameGroup = "ListGroupsButtonRenameGroup"
	ListGroupsButtonRenameTopic = "ListGroupsButtonRenameTopic"
	ListGroupsButtonSoftDelete  = "ListGroupsButtonSoftDelete"
	ListGroupsButtonRestore     = "ListGroupsButtonRestore"
	ListGroupsButtonDeleteTopic = "ListGroupsButtonDeleteTopic"
	ListGroupsErrorGet          = "ListGroupsErrorGet"
	ListGroupsErrorSend         = "ListGroupsErrorSend"

	// Group members
	GroupMembersTitle            = "GroupMembersTitle"
	GroupMembersSelectGroup      = "GroupMembersSelectGroup"
	GroupMembersEmpty            = "GroupMembersEmpty"
	GroupMembersItemNumber       = "GroupMembersItemNumber"
	GroupMembersItemPoints       = "GroupMembersItemPoints"
	GroupMembersItemAchievements = "GroupMembersItemAchievements"
	GroupMembersItemJoined       = "GroupMembersItemJoined"
	GroupMembersErrorGet         = "GroupMembersErrorGet"
	GroupMembersErrorGroup       = "GroupMembersErrorGroup"
	GroupMembersErrorSend        = "GroupMembersErrorSend"

	// Remove member
	RemoveMemberTitle       = "RemoveMemberTitle"
	RemoveMemberSelectGroup = "RemoveMemberSelectGroup"
	RemoveMemberSelectUser  = "RemoveMemberSelectUser"
	RemoveMemberSuccess     = "RemoveMemberSuccess"
	RemoveMemberEmpty       = "RemoveMemberEmpty"
	RemoveMemberErrorGet    = "RemoveMemberErrorGet"
	RemoveMemberErrorGroup  = "RemoveMemberErrorGroup"
	RemoveMemberErrorUpdate = "RemoveMemberErrorUpdate"
	RemoveMemberErrorSend   = "RemoveMemberErrorSend"

	// Bot added to group
	BotAddedTitle                 = "BotAddedTitle"
	BotAddedBy                    = "BotAddedBy"
	BotAddedGroupName             = "BotAddedGroupName"
	BotAddedChatID                = "BotAddedChatID"
	BotAddedTypeForum             = "BotAddedTypeForum"
	BotAddedTypeRegular           = "BotAddedTypeRegular"
	BotAddedForumInstructions     = "BotAddedForumInstructions"
	BotAddedRegisterCommand       = "BotAddedRegisterCommand"
	BotAddedUserNotification      = "BotAddedUserNotification"
	BotAddedUserForumInstructions = "BotAddedUserForumInstructions"
	BotAddedUserRegisterCommand   = "BotAddedUserRegisterCommand"

	// Leave group
	LeaveGroupButton  = "LeaveGroupButton"
	LeaveGroupSuccess = "LeaveGroupSuccess"
	LeaveGroupError   = "LeaveGroupError"

	// Delete group
	DeleteGroupTitle        = "DeleteGroupTitle"
	DeleteGroupSelectPrompt = "DeleteGroupSelectPrompt"
	DeleteGroupEmpty        = "DeleteGroupEmpty"
	DeleteGroupSuccess      = "DeleteGroupSuccess"
	DeleteGroupError        = "DeleteGroupError"

	// Delete topic
	DeleteTopicTitle        = "DeleteTopicTitle"
	DeleteTopicSelectForum  = "DeleteTopicSelectForum"
	DeleteTopicSelectPrompt = "DeleteTopicSelectPrompt"
	DeleteTopicEmpty        = "DeleteTopicEmpty"
	DeleteTopicNoTopics     = "DeleteTopicNoTopics"
	DeleteTopicSuccess      = "DeleteTopicSuccess"
	DeleteTopicError        = "DeleteTopicError"

	// Soft delete group
	SoftDeleteGroupTitle        = "SoftDeleteGroupTitle"
	SoftDeleteGroupSelectPrompt = "SoftDeleteGroupSelectPrompt"
	SoftDeleteGroupEmpty        = "SoftDeleteGroupEmpty"
	SoftDeleteGroupSuccess      = "SoftDeleteGroupSuccess"
	SoftDeleteGroupError        = "SoftDeleteGroupError"

	// Restore group
	RestoreGroupTitle        = "RestoreGroupTitle"
	RestoreGroupSelectPrompt = "RestoreGroupSelectPrompt"
	RestoreGroupEmpty        = "RestoreGroupEmpty"
	RestoreGroupSuccess      = "RestoreGroupSuccess"
	RestoreGroupError        = "RestoreGroupError"

	// Rename group
	RenameGroupTitle        = "RenameGroupTitle"
	RenameGroupSelectPrompt = "RenameGroupSelectPrompt"
	RenameGroupEmpty        = "RenameGroupEmpty"
	RenameGroupPrompt       = "RenameGroupPrompt"
	RenameGroupErrorStart   = "RenameGroupErrorStart"
	RenameGroupErrorGet     = "RenameGroupErrorGet"
	RenameGroupErrorSend    = "RenameGroupErrorSend"

	// Rename topic
	RenameTopicTitle        = "RenameTopicTitle"
	RenameTopicSelectForum  = "RenameTopicSelectForum"
	RenameTopicSelectPrompt = "RenameTopicSelectPrompt"
	RenameTopicEmpty        = "RenameTopicEmpty"
	RenameTopicNoTopics     = "RenameTopicNoTopics"
	RenameTopicPrompt       = "RenameTopicPrompt"
	RenameTopicErrorStart   = "RenameTopicErrorStart"
	RenameTopicErrorGet     = "RenameTopicErrorGet"
	RenameTopicErrorSend    = "RenameTopicErrorSend"

	// FSM errors
	FSMErrorRestart       = "FSMErrorRestart"
	FSMErrorRestartEvent  = "FSMErrorRestartEvent"
	FSMErrorRestartGroup  = "FSMErrorRestartGroup"
	FSMErrorRestartRename = "FSMErrorRestartRename"
	FSMErrorRestartEdit   = "FSMErrorRestartEdit"

	// ============================================================================
	// MISCELLANEOUS
	// ============================================================================

	// Generic messages
	MessageSuccess = "MessageSuccess"
	MessageFailed  = "MessageFailed"
	MessageLoading = "MessageLoading"

	// Navigation
	NavigationBack   = "NavigationBack"
	NavigationNext   = "NavigationNext"
	NavigationCancel = "NavigationCancel"

	// ============================================================================
	// ADDITIONAL HANDLER MESSAGES (from hardcoded strings cleanup)
	// ============================================================================

	// Edit event errors
	ErrorEditEventNoPermission = "ErrorEditEventNoPermission"
	ErrorEditEventHasVotes     = "ErrorEditEventHasVotes"
	ErrorEditEventStart        = "ErrorEditEventStart"

	// Group creation messages
	GroupCreationForumDetectedFull = "GroupCreationForumDetectedFull"
	GroupCreationPromptName        = "GroupCreationPromptName"

	// List groups formatting
	ListGroupsItemMembersFormat = "ListGroupsItemMembersFormat"
	ListGroupsItemLinkFormat    = "ListGroupsItemLinkFormat"
	ListGroupsItemTypeFormat    = "ListGroupsItemTypeFormat"
	ListGroupsItemTopicsHeader  = "ListGroupsItemTopicsHeader"
	ListGroupsLinkError         = "ListGroupsLinkError"

	// Group members formatting
	GroupMembersItemPointsFormat       = "GroupMembersItemPointsFormat"
	GroupMembersItemAchievementsFormat = "GroupMembersItemAchievementsFormat"
	GroupMembersItemJoinedFormat       = "GroupMembersItemJoinedFormat"

	// User groups formatting
	GroupsItemMembersFormat = "GroupsItemMembersFormat"
	GroupsItemJoinedFormat  = "GroupsItemJoinedFormat"

	// Bot added messages (additional formats)
	BotAddedGroupNameFormat        = "BotAddedGroupNameFormat"
	BotAddedChatIDFormat           = "BotAddedChatIDFormat"
	BotAddedForumInstructionsStep1 = "BotAddedForumInstructionsStep1"
	BotAddedForumInstructionsStep2 = "BotAddedForumInstructionsStep2"
	BotAddedForumInstructionsStep3 = "BotAddedForumInstructionsStep3"
	BotAddedForumSetup             = "BotAddedForumSetup"
	BotAddedForumEvents            = "BotAddedForumEvents"

	// User notification
	BotAddedUserNameFormat   = "BotAddedUserNameFormat"
	BotAddedUserChatIDFormat = "BotAddedUserChatIDFormat"
	BotAddedUserForumStep1   = "BotAddedUserForumStep1"
	BotAddedUserForumStep2   = "BotAddedUserForumStep2"
	BotAddedUserForumStep3   = "BotAddedUserForumStep3"
	BotAddedUserForumEvents  = "BotAddedUserForumEvents"

	// Remove member
	RemoveMemberSuccessFormat = "RemoveMemberSuccessFormat"

	// Errors
	ErrorInvalidEventID    = "ErrorInvalidEventID"
	ErrorInvalidDataFormat = "ErrorInvalidDataFormat"
	ErrorInvalidChatID     = "ErrorInvalidChatID"
	ErrorRequestProcessing = "ErrorRequestProcessing"
	ErrorTopicNotFound     = "ErrorTopicNotFound"

	// Group status messages
	GroupEmptyMembers       = "GroupEmptyMembers"
	GroupEmptyActiveMembers = "GroupEmptyActiveMembers"
	GroupMarkedDeleted      = "GroupMarkedDeleted"
	GroupRestored           = "GroupRestored"

	// Prompts with group name
	GroupMembersTitleWithName  = "GroupMembersTitleWithName"
	RemoveMemberPromptWithName = "RemoveMemberPromptWithName"
	RenameGroupPromptWithName  = "RenameGroupPromptWithName"
	DeleteTopicPromptWithName  = "DeleteTopicPromptWithName"
	RenameTopicPromptWithName  = "RenameTopicPromptWithName"

	// Success messages with names
	GroupDeletedSuccess = "GroupDeletedSuccess"
	TopicDeletedSuccess = "TopicDeletedSuccess"
)
