package main

import (
	"fmt"
	"os"

	"agent-board/internal/board"
)

func main() {
	if err := board.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
