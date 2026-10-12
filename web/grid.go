package web

import "strings"

// squareClass maps share-grid emoji to palette squares; the colour keeps each
// game's meaning (red/green right, yellow/orange close, grey miss).
var squareClass = map[rune]string{
	'🟥': "sq red", '🟩': "sq green", '✅': "sq green",
	'🟨': "sq yellow", '🟧': "sq orange", '⬛': "sq miss", '⬜': "sq empty",
}

// squares turns a stored grid into rows of square classes, or nil when the
// grid holds anything else, so the template shows the emoji text instead.
func squares(grid string) [][]string {
	if grid == "" {
		return nil
	}
	var rows [][]string
	for _, line := range strings.Split(grid, "\n") {
		var row []string
		for _, r := range line {
			if r == '️' {
				continue
			}
			c, ok := squareClass[r]
			if !ok {
				return nil
			}
			row = append(row, c)
		}
		rows = append(rows, row)
	}
	return rows
}
