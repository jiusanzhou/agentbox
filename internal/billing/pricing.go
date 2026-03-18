package billing

import (
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// Token costs in micros (1/1,000,000 USD).
// Defaults approximate Claude Sonnet pricing.
const (
	InputTokenCostMicrosPer1M  int64 = 3_000_000  // $3.00 / 1M input tokens
	OutputTokenCostMicrosPer1M int64 = 15_000_000 // $15.00 / 1M output tokens

	// Compute cost: $0.10 / minute = 1_667 micros/second
	ComputeCostMicrosPerSec int64 = 1_667
)

// ComputeRunCost calculates the full cost breakdown for a completed run.
// All costs are returned in micros (1/1,000,000 USD).
func ComputeRunCost(
	runID string,
	pricingModel string,
	pricePerUnit float64,
	duration time.Duration,
	inputTokens, outputTokens int64,
	revenueShare model.RevenueShareConfig,
) model.RunCostBreakdown {
	durationMs := duration.Milliseconds()
	durationSec := duration.Seconds()

	// Infra/compute cost (platform cost, always charged)
	computeMicros := int64(durationSec * float64(ComputeCostMicrosPerSec))

	// LLM token cost (platform cost)
	tokenMicros := (inputTokens * InputTokenCostMicrosPer1M / 1_000_000) +
		(outputTokens * OutputTokenCostMicrosPer1M / 1_000_000)

	// Agent fee (revenue to creator, based on pricing model)
	priceUnitMicros := int64(pricePerUnit * 1_000_000)
	var agentFeeMicros int64
	switch pricingModel {
	case "per_task", "one-time":
		// Flat fee per task execution
		agentFeeMicros = priceUnitMicros
	case "per_minute":
		// Proportional to run duration
		minutes := duration.Minutes()
		agentFeeMicros = int64(minutes * float64(priceUnitMicros))
	case "subscription", "flat_monthly":
		// No per-run agent fee; subscription already covers creator revenue
		agentFeeMicros = 0
	default:
		// Free
		agentFeeMicros = 0
	}

	// Revenue share breakdown of agent fee
	var creatorEarningsMicros, platformFeeMicros int64
	if agentFeeMicros > 0 {
		creatorEarningsMicros = agentFeeMicros * int64(revenueShare.AuthorPercent) / 100
		platformFeeMicros = agentFeeMicros - creatorEarningsMicros
	}

	totalMicros := computeMicros + tokenMicros + agentFeeMicros

	return model.RunCostBreakdown{
		RunID:                 runID,
		PricingModel:         pricingModel,
		DurationMs:           durationMs,
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
		ComputeMicros:        computeMicros,
		TokenMicros:          tokenMicros,
		AgentFeeMicros:       agentFeeMicros,
		CreatorEarningsMicros: creatorEarningsMicros,
		PlatformFeeMicros:    platformFeeMicros,
		TotalMicros:          totalMicros,
	}
}

// MicrosToUSD converts micros to a human-readable USD string.
func MicrosToUSD(micros int64) float64 {
	return float64(micros) / 1_000_000
}
