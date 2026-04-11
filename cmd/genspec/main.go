// Command genspec writes the generated v7 spec to:
//   - docs/developer/block-spec.json  (raw JSON — used by the HTML tools and docs)
//   - docs/developer/block-spec.md    (Markdown wrapper embedding the JSON for the docs site)
//
// Run via: go run ./cmd/genspec
// Or via:  go generate ./internal/specgen/
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nathanhollows/Rapua/v7/internal/specgen"
)

const mdHeader = `---
title: "v7 Block Spec"
sidebar: true
order: 21
---

# v7 Block Spec

> This file is generated from code. Do not edit by hand.
> Regenerate with: ` + "`go run ./cmd/genspec`" + `

` + "```json\n"

const mdFooter = "\n```\n"

func main() {
	data, err := specgen.GenerateJSON()
	if err != nil {
		log.Fatalf("genspec: generate: %v", err)
	}

	// Write raw JSON — the single source of truth consumed by HTML tools.
	jsonOut := filepath.Join("docs", "developer", "block-spec.json")
	if err := os.WriteFile(jsonOut, data, 0o644); err != nil {
		log.Fatalf("genspec: write %s: %v", jsonOut, err)
	}
	log.Printf("wrote %s (%d bytes)", jsonOut, len(data))

	// Write Markdown wrapper that embeds the same JSON for the docs site.
	md := fmt.Sprintf("%s%s%s", mdHeader, data, mdFooter)
	mdOut := filepath.Join("docs", "developer", "block-spec.md")
	if err := os.WriteFile(mdOut, []byte(md), 0o644); err != nil {
		log.Fatalf("genspec: write %s: %v", mdOut, err)
	}
	log.Printf("wrote %s (%d bytes)", mdOut, len(md))
}
