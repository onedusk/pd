// Package skilldata embeds the decompose skill files for distribution
// inside the decompose binary. The embedded filesystem is rooted at
// "skill/decompose/" and contains SKILL.md, references/, and
// assets/templates/.
package skilldata

import "embed"

// SkillFS contains the embedded skill files. Walk from "skill/decompose"
// to iterate over all files.
//
//go:embed all:skill
var SkillFS embed.FS

// HooksFS contains the embedded hook scripts installed alongside the skill.
//
//go:embed hooks/*
var HooksFS embed.FS
