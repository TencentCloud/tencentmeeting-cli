package utils

import (
	"tmeet/internal/exception"
	"unicode/utf8"
)

// CharacterLimit character limit
func CharacterLimit(flag, str string, limit int) error {
	if n := utf8.RuneCountInString(str); n > limit {
		return exception.InvalidArgsError.With("flag:%s, character limit exceeded, limit: %d, actual: %d", flag, limit, n)
	}
	return nil
}

// CharacterWidthLimit character width limit: ASCII counts as 1, non-ASCII (e.g. Chinese) counts as 2.
func CharacterWidthLimit(flag, str string, limit int) error {
	width := 0
	for _, r := range str {
		if r < utf8.RuneSelf {
			width++
		} else {
			width += 2
		}
	}
	if width > limit {
		return exception.InvalidArgsError.With("flag:%s, character width limit exceeded, limit: %d, actual: %d", flag, limit, width)
	}
	return nil
}
