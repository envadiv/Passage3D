package main

import (
	"os"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/cmd/passage/cmd"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd, _ := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		os.Exit(1)
	}
}
