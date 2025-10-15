package main

import (
	"fmt"
	"os"

	"github.com/giwty/switch-library-manager/cmd"
	slmErrors "github.com/giwty/switch-library-manager/pkg/errors"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Format and display user-friendly error
		fmt.Fprint(os.Stderr, slmErrors.FormatError(err))
		os.Exit(1)
	}
}