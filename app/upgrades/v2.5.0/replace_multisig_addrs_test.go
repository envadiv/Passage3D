package v2_5_test

import (
	"testing"
	"time"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/cosmos-sdk/x/feegrant"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	app "github.com/envadiv/Passage3D/app"
	v2_5 "github.com/envadiv/Passage3D/app/upgrades/v2.5.0"
)

type UpgradeTestSuite struct {
	suite.Suite

	app           *app.PassageApp
	ctx           sdk.Context
	oldAddr       sdk.AccAddress
	newAddr       sdk.AccAddress
	oldPrivKey    *secp256k1.PrivKey
	newPrivKey    *secp256k1.PrivKey
	migrations    []v2_5.AddressMigration
	vestingAmount sdk.Coins // Amount that is vesting
	totalBalance  sdk.Coins // Total balance including vesting and non-vesting
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) SetupTest() {
	// Generate test addresses
	s.oldPrivKey = secp256k1.GenPrivKey()
	s.newPrivKey = secp256k1.GenPrivKey()
	accKey := secp256k1.GenPrivKey()
	s.oldAddr = sdk.AccAddress(s.oldPrivKey.PubKey().Address())
	s.newAddr = sdk.AccAddress(s.newPrivKey.PubKey().Address())
	accAddr := sdk.AccAddress(accKey.PubKey().Address())

	s.vestingAmount = sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(1000)))
	s.totalBalance = sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(1500)))

	// Create old vesting account for genesis
	vestingStart := time.Now().Unix()
	oldBaseAcc := authtypes.NewBaseAccount(s.oldAddr, s.oldPrivKey.PubKey(), 1, 0)
	oldVestingAcc := vestingtypes.NewPeriodicVestingAccount(
		oldBaseAcc,
		s.vestingAmount,
		vestingStart,
		[]vestingtypes.Period{
			{Length: 50000, Amount: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(500)))},
			{Length: 50000, Amount: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(500)))},
		},
	)

	accAcc := authtypes.NewBaseAccount(accAddr, accKey.PubKey(), 2, 0)
	// Setup the app with genesis accounts
	genAccs := []authtypes.GenesisAccount{
		accAcc,
		oldVestingAcc,
	}
	balances := []banktypes.Balance{
		{
			Address: s.oldAddr.String(),
			Coins:   s.totalBalance,
		},
	}

	// Create validator
	valPrivKey := secp256k1.GenPrivKey()
	valPubKey := valPrivKey.PubKey()
	tmPubKey, err := cryptocodec.ToTmPubKeyInterface(valPubKey)
	require.NoError(s.T(), err)

	validator := tmtypes.NewValidator(tmPubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})

	// Setup the app with genesis state
	s.app = app.SetupWithGenesisValSet(s.T(), valSet, genAccs, balances...)
	s.ctx = s.app.BaseApp.NewContext(false, tmproto.Header{Height: 1, Time: time.Now().Add(time.Second * 120)})

	s.app.Commit()

	// Create migration data
	s.migrations = []v2_5.AddressMigration{
		{
			OldAddress: s.oldAddr.String(),
			NewAddress: s.newAddr.String(),
		},
	}
}

func (s *UpgradeTestSuite) TestMigrateMultisigAddresses() {
	// Setup delegation
	delegationAmt := sdk.NewInt(500)
	validator := s.app.StakingKeeper.GetValidators(s.ctx, 1)[0]

	// Create another validator for redelegation
	valPrivKey := secp256k1.GenPrivKey()
	valPubKey := valPrivKey.PubKey()
	validator2, err := stakingtypes.NewValidator(sdk.ValAddress(valPubKey.Address()), valPubKey, stakingtypes.Description{})
	require.NoError(s.T(), err)
	validator2.Status = stakingtypes.Bonded
	validator2.Tokens = sdk.NewInt(1000000)
	s.app.StakingKeeper.SetValidator(s.ctx, validator2)
	s.app.StakingKeeper.AfterValidatorCreated(s.ctx, validator2.GetOperator())

	// Get the old vesting account before migration
	oldAcc := s.app.AccountKeeper.GetAccount(s.ctx, s.oldAddr)
	require.NotNil(s.T(), oldAcc)
	oldVestingAcc, ok := oldAcc.(*vestingtypes.PeriodicVestingAccount)
	require.True(s.T(), ok)
	originalVesting := oldVestingAcc.OriginalVesting
	vestingPeriods := oldVestingAcc.VestingPeriods
	startTime := oldVestingAcc.StartTime

	// Perform delegation
	_, err = s.app.StakingKeeper.Delegate(
		s.ctx,
		s.oldAddr,
		delegationAmt,
		stakingtypes.Unbonded,
		validator,
		true,
	)
	require.NoError(s.T(), err)

	// Create an unbonding delegation
	unbondAmt := sdk.NewInt(200)
	shares, err := s.app.StakingKeeper.ValidateUnbondAmount(s.ctx, s.oldAddr, validator.GetOperator(), unbondAmt)
	require.NoError(s.T(), err)
	_, err = s.app.StakingKeeper.Undelegate(s.ctx, s.oldAddr, validator.GetOperator(), shares)
	require.NoError(s.T(), err)

	// Create a redelegation
	redelegateAmt := sdk.NewInt(50)
	shares, err = s.app.StakingKeeper.ValidateUnbondAmount(s.ctx, s.oldAddr, validator.GetOperator(), redelegateAmt)
	require.NoError(s.T(), err)
	_, err = s.app.StakingKeeper.BeginRedelegation(s.ctx, s.oldAddr, validator.GetOperator(), validator2.GetOperator(), shares)
	require.NoError(s.T(), err)

	// Setup authorization
	grantee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	authorization := authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgSend{}))
	expiration := time.Now().Add(time.Hour)
	err = s.app.AuthzKeeper.SaveGrant(s.ctx, grantee, s.oldAddr, authorization, expiration)
	require.NoError(s.T(), err)

	// Setup feegrant
	basicAllowance := &feegrant.BasicAllowance{
		SpendLimit: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(100))),
		Expiration: &expiration,
	}
	err = s.app.FeeGrantKeeper.GrantAllowance(s.ctx, s.oldAddr, grantee, basicAllowance)
	require.NoError(s.T(), err)

	// Setup gov votes to test it
	var proposalID uint64 = 1
	weightedVote := []govtypes.WeightedVoteOption{
		{Option: govtypes.OptionYes},
	}
	vote := govtypes.NewVote(proposalID, s.oldAddr, weightedVote)
	s.app.GovKeeper.SetVote(s.ctx, vote)

	// fetch balances and state before migration
	oldSpendableBalance := s.app.BankKeeper.SpendableCoins(s.ctx, s.oldAddr)
	oldTotalBalance := s.app.BankKeeper.GetAllBalances(s.ctx, s.oldAddr)

	oldDelegations := s.app.StakingKeeper.GetAllDelegatorDelegations(s.ctx, s.oldAddr)
	require.Len(s.T(), oldDelegations, 2)

	oldUnbonding := s.app.StakingKeeper.GetAllUnbondingDelegations(s.ctx, s.oldAddr)
	require.Len(s.T(), oldUnbonding, 1)

	oldRedelegations := s.app.StakingKeeper.GetAllRedelegations(s.ctx, s.oldAddr, validator.GetOperator(), validator2.GetOperator())
	require.Len(s.T(), oldRedelegations, 1)

	// get staking module accounts balances
	oldBondedPoolBal := s.app.BankKeeper.GetAllBalances(s.ctx, s.app.StakingKeeper.GetBondedPool(s.ctx).GetAddress())
	oldNonBondedPoolBal := s.app.BankKeeper.GetAllBalances(s.ctx, s.app.StakingKeeper.GetNotBondedPool(s.ctx).GetAddress())

	// Perform migration
	err = v2_5.MigrateMultisigAddresses(
		s.ctx,
		s.app.AppCodec(),
		s.migrations,
		s.app.BankKeeper,
		s.app.AccountKeeper,
		s.app.StakingKeeper,
		s.app.GovKeeper,
		s.app.AuthzKeeper,
		s.app.FeeGrantKeeper,
		s.app.ClaimKeeper,
	)
	require.NoError(s.T(), err)

	// Verify account migration
	newAcc := s.app.AccountKeeper.GetAccount(s.ctx, s.newAddr)
	require.NotNil(s.T(), newAcc)
	newVestingAcc, ok := newAcc.(*vestingtypes.PeriodicVestingAccount)
	require.True(s.T(), ok)

	// Verify vesting details are correctly migrated
	require.Equal(s.T(), originalVesting, newVestingAcc.OriginalVesting)
	require.Equal(s.T(), vestingPeriods, newVestingAcc.VestingPeriods)
	require.Equal(s.T(), startTime, newVestingAcc.StartTime)

	// Verify old account vesting periods are cleared
	oldAccAfterMigration := s.app.AccountKeeper.GetAccount(s.ctx, s.oldAddr)
	require.NotNil(s.T(), oldAccAfterMigration)
	oldVestingAccAfterMigration, ok := oldAccAfterMigration.(*vestingtypes.PeriodicVestingAccount)
	require.True(s.T(), ok)
	for _, period := range oldVestingAccAfterMigration.VestingPeriods {
		require.Equal(s.T(), int64(0), period.Length)
	}

	// Verify balances
	// Check total balances
	actualBalance := s.app.BankKeeper.GetAllBalances(s.ctx, s.newAddr)
	require.Equal(s.T(), oldTotalBalance, actualBalance)

	// Check spendable balances
	spendableBalance := s.app.BankKeeper.SpendableCoins(s.ctx, s.newAddr)
	require.Equal(s.T(), oldSpendableBalance, spendableBalance)

	// Old address should have no balance
	oldBalance := s.app.BankKeeper.GetAllBalances(s.ctx, s.oldAddr)
	require.True(s.T(), oldBalance.IsZero())
	oldSpendable := s.app.BankKeeper.SpendableCoins(s.ctx, s.oldAddr)
	require.True(s.T(), oldSpendable.IsZero())

	// Verify delegation migration
	newDelegations := s.app.StakingKeeper.GetAllDelegatorDelegations(s.ctx, s.newAddr)
	require.Len(s.T(), newDelegations, 2)
	require.Equal(s.T(), oldDelegations[0].ValidatorAddress, newDelegations[0].ValidatorAddress)
	require.Equal(s.T(), oldDelegations[0].Shares, newDelegations[0].Shares)
	oldDelegations = s.app.StakingKeeper.GetAllDelegatorDelegations(s.ctx, s.oldAddr)
	require.Empty(s.T(), oldDelegations)

	// Verify authorization migration
	oldGrants := s.app.AuthzKeeper.GetAuthorizations(s.ctx, grantee, s.oldAddr)
	require.Empty(s.T(), oldGrants)
	newGrants := s.app.AuthzKeeper.GetAuthorizations(s.ctx, grantee, s.newAddr)
	require.Len(s.T(), newGrants, 1)
	require.Equal(s.T(), authorization.MsgTypeURL(), newGrants[0].MsgTypeURL())

	// Verify feegrant migration
	newFeeGrants, err := s.app.FeeGrantKeeper.GetAllowance(s.ctx, s.newAddr, grantee)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), newFeeGrants)
	require.Equal(s.T(), basicAllowance.SpendLimit, newFeeGrants.(*feegrant.BasicAllowance).SpendLimit)

	// Verify gov votes migration
	votes := s.app.GovKeeper.GetAllVotes(s.ctx)
	require.Len(s.T(), votes, 2) // includes old vote too
	newVote := vote
	newVote.Voter = s.newAddr.String()
	require.Contains(s.T(), votes, newVote)

	// validate staking module accounts balances
	newBondedPoolBal := s.app.BankKeeper.GetAllBalances(s.ctx, s.app.StakingKeeper.GetBondedPool(s.ctx).GetAddress())
	newNonBondedPoolBal := s.app.BankKeeper.GetAllBalances(s.ctx, s.app.StakingKeeper.GetNotBondedPool(s.ctx).GetAddress())
	require.Equal(s.T(), oldBondedPoolBal, newBondedPoolBal)
	require.Equal(s.T(), oldNonBondedPoolBal, newNonBondedPoolBal)

	// Verify unbonding delegation migration
	newUnbonding := s.app.StakingKeeper.GetAllUnbondingDelegations(s.ctx, s.newAddr)
	require.Len(s.T(), newUnbonding, 1)
	require.Equal(s.T(), oldUnbonding[0].ValidatorAddress, newUnbonding[0].ValidatorAddress)
	require.Equal(s.T(), oldUnbonding[0].Entries, newUnbonding[0].Entries)

	oldUnbonding = s.app.StakingKeeper.GetAllUnbondingDelegations(s.ctx, s.oldAddr)
	require.Empty(s.T(), oldUnbonding)

	// Verify redelegation migration
	newRedelegations := s.app.StakingKeeper.GetAllRedelegations(s.ctx, s.newAddr, validator.GetOperator(), validator2.GetOperator())
	require.Len(s.T(), newRedelegations, 1)
	require.Equal(s.T(), oldRedelegations[0].ValidatorSrcAddress, newRedelegations[0].ValidatorSrcAddress)
	require.Equal(s.T(), oldRedelegations[0].ValidatorDstAddress, newRedelegations[0].ValidatorDstAddress)
	require.Equal(s.T(), oldRedelegations[0].Entries, newRedelegations[0].Entries)

	oldRedelegations = s.app.StakingKeeper.GetAllRedelegations(s.ctx, s.oldAddr, validator.GetOperator(), validator2.GetOperator())
	require.Empty(s.T(), oldRedelegations)
}
