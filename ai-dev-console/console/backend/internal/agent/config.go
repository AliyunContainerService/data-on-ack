/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"os"
	"strconv"
	"time"
)

// Config holds all agent-runtime settings. Everything is sourced from
// environment variables (injected by the Helm chart) so that the agent stays
// an optional add-on: when it is disabled or misconfigured the rest of the
// console is completely unaffected.
type Config struct {
	Enabled bool

	// Model settings.
	Provider string // "dashscope" (default) or "openai-compatible"
	Model    string // default qwen-plus: diagnosis contexts are token-heavy
	APIKey   string
	BaseURL  string

	// Budget caps (design §5.2: the framework has no built-in spend cap).
	MaxDailyCostUSDPerUser float64
	MaxRoundsPerRun        int
	MaxToolCallsPerRun     int
	MaxActiveRunsPerUser   int
	MaxSessionsPerUser     int

	// Lifecycle limits (design §3.1: runs are server-side objects).
	RunTimeout     time.Duration
	ConfirmTimeout time.Duration
	SessionTTL     time.Duration

	// Per-model token price (USD per 1M tokens) used by the budget tracker.
	PriceInputPerMillion  float64
	PriceOutputPerMillion float64
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// LoadConfig reads the agent configuration from the environment.
func LoadConfig() *Config {
	return &Config{
		Enabled: os.Getenv("AGENT_ENABLED") == "true",

		Provider: envStr("AGENT_MODEL_PROVIDER", "dashscope"),
		Model:    envStr("AGENT_MODEL_NAME", "qwen-plus"),
		APIKey:   os.Getenv("AGENT_MODEL_API_KEY"),
		BaseURL:  os.Getenv("AGENT_MODEL_BASE_URL"),

		// Non-zero defaults are mandatory by design: an agent without budget
		// caps is a cost/DoS amplifier.
		MaxDailyCostUSDPerUser: envFloat("AGENT_MAX_DAILY_COST_USD", 2.0),
		MaxRoundsPerRun:        envInt("AGENT_MAX_ROUNDS_PER_RUN", 12),
		MaxToolCallsPerRun:     envInt("AGENT_MAX_TOOL_CALLS_PER_RUN", 40),
		MaxActiveRunsPerUser:   envInt("AGENT_MAX_ACTIVE_RUNS_PER_USER", 2),
		MaxSessionsPerUser:     envInt("AGENT_MAX_SESSIONS_PER_USER", 5),

		RunTimeout: envDuration("AGENT_RUN_TIMEOUT", 30*time.Minute),
		// Keep below RunTimeout: the run context is the outer bound; the watchdog
		// must deny first so confirmations never dangle past the run window.
		ConfirmTimeout: envDuration("AGENT_CONFIRM_TIMEOUT", 25*time.Minute),
		SessionTTL:     envDuration("AGENT_SESSION_TTL", 24*time.Hour),

		// Approximate public pricing for the default model; override via chart.
		PriceInputPerMillion:  envFloat("AGENT_PRICE_INPUT_PER_M", 0.4),
		PriceOutputPerMillion: envFloat("AGENT_PRICE_OUTPUT_PER_M", 1.2),
	}
}

// Available reports whether the agent runtime can actually start: it must be
// explicitly enabled AND have a usable model configuration. The console hides
// all agent entry points when this is false (edge/offline clusters).
func (c *Config) Available() bool {
	return c.Enabled && c.APIKey != ""
}
