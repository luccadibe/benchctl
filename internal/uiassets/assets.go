package uiassets

import (
	"embed"
	_ "embed"
)

//go:embed dist/*
var Dist embed.FS
