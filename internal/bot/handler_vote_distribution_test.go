package bot

import (
	"testing"
	"time"

	"telegram-prediction-bot/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: telegram-prediction-bot, Property 30: Vote percentage calculation
// Validates: Requirements 14.2
func TestVotePercentageCalculation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("displayed percentage for each option equals (votes for option / total votes) * 100", prop.ForAll(
		func(numOptions int, votes []int) bool {
			// Skip invalid inputs
			if numOptions < 2 || numOptions > 6 {
				return true
			}
			if len(votes) == 0 {
				return true
			}

			// Create mock predictions based on votes
			// votes[i] represents which option was selected for vote i
			var predictions []*domain.Prediction
			for i, optionIndex := range votes {
				// Ensure option index is within valid range
				if optionIndex < 0 || optionIndex >= numOptions {
					return true // Skip invalid option indices
				}

				predictions = append(predictions, &domain.Prediction{
					ID:        int64(i + 1),
					EventID:   1,
					UserID:    int64(i + 1),
					Option:    optionIndex,
					Timestamp: time.Now(),
				})
			}

			// Create a mock handler (we only need the method, not the full handler)
			handler := &BotHandler{}

			// Calculate vote distribution
			distribution := handler.calculateVoteDistribution(predictions, numOptions)

			// Verify the calculation
			// Count actual votes for each option
			actualCounts := make(map[int]int)
			for _, pred := range predictions {
				actualCounts[pred.Option]++
			}

			totalVotes := float64(len(predictions))

			// Check each option's percentage
			for option := 0; option < numOptions; option++ {
				expectedPercentage := 0.0
				if totalVotes > 0 {
					expectedPercentage = (float64(actualCounts[option]) / totalVotes) * 100.0
				}

				actualPercentage := distribution[option]

				// Allow small floating point error (0.01%)
				diff := actualPercentage - expectedPercentage
				if diff < -0.01 || diff > 0.01 {
					t.Logf("Percentage mismatch for option %d: expected %.2f%%, got %.2f%%",
						option, expectedPercentage, actualPercentage)
					return false
				}
			}

			// Verify all percentages sum to approximately 100% (or 0% if no votes)
			totalPercentage := 0.0
			for _, percentage := range distribution {
				totalPercentage += percentage
			}

			expectedTotal := 0.0
			if len(predictions) > 0 {
				expectedTotal = 100.0
			}

			diff := totalPercentage - expectedTotal
			if diff < -0.01 || diff > 0.01 {
				t.Logf("Total percentage mismatch: expected %.2f%%, got %.2f%%",
					expectedTotal, totalPercentage)
				return false
			}

			return true
		},
		gen.IntRange(2, 6),              // numOptions: 2-6 options
		gen.SliceOf(gen.IntRange(0, 5)), // votes: array of option indices
	))

	properties.TestingRun(t)
}

// Additional property test: Empty predictions should return all zeros
func TestVotePercentageCalculationEmptyPredictions(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("empty predictions return 0% for all options", prop.ForAll(
		func(numOptions int) bool {
			// Skip invalid inputs
			if numOptions < 2 || numOptions > 6 {
				return true
			}

			handler := &BotHandler{}
			distribution := handler.calculateVoteDistribution([]*domain.Prediction{}, numOptions)

			// All options should have 0%
			for option := 0; option < numOptions; option++ {
				if distribution[option] != 0.0 {
					t.Logf("Expected 0%% for option %d with no votes, got %.2f%%",
						option, distribution[option])
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 6),
	))

	properties.TestingRun(t)
}

// Additional property test: Single vote should be 100%
func TestVotePercentageCalculationSingleVote(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("single vote for an option results in 100% for that option", prop.ForAll(
		func(numOptions int, selectedOption int) bool {
			// Skip invalid inputs
			if numOptions < 2 || numOptions > 6 {
				return true
			}
			if selectedOption < 0 || selectedOption >= numOptions {
				return true
			}

			predictions := []*domain.Prediction{
				{
					ID:        1,
					EventID:   1,
					UserID:    1,
					Option:    selectedOption,
					Timestamp: time.Now(),
				},
			}

			handler := &BotHandler{}
			distribution := handler.calculateVoteDistribution(predictions, numOptions)

			// Selected option should have 100%
			if distribution[selectedOption] != 100.0 {
				t.Logf("Expected 100%% for option %d, got %.2f%%",
					selectedOption, distribution[selectedOption])
				return false
			}

			// All other options should have 0%
			for option := 0; option < numOptions; option++ {
				if option != selectedOption && distribution[option] != 0.0 {
					t.Logf("Expected 0%% for option %d, got %.2f%%",
						option, distribution[option])
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 6),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}
