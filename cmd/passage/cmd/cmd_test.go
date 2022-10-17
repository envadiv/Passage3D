package cmd_test

import (
	"fmt"
	"testing"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/cmd/passage/cmd"
	"github.com/stretchr/testify/require"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	"github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

func TestInitCmd(t *testing.T) {
	rootCmd, _ := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",             // Test the init cmd
		"passage-app-test", // Moniker
		fmt.Sprintf("--%s=%s", cli.FlagOverwrite, "true"), // Overwrite genesis.json, in case it already exists
	})

	require.NoError(t, svrcmd.Execute(rootCmd, app.DefaultNodeHome))
}
