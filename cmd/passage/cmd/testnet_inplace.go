package cmd

import (
	"fmt"
	"io"
	"strings"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto"
	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"github.com/envadiv/Passage3D/app"
)

const (
	flagScheduleUpgrade      = "schedule-upgrade"
	flagScheduleUpgradeDelay = "schedule-upgrade-delay"
	flagAccountsToFund       = "accounts-to-fund"
)

// addInPlaceTestnetCommand registers `passage in-place-testnet`, which turns a copy
// of a node's data into a chain controlled by the local priv_validator_key (rehearsal
// of upgrades on real state). It never runs as part of `start`.
func addInPlaceTestnetCommand(rootCmd *cobra.Command, a appCreator) {
	server.AddTestnetCreatorCommand(rootCmd, a.newTestnetApp, addModuleInitFlags)
	for _, c := range rootCmd.Commands() {
		if c.Name() != "in-place-testnet" {
			continue
		}
		c.Flags().String(flagScheduleUpgrade, "", "Schedule this upgrade plan (the next upgrade name); the node halts there for the next binary")
		c.Flags().Int64(flagScheduleUpgradeDelay, 20, "Blocks between the current height and the scheduled upgrade")
		c.Flags().String(flagAccountsToFund, "", "Comma-separated bech32 accounts funded with 10,000,000 PASG each (the operator is always funded)")
	}
}

// newTestnetApp builds the app and rewrites its state for the in-place testnet.
func (a appCreator) newTestnetApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	passageApp, ok := a.newApp(logger, db, traceStore, appOpts).(*app.PassageApp)
	if !ok {
		panic("in-place-testnet: unexpected application type")
	}
	newValAddr, ok := appOpts.Get(server.KeyNewValAddr).(cmtbytes.HexBytes)
	if !ok {
		panic("in-place-testnet: missing new validator address")
	}
	newValPubKey, ok := appOpts.Get(server.KeyUserPubKey).(crypto.PubKey)
	if !ok {
		panic("in-place-testnet: missing new validator public key")
	}
	operator, ok := appOpts.Get(server.KeyNewOpAddr).(string)
	if !ok {
		panic("in-place-testnet: missing operator address")
	}
	var fund []string
	for _, s := range strings.Split(cast.ToString(appOpts.Get(flagAccountsToFund)), ",") {
		if s = strings.TrimSpace(s); s != "" {
			fund = append(fund, s)
		}
	}
	out, err := app.InitPassageAppForTestnet(passageApp, app.TestnetOptions{
		NewValAddr:           newValAddr,
		NewValPubKey:         newValPubKey,
		NewOperator:          operator,
		AccountsToFund:       fund,
		FundAmount:           sdkmath.NewInt(10_000_000_000_000), // 10,000,000 PASG
		ScheduleUpgrade:      cast.ToString(appOpts.Get(flagScheduleUpgrade)),
		ScheduleUpgradeDelay: cast.ToInt64(appOpts.Get(flagScheduleUpgradeDelay)),
	})
	if err != nil {
		panic(fmt.Errorf("in-place-testnet: %w", err))
	}
	return out
}
