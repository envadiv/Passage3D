package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/envadiv/Passage3D/x/claim/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	claimTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE: client.ValidateCmd,
	}

	claimTxCmd.AddCommand(CmdInitialClaim())

	return claimTxCmd
}

func CmdInitialClaim() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim [claim_action]",
		Short: "Claim amount based on action from airdrop, claim action are (ActionInitialClaim)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			claimAction := args[0]
			if len(claimAction) == 0 {
				return fmt.Errorf("action type is required")
			}

			v, ok := types.Action_value[claimAction]
			if !ok {
				return fmt.Errorf("invalid action type: %s", claimAction)
			}
			if v != types.ActionInitialClaim {
				return fmt.Errorf("invalid action type: %s, %s is not allowed", claimAction, claimAction)
			}

			msg := types.NewMsgClaim(
				clientCtx.GetFromAddress().String(),
				claimAction,
			)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
