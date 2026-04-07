package checkers

// MoveResult is the outcome of a validated move.
type MoveResult struct {
	NewBoard     Board
	Flags        uint32
	Captured     Piece
	ContinueJump bool
}

// ValidateMove checks if a move is legal and returns the result.
// Port of checkmov.cpp: ZCheckersPieceCanMoveTo + ZCheckersIsLegalMoveInternal.
func ValidateMove(board Board, player int, move Move) (MoveResult, bool) {
	piece := board[move.Start.Row][move.Start.Col]
	if piece == None {
		return MoveResult{}, false
	}
	if PieceOwner(piece) != player {
		return MoveResult{}, false
	}

	// Work on a copy
	b := board
	var flags uint32
	var captured Piece

	pt := PieceType(piece)
	switch pt {
	case BlackPawn, WhitePawn:
		// Pawns must move forward
		if !isForward(player, move) {
			return MoveResult{}, false
		}
		ok, cap, f := kingCanMoveTo(&b, player, move)
		if !ok {
			return MoveResult{}, false
		}
		captured = cap
		flags = f

		// Check promotion
		if (player == PlayerBlack && move.Finish.Row == 7) ||
			(player == PlayerWhite && move.Finish.Row == 0) {
			if player == PlayerBlack {
				b[move.Finish.Row][move.Finish.Col] = BlackKing
			} else {
				b[move.Finish.Row][move.Finish.Col] = WhiteKing
			}
			flags |= FlagPromote
		}

	case BlackKing, WhiteKing:
		ok, cap, f := kingCanMoveTo(&b, player, move)
		if !ok {
			return MoveResult{}, false
		}
		captured = cap
		flags = f

	default:
		return MoveResult{}, false
	}

	// Mandatory jump check
	mustJump, _ := PlayerCanJump(board, player)
	if mustJump && (flags&FlagWasJump) == 0 {
		return MoveResult{}, false
	}

	// Check for continue jump (multi-jump)
	continueJump := false
	if (flags & FlagWasJump) != 0 {
		if cj, _ := pieceCanJump(&b, player, move.Finish); cj {
			if (flags & FlagPromote) == 0 {
				flags |= FlagContinueJump
				continueJump = true
			}
		}
	}

	return MoveResult{
		NewBoard:     b,
		Flags:        flags,
		Captured:     captured,
		ContinueJump: continueJump,
	}, true
}

// isForward checks if the move is forward for the given player.
func isForward(player int, move Move) bool {
	if player == PlayerBlack {
		return move.Start.Row < move.Finish.Row
	}
	return move.Start.Row > move.Finish.Row
}

// kingCanMoveTo checks if a piece can move to the target (king rules - any direction).
// Modifies board in-place. Returns ok, captured piece, flags.
func kingCanMoveTo(b *Board, player int, move Move) (bool, Piece, uint32) {
	if b[move.Finish.Row][move.Finish.Col] != None {
		return false, None, 0
	}

	dr := int(move.Finish.Row) - int(move.Start.Row)
	dc := int(move.Finish.Col) - int(move.Start.Col)
	absDr := abs(dr)
	absDc := abs(dc)

	if absDr == 1 && absDc == 1 {
		// Simple diagonal move
		piece := b[move.Start.Row][move.Start.Col]
		b[move.Start.Row][move.Start.Col] = None
		b[move.Finish.Row][move.Finish.Col] = piece
		return true, None, 0
	}

	if absDr == 2 && absDc == 2 {
		// Jump
		midRow := (int(move.Start.Row) + int(move.Finish.Row)) / 2
		midCol := (int(move.Start.Col) + int(move.Finish.Col)) / 2
		midPiece := b[midRow][midCol]

		if midPiece == None || PieceOwner(midPiece) == player {
			return false, None, 0
		}

		piece := b[move.Start.Row][move.Start.Col]
		b[move.Start.Row][move.Start.Col] = None
		b[move.Finish.Row][move.Finish.Col] = piece
		captured := b[midRow][midCol]
		b[midRow][midCol] = None

		return true, captured, FlagWasJump
	}

	return false, None, 0
}

// pieceCanJump checks if a specific piece at sq can jump.
func pieceCanJump(b *Board, player int, sq Square) (bool, Move) {
	piece := b[sq.Row][sq.Col]
	if piece == None || PieceOwner(piece) != player {
		return false, Move{}
	}

	for _, dr := range []int{-2, 2} {
		for _, dc := range []int{-2, 2} {
			fr := int(sq.Row) + dr
			fc := int(sq.Col) + dc
			if fr < 0 || fr >= 8 || fc < 0 || fc >= 8 {
				continue
			}

			move := Move{Start: sq, Finish: Square{Col: byte(fc), Row: byte(fr)}}

			// Pawns can only jump forward
			pt := PieceType(piece)
			if pt == BlackPawn || pt == WhitePawn {
				if !isForward(player, move) {
					continue
				}
			}

			// Check if jump is valid on a copy
			bc := *b
			ok, _, flags := kingCanMoveTo(&bc, player, move)
			if ok && (flags&FlagWasJump) != 0 {
				return true, move
			}
		}
	}
	return false, Move{}
}

// PlayerCanJump checks if any piece of the player can jump.
func PlayerCanJump(board Board, player int) (bool, Move) {
	for row := byte(0); row < 8; row++ {
		for col := byte(0); col < 8; col++ {
			p := board[row][col]
			if p == None || PieceOwner(p) != player {
				continue
			}
			if ok, m := pieceCanJump(&board, player, Square{Col: col, Row: row}); ok {
				return true, m
			}
		}
	}
	return false, Move{}
}

// PlayerCanMove checks if the player has any legal move at all.
func PlayerCanMove(board Board, player int) bool {
	for row := byte(0); row < 8; row++ {
		for col := byte(0); col < 8; col++ {
			p := board[row][col]
			if p == None || PieceOwner(p) != player {
				continue
			}
			sq := Square{Col: col, Row: row}
			// Try all possible destinations
			for dr := -2; dr <= 2; dr++ {
				for dc := -2; dc <= 2; dc++ {
					if dr == 0 || dc == 0 {
						continue
					}
					fr := int(sq.Row) + dr
					fc := int(sq.Col) + dc
					if fr < 0 || fr >= 8 || fc < 0 || fc >= 8 {
						continue
					}
					move := Move{Start: sq, Finish: Square{Col: byte(fc), Row: byte(fr)}}
					if _, ok := ValidateMove(board, player, move); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

// CheckStalemate checks if the next player has no moves (stalemate = they lose).
// Matches ZCheckersCheckCheckmateFlags in checkmov.cpp.
func CheckStalemate(board Board, nextPlayer int) bool {
	return !PlayerCanMove(board, nextPlayer)
}

// IsGameOver checks if the game is over given the current flags.
// Matches ZCheckersIsGameOver in checklib.cpp.
func IsGameOver(flags uint32, lastMovePlayer int) (gameOver bool, score int) {
	if (flags & FlagResign) != 0 {
		if lastMovePlayer == PlayerBlack {
			return true, ScoreWhiteWins
		}
		return true, ScoreBlackWins
	}
	if (flags & FlagStalemate) != 0 {
		// Stalemate means the NEXT player (not lastMovePlayer) can't move, so they lose
		if lastMovePlayer == PlayerBlack {
			return true, ScoreBlackWins
		}
		return true, ScoreWhiteWins
	}
	if (flags & FlagDraw) != 0 {
		return true, ScoreDraw
	}
	return false, 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
