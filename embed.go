package smolbot

import "embed"

// EmbeddedAssets contains the builtin template and skill files shipped with nanobot-go.
//
//go:embed templates/** skills/**
var EmbeddedAssets embed.FS
