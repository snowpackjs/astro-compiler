package helpers

import (
	"strings"
)

func peekIs(input string, cur int, assert byte) bool {
	return cur+1 < len(input) && input[cur+1] == assert
}

// RemoveComments removes both block and inline comments from a string
func RemoveComments(input string) string {
	var (
		sb        = strings.Builder{}
		inComment = false
	)
	for cur := 0; cur < len(input); cur++ {
		if input[cur] == '/' && !inComment {
			if peekIs(input, cur, '*') {
				inComment = true
				cur++
			} else if peekIs(input, cur, '/') {
				// Skip until the end of line for inline comments
				for cur < len(input) && input[cur] != '\n' {
					cur++
				}
				continue
			}
		} else if input[cur] == '*' && inComment && peekIs(input, cur, '/') {
			inComment = false
			cur++
			continue
		}

		if !inComment {
			sb.WriteByte(input[cur])
		}
	}

	if inComment {
		return ""
	}

	return strings.TrimSpace(sb.String())
}
