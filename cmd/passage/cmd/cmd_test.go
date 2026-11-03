package cmd_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client/flags"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/envadiv/Passage3D/app"
	"github.com/envadiv/Passage3D/cmd/passage/cmd"
)

func TestInitCmd(t *testing.T) {
	home := t.TempDir()
	rootCmd, encCfg := cmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"init",             // Test the init cmd
		"passage-app-test", // Moniker
		fmt.Sprintf("--%s=%s", genutilcli.FlagOverwrite, "true"), // Overwrite genesis.json, in case it already exists
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
	})

	require.NoError(t, svrcmd.Execute(rootCmd, "", home))

	appState, _, err := genutiltypes.GenesisStateFromGenFile(filepath.Join(home, "config", "genesis.json"))
	require.NoError(t, err)
	stakingGenState := stakingtypes.GetGenesisStateFromAppState(encCfg.Marshaler, appState)
	require.Equal(t, app.BondDenom, stakingGenState.Params.BondDenom)
}
