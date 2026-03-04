package linkup

// canConnect checks if two cells can be connected
func canConnect(board [][]int, rows, cols int, cell1, cell2 Cell) bool {
	// Check if cells are valid
	if !isValidCell(cell1, rows, cols) || !isValidCell(cell2, rows, cols) {
		return false
	}

	// Check if cells are the same
	if cell1.Row == cell2.Row && cell1.Col == cell2.Col {
		return false
	}

	// Check if both cells have the same icon type and are not empty
	if board[cell1.Row][cell1.Col] != board[cell2.Row][cell2.Col] {
		return false
	}
	if board[cell1.Row][cell1.Col] == 0 || board[cell2.Row][cell2.Col] == 0 {
		return false
	}

	// Try different connection types
	return canConnectStraight(board, rows, cols, cell1, cell2) ||
		canConnectOneCorner(board, rows, cols, cell1, cell2) ||
		canConnectTwoCorners(board, rows, cols, cell1, cell2)
}

// isValidCell checks if a cell is within board boundaries
func isValidCell(cell Cell, rows, cols int) bool {
	return cell.Row >= 0 && cell.Row < rows && cell.Col >= 0 && cell.Col < cols
}

// canConnectStraight checks if two cells can be connected with a straight line (0 corners)
func canConnectStraight(board [][]int, rows, cols int, cell1, cell2 Cell) bool {
	// Check horizontal line (same row)
	if cell1.Row == cell2.Row {
		startCol := min(cell1.Col, cell2.Col)
		endCol := max(cell1.Col, cell2.Col)
		for col := startCol + 1; col < endCol; col++ {
			if board[cell1.Row][col] != 0 {
				return false
			}
		}
		return true
	}

	// Check vertical line (same column)
	if cell1.Col == cell2.Col {
		startRow := min(cell1.Row, cell2.Row)
		endRow := max(cell1.Row, cell2.Row)
		for row := startRow + 1; row < endRow; row++ {
			if board[row][cell1.Col] != 0 {
				return false
			}
		}
		return true
	}

	return false
}

// canConnectOneCorner checks if two cells can be connected with one corner
func canConnectOneCorner(board [][]int, rows, cols int, cell1, cell2 Cell) bool {
	// Try corner at (cell1.Row, cell2.Col)
	corner1 := Cell{Row: cell1.Row, Col: cell2.Col}
	if board[corner1.Row][corner1.Col] == 0 {
		if canConnectStraight(board, rows, cols, cell1, corner1) &&
			canConnectStraight(board, rows, cols, corner1, cell2) {
			return true
		}
	}

	// Try corner at (cell2.Row, cell1.Col)
	corner2 := Cell{Row: cell2.Row, Col: cell1.Col}
	if board[corner2.Row][corner2.Col] == 0 {
		if canConnectStraight(board, rows, cols, cell1, corner2) &&
			canConnectStraight(board, rows, cols, corner2, cell2) {
			return true
		}
	}

	return false
}

