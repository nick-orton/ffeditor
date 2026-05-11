package main

import (
	"path/filepath"
	"strings"
)

type extSet map[string]struct{}

func (s extSet) contains(name string) bool {
	_, ok := s[strings.ToLower(filepath.Ext(name))]
	return ok
}

// audioExts is the full set of audio formats recognised by the browser.
var audioExts = extSet{
	".mp3": {}, ".flac": {},
	".opus": {}, ".m4a": {}, ".ogg": {}, ".aac": {}, ".wav": {},
}

// convertibleExts are accepted input formats that can be converted to a blessed format.
var convertibleExts = extSet{".opus": {}, ".m4a": {}, ".ogg": {}, ".aac": {}, ".wav": {}}

// blessedExts are final/editable output formats.
var blessedExts = extSet{".mp3": {}, ".flac": {}}

func isAudio(name string) bool       { return audioExts.contains(name) }
func isConvertible(name string) bool { return convertibleExts.contains(name) }
func isBlessed(name string) bool     { return blessedExts.contains(name) }
