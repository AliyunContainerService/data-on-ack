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

// Package agent integrates agentscope-go into the dev console backend.
//
// This is the build-verified scaffold for the "agentic console" design
// (see docs/design/agentic-console.md). Full capability implementation
// follows the approved design.
package agent

import (
	"fmt"
	"os"

	asmodel "github.com/alanfokco/agentscope-go/v2/pkg/agentscope/model"
)

// ModelConfig selects and configures the chat model used by console agents.
type ModelConfig struct {
	// Provider: "dashscope" | "openai-compatible"
	Provider string
	// Model name, e.g. "qwen-max", "qwen-plus".
	ModelName string
	// APIKey for the provider. Falls back to env AGENT_MODEL_API_KEY.
	APIKey string
	// BaseURL for OpenAI-compatible gateways (optional).
	BaseURL string
}

// BuildChatModel constructs the agentscope ChatModel for console agents.
func BuildChatModel(cfg ModelConfig) (asmodel.ChatModel, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("AGENT_MODEL_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("agent model api key not configured (set AGENT_MODEL_API_KEY)")
	}

	// SecretAPIKey (SecretStr) is preferred over the plain APIKey field so
	// the key is never rendered in logs or struct dumps.
	switch cfg.Provider {
	case "", "dashscope":
		return asmodel.NewDashScopeChatModel(asmodel.DashScopeConfig{
			Model:        cfg.ModelName,
			SecretAPIKey: asmodel.NewSecretStr(apiKey),
			BaseURL:      cfg.BaseURL,
		})
	case "openai-compatible", "openai":
		return asmodel.NewOpenAIChatModel(asmodel.OpenAIConfig{
			Model:        cfg.ModelName,
			SecretAPIKey: asmodel.NewSecretStr(apiKey),
			BaseURL:      cfg.BaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported agent model provider: %s", cfg.Provider)
	}
}
