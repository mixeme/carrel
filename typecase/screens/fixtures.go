package screens

import "carrel.dev/typecase/widget"

var demoBuild = widget.Build{
	Name:    "Carrel",
	Version: "0.0.1",
	Commit:  "786949a",
	Href:    "#rest",
}

func intPtr(n int) *int { return &n }
