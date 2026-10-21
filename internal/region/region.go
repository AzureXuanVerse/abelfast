package region

import (
	"fmt"
	"slices"
	"strings"
)

const Default = "EN"

// All are the client regions, as AzurLaneLuaScripts names them: the game
// data dir has one directory per region (<data dir>/<region>).
var All = []string{"EN", "CN", "JP", "KR", "TW"}

var current = Default

func Current() string {
	return current
}

// Set selects the region, case-insensitively, before anything reads the game
// data.
func Set(name string) error {
	name = strings.ToUpper(name)
	if !slices.Contains(All, name) {
		return fmt.Errorf("unknown region %q, want one of %s", name, strings.Join(All, ", "))
	}
	current = name
	return nil
}
