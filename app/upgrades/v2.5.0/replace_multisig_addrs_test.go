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
	s.oldAddr = sdk.AccAddress(s.oldPrivKey.PubKey().Address())
	s.newAddr = sdk.AccAddress(s.newPrivKey.PubKey().Address())

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

	// Setup the app with genesis accounts
	genAccs := []authtypes.GenesisAccount{
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
	s.ctx = s.app.BaseApp.NewContext(false, tmproto.Header{Height: 1})

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

	// Get the old vesting account before migration
	oldAcc := s.app.AccountKeeper.GetAccount(s.ctx, s.oldAddr)
	require.NotNil(s.T(), oldAcc)
	oldVestingAcc, ok := oldAcc.(*vestingtypes.PeriodicVestingAccount)
	require.True(s.T(), ok)
	originalVesting := oldVestingAcc.OriginalVesting
	vestingPeriods := oldVestingAcc.VestingPeriods
	startTime := oldVestingAcc.StartTime

	// Perform delegation
	_, err := s.app.StakingKeeper.Delegate(
		s.ctx,
		s.oldAddr,
		delegationAmt,
		stakingtypes.Unbonded,
		validator,
		true,
	)
	require.NoError(s.T(), err)

	// Setup authorization
	grantee := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address())
	authorization := authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgSend{}))
	expiration := time.Now().Add(time.Hour)
	err = s.app.AuthzKeeper.SaveGrant(s.ctx, grantee, s.oldAddr, authorization, expiration)
	require.NoError(s.T(), err)

	// Use migrations from suite instead of creating new ones
	err = v2_5.MigrateMultisigAddresses(
		s.ctx,
		s.app.AppCodec(),
		s.migrations,
		s.app.BankKeeper,
		s.app.AccountKeeper,
		s.app.StakingKeeper,
		s.app.GovKeeper,
		s.app.AuthzKeeper,
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
	require.Equal(s.T(), s.totalBalance, actualBalance)

	// Check spendable balances
	spendableBalance := s.app.BankKeeper.SpendableCoins(s.ctx, s.newAddr)
	expectedSpendable := s.totalBalance.Sub(s.vestingAmount)
	require.Equal(s.T(), expectedSpendable, spendableBalance)

	// Old address should have no balance
	oldBalance := s.app.BankKeeper.GetAllBalances(s.ctx, s.oldAddr)
	require.True(s.T(), oldBalance.IsZero())
	oldSpendable := s.app.BankKeeper.SpendableCoins(s.ctx, s.oldAddr)
	require.True(s.T(), oldSpendable.IsZero())

	// Verify delegation migration
	oldDelegations := s.app.StakingKeeper.GetAllDelegatorDelegations(s.ctx, s.oldAddr)
	require.Empty(s.T(), oldDelegations)
	newDelegations := s.app.StakingKeeper.GetAllDelegatorDelegations(s.ctx, s.newAddr)
	require.Len(s.T(), newDelegations, 1)
	require.Equal(s.T(), delegationAmt, newDelegations[0].Shares.TruncateInt())

	// Verify authorization migration
	oldGrants := s.app.AuthzKeeper.GetAuthorizations(s.ctx, grantee, s.oldAddr)
	require.Empty(s.T(), oldGrants)
	newGrants := s.app.AuthzKeeper.GetAuthorizations(s.ctx, grantee, s.newAddr)
	require.Len(s.T(), newGrants, 1)
	require.Equal(s.T(), authorization.MsgTypeURL(), newGrants[0].MsgTypeURL())
}