// canConnectTwoCorners checks if two cells can be connected with two corners
func canConnectTwoCorners(board [][]int, rows, cols int, cell1, cell2 Cell) bool {
	// Standard two-corner algorithm for LianLianKan:
	// We need to find a path: cell1 -> corner1 -> corner2 -> cell2
	// where corner1 and corner2 are empty cells, and:
	// 1. cell1 to corner1 is straight line (no obstacles)
	// 2. corner1 to corner2 is straight line (no obstacles)
	// 3. corner2 to cell2 is straight line (no obstacles)
	// 4. The lines are perpendicular (forming an "L" shape with two corners)

	// Approach: Find two empty cells that form a rectangle with cell1 and cell2
	// where all sides are straight lines with no obstacles

	// Check all possible corner1 positions (empty cells that can connect straight to cell1)
	for corner1Row := 0; corner1Row < rows; corner1Row++ {
		for corner1Col := 0; corner1Col < cols; corner1Col++ {
			corner1 := Cell{Row: corner1Row, Col: corner1Col}

			// Skip if corner1 is not empty
			if board[corner1Row][corner1Col] != 0 {
				continue
			}

			// Check if cell1 can connect straight to corner1
			if !canConnectStraight(board, rows, cols, cell1, corner1) {
				continue
			}

			// Now find corner2 that can connect straight to corner1 and straight to cell2
			// The lines must be perpendicular: if cell1->corner1 is horizontal, then corner1->corner2 must be vertical
			// and corner2->cell2 must be horizontal (or vice versa)

			// Determine the direction from cell1 to corner1
			sameRow := cell1.Row == corner1.Row
			sameCol := cell1.Col == corner1.Col

			if sameRow {
				// Horizontal line from cell1 to corner1
				// Need vertical line from corner1 to corner2, then horizontal line from corner2 to cell2
				// So corner2 must have same column as corner1, and same row as cell2
				corner2 := Cell{Row: cell2.Row, Col: corner1.Col}
				if isValidCell(corner2, rows, cols) && board[corner2.Row][corner2.Col] == 0 {
					// Check if corner1 can connect straight to corner2 (vertical)
					// and corner2 can connect straight to cell2 (horizontal)
					if canConnectStraight(board, rows, cols, corner1, corner2) &&
					   canConnectStraight(board, rows, cols, corner2, cell2) {
						return true
					}
				}
			} else if sameCol {
				// Vertical line from cell1 to corner1
				// Need horizontal line from corner1 to corner2, then vertical line from corner2 to cell2
				// So corner2 must have same row as corner1, and same column as cell2
				corner2 := Cell{Row: corner1.Row, Col: cell2.Col}
				if isValidCell(corner2, rows, cols) && board[corner2.Row][corner2.Col] == 0 {
					// Check if corner1 can connect straight to corner2 (horizontal)
					// and corner2 can connect straight to cell2 (vertical)
					if canConnectStraight(board, rows, cols, corner1, corner2) &&
					   canConnectStraight(board, rows, cols, corner2, cell2) {
						return true
					}
				}
			}
			// Note: cell1 and corner1 could be the same cell (distance 0) but that's handled by canConnectStraight returning true
		}
	}

	return false
}

// findPossibleMatch finds a possible match on the board
func findPossibleMatch(board [][]int, rows, cols int) (Cell, Cell, bool) {
	// Collect all non-empty cells
	cells := make([]Cell, 0)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if board[r][c] != 0 {
				cells = append(cells, Cell{Row: r, Col: c})
			}
		}
	}

	// Try all pairs
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if canConnect(board, rows, cols, cells[i], cells[j]) {
				return cells[i], cells[j], true
			}
		}
	}

	return Cell{}, Cell{}, false
}

// findBestMatch finds the "best" match based on some heuristic
// For AI use: prefer matches that are easier to see (straight lines, fewer corners)
func findBestMatch(board [][]int, rows, cols int) (Cell, Cell, int) {
	bestScore := -1
	var bestCell1, bestCell2 Cell
	found := false

	// Collect all non-empty cells
	cells := make([]Cell, 0)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if board[r][c] != 0 {
				cells = append(cells, Cell{Row: r, Col: c})
			}
		}
	}

	// Score each possible match
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if board[cells[i].Row][cells[i].Col] != board[cells[j].Row][cells[j].Col] {
				continue
			}

			// Check connection type and score it
			if canConnectStraight(board, rows, cols, cells[i], cells[j]) {
				// Straight line is best
				score := 100
				if score > bestScore {
					bestScore = score
					bestCell1, bestCell2 = cells[i], cells[j]
					found = true
				}
			} else if canConnectOneCorner(board, rows, cols, cells[i], cells[j]) {
				// One corner is good
				score := 50
				if score > bestScore {
					bestScore = score
					bestCell1, bestCell2 = cells[i], cells[j]
					found = true
				}
			} else if canConnectTwoCorners(board, rows, cols, cells[i], cells[j]) {
				// Two corners is acceptable
				score := 10
				if score > bestScore {
					bestScore = score
					bestCell1, bestCell2 = cells[i], cells[j]
					found = true
				}
			}
		}
	}

	if found {
		return bestCell1, bestCell2, bestScore
	}
	return Cell{}, Cell{}, -1
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}