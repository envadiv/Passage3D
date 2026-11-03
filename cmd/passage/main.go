package main

import (
	"fmt"
	"os"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/cmd/passage/cmd"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		// The root command silences cobra's own error output, so print it here.
		fmt.Fprintln(rootCmd.ErrOrStderr(), "Error:", err)
		os.Exit(1)
	}
}
