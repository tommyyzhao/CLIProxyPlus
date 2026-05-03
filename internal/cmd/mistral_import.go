package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

// MistralImportOptions controls the mistral-import flow.
type MistralImportOptions struct {
	// APIKey, when non-empty, overrides discovery.
	APIKey string
	// VibeEnvPath overrides the default ~/.vibe/.env lookup path.
	VibeEnvPath string
	// Label customizes the saved file name; defaults to a timestamp.
	Label string
}

// DoMistralImport reads a Mistral API key produced by the mistral-vibe browser
// sign-in flow and writes a self-contained auth file that the file synthesizer
// routes through the existing OpenAI-compatibility executor.
//
// Discovery order: APIKey option > MISTRAL_API_KEY env var > ~/.vibe/.env (or
// $VIBE_HOME/.env). The first non-empty value wins.
func DoMistralImport(cfg *config.Config, opts *MistralImportOptions) {
	if cfg == nil {
		log.Error("Mistral import: configuration is required")
		return
	}
	if opts == nil {
		opts = &MistralImportOptions{}
	}

	apiKey := strings.TrimSpace(opts.APIKey)
	source := "--api-key flag"
	if apiKey == "" {
		if envKey := strings.TrimSpace(os.Getenv("MISTRAL_API_KEY")); envKey != "" {
			apiKey = envKey
			source = "MISTRAL_API_KEY env var"
		}
	}
	if apiKey == "" {
		envPath, err := resolveVibeEnvPath(opts.VibeEnvPath)
		if err != nil {
			log.Errorf("Mistral import: %v", err)
			return
		}
		envMap, err := godotenv.Read(envPath)
		if err != nil {
			log.Errorf("Mistral import: read %s failed: %v", envPath, err)
			return
		}
		apiKey = strings.TrimSpace(envMap["MISTRAL_API_KEY"])
		source = envPath
	}

	if apiKey == "" {
		log.Error("Mistral import: no API key found. Run `vibe` to sign in, or pass --api-key.")
		return
	}

	authDir := strings.TrimSpace(cfg.AuthDir)
	if authDir == "" {
		log.Error("Mistral import: cfg.AuthDir is empty; cannot write auth file")
		return
	}
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		log.Errorf("Mistral import: create auth dir failed: %v", err)
		return
	}

	label := strings.TrimSpace(opts.Label)
	if label == "" {
		label = fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	fileName := fmt.Sprintf("mistral-%s.json", label)
	fullPath := filepath.Join(authDir, fileName)

	metadata := map[string]any{
		"type":         "mistral",
		"api_key":      apiKey,
		"base_url":     "https://api.mistral.ai/v1",
		"compat_name":  "mistral",
		"provider_key": "openai-compatibility",
		"timestamp":    time.Now().UnixMilli(),
	}

	payload, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Errorf("Mistral import: marshal failed: %v", err)
		return
	}
	if err := os.WriteFile(fullPath, payload, 0o600); err != nil {
		log.Errorf("Mistral import: write %s failed: %v", fullPath, err)
		return
	}

	fmt.Printf("Mistral API key imported from %s\n", source)
	fmt.Printf("Auth file written to %s\n", fullPath)
	fmt.Println("Default models served via openai-compatibility:")
	fmt.Println("  - mistral-medium-3.5  (alias for mistral-vibe-cli-latest)")
	fmt.Println("  - devstral-small      (alias for devstral-small-latest)")
	fmt.Println("Customize models or aliases by adding a matching openai-compatibility entry in config.yaml.")
}

func resolveVibeEnvPath(override string) (string, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed, nil
	}
	if vibeHome := strings.TrimSpace(os.Getenv("VIBE_HOME")); vibeHome != "" {
		return filepath.Join(vibeHome, ".env"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".vibe", ".env"), nil
}
