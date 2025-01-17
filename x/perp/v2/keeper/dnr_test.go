package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/app"

	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	. "github.com/NibiruChain/nibiru/x/common/testutil/action"
	. "github.com/NibiruChain/nibiru/x/perp/v2/integration/action"
	"github.com/NibiruChain/nibiru/x/perp/v2/types"
)

func TestUserVolumes(t *testing.T) {
	alice := testutil.AccAddress()
	pairBtcNusd := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	positionSize := sdk.NewInt(10_000)
	startBlockTime := time.Now()

	tests := TestCases{
		TC("user volume correctly sets the first time and the second time").
			Given(
				DnREpochIs(1),
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				MarketOrder(alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
				MarketOrder(alice, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
			).
			Then(
				DnRCurrentVolumeIs(alice, positionSize.MulRaw(2)),
			),
		TC("user volume correctly sets across epochs").
			Given(
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				DnREpochIs(1),
				MarketOrder(alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()), // open epoch 1
				DnREpochIs(2),
				MarketOrder(alice, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()), // close epoch 2
			).
			Then(
				DnRCurrentVolumeIs(alice, positionSize),
				DnRPreviousVolumeIs(alice, positionSize),
			),
		TC("user volume is correctly cleaned up").
			Given(
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).When(
			DnREpochIs(1),
			MarketOrder(alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()), // open epoch 1
			DnREpochIs(2),
			MarketOrder(alice, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(5_000), sdk.OneDec(), sdk.ZeroDec()), // reduce epoch 2
			DnREpochIs(3),
			MarketOrder(alice, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(2_000), sdk.OneDec(), sdk.ZeroDec()), // reduce epoch 3
			MarketOrder(alice, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(2_000), sdk.OneDec(), sdk.ZeroDec()), // reduce more epoch 3
		).
			Then(
				DnRCurrentVolumeIs(alice, math.NewInt(4000)),  // for current epoch only 4k in volume.
				DnRPreviousVolumeIs(alice, math.NewInt(5000)), // for previous epoch only 5k in volume.
			),
	}
	NewTestSuite(t).WithTestCases(tests...).Run()
}

func TestDiscount(t *testing.T) {
	alice := testutil.AccAddress()
	pairBtcNusd := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	positionSize := sdk.NewInt(10_000)
	startBlockTime := time.Now()

	exchangeFee := sdk.MustNewDecFromStr("0.0010")           // 0.1%, default fee taken from CreateCustomMarketAction
	globalFeeDiscount := sdk.MustNewDecFromStr("0.0005")     // 0.05%
	fauxGlobalFeeDiscount := sdk.MustNewDecFromStr("0.0006") // 0.06%
	customFeeDiscount := sdk.MustNewDecFromStr("0.0002")     // 0.02%
	fauxCustomFeeDiscount := sdk.MustNewDecFromStr("0.0003") // 0.03%

	tests := TestCases{
		TC("user does not have any past epoch volume: no discount applies").
			Given(
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				DnREpochIs(1),
			).
			Then(
				MarketOrderFeeIs(exchangeFee, alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
			),
		TC("user has past epoch volume: no discount applies").
			Given(
				DnREpochIs(2),
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				SetGlobalDiscount(fauxGlobalFeeDiscount, sdk.NewInt(50_000)),
				SetGlobalDiscount(globalFeeDiscount, sdk.NewInt(100_000)),
				SetCustomDiscount(alice, fauxCustomFeeDiscount, sdk.NewInt(50_000)),
				SetCustomDiscount(alice, customFeeDiscount, sdk.NewInt(100_000)),
				SetPreviousEpochUserVolume(alice, sdk.NewInt(10_000)), // lower than 50_000
			).
			Then(
				MarketOrderFeeIs(exchangeFee, alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
			),
		TC("user has past epoch volume: custom discount applies").
			Given(
				DnREpochIs(2),
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				SetGlobalDiscount(globalFeeDiscount, sdk.NewInt(50_000)),
				SetGlobalDiscount(fauxGlobalFeeDiscount, sdk.NewInt(100_000)),
				SetCustomDiscount(alice, fauxCustomFeeDiscount, sdk.NewInt(50_000)),
				SetCustomDiscount(alice, customFeeDiscount, sdk.NewInt(100_000)),
				SetPreviousEpochUserVolume(alice, sdk.NewInt(100_001)),
			).
			Then(
				MarketOrderFeeIs(customFeeDiscount, alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
			),
		TC("user has past epoch volume: global discount applies").
			Given(
				DnREpochIs(2),
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(1000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				SetGlobalDiscount(sdk.MustNewDecFromStr("0.0004"), sdk.NewInt(50_000)),
				SetGlobalDiscount(globalFeeDiscount, sdk.NewInt(100_000)),
				SetPreviousEpochUserVolume(alice, sdk.NewInt(100_000)),
			).
			Then(
				MarketOrderFeeIs(globalFeeDiscount, alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
			),
	}
	NewTestSuite(t).WithTestCases(tests...).Run()
}

func TestRebates(t *testing.T) {
	alice := testutil.AccAddress()
	bob := testutil.AccAddress()

	pairBtcNusd := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	positionSize := sdk.NewInt(10_000)
	startBlockTime := time.Now()

	allocation := sdk.NewCoins(sdk.NewCoin(denoms.NUSD, sdk.NewInt(1_000_000)))

	tests := TestCases{
		TC("rebates correctly apply").
			Given(
				CreateCustomMarket(
					pairBtcNusd,
					WithEnabled(true),
					WithPricePeg(sdk.OneDec()),
					WithSqrtDepth(sdk.NewDec(100_000)),
				),
				SetBlockNumber(1),
				SetBlockTime(startBlockTime),

				FundAccount(alice, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(100_000)))),
				FundAccount(bob, sdk.NewCoins(sdk.NewCoin(types.TestingCollateralDenomNUSD, positionSize.AddRaw(100_000)))),
				FundModule(types.PerpEFModuleAccount, sdk.NewCoins(sdk.NewCoin(denoms.NUSD, sdk.NewInt(100_000_000)))),
			).
			When(
				DnREpochIs(1),
				FundDnREpoch(allocation),
				MarketOrder(alice, pairBtcNusd, types.Direction_LONG, sdk.NewInt(10_000), sdk.OneDec(), sdk.ZeroDec()),
				MarketOrder(bob, pairBtcNusd, types.Direction_SHORT, sdk.NewInt(30_000), sdk.OneDec(), sdk.ZeroDec()),
				StartNewDnREpoch(),
			).
			Then(
				DnRRebateIs(alice, 1, allocation.QuoInt(sdk.NewInt(4))),                     // 1/4 of the allocation
				DnRRebateIs(bob, 1, allocation.QuoInt(sdk.NewInt(4)).MulInt(sdk.NewInt(3))), // 3/4 of the allocation
				DnRRebateIs(alice, 1, sdk.NewCoins()),                                       // can only claim once
				DnREpochIs(2),
				DnRRebateIs(alice, 1, sdk.NewCoins()), // claiming again after the epoch is not possible.
			),
	}
	NewTestSuite(t).WithTestCases(tests...).Run()
}

type actionFn func(app *app.NibiruApp, ctx sdk.Context) (outCtx sdk.Context, err error)

func (f actionFn) Do(app *app.NibiruApp, ctx sdk.Context) (outCtx sdk.Context, err error) {
	return f(app, ctx)
}

func TestDnREpoch(t *testing.T) {
	dnrEpochIdentifierIs := func(identifier string) actionFn {
		return func(app *app.NibiruApp, ctx sdk.Context) (outCtx sdk.Context, err error) {
			app.PerpKeeperV2.DnREpochName.Set(ctx, identifier)
			return ctx, nil
		}
	}

	triggerEpoch := func(identifier string, epoch uint64) actionFn {
		return func(app *app.NibiruApp, ctx sdk.Context) (outCtx sdk.Context, err error) {
			app.PerpKeeperV2.AfterEpochEnd(ctx, identifier, epoch)
			return ctx, nil
		}
	}

	expectDnREpoch := func(epoch uint64) actionFn {
		return func(app *app.NibiruApp, ctx sdk.Context) (outCtx sdk.Context, err error) {
			require.Equal(t, epoch, app.PerpKeeperV2.DnREpoch.GetOr(ctx, 0))
			return ctx, nil
		}
	}

	tests := TestCases{
		TC("DnR epoch with valid identifier").
			When(
				dnrEpochIdentifierIs("monthly"),
				triggerEpoch("monthly", 1)).
			Then(
				expectDnREpoch(1),
			),
		TC("DnR epoch with invalid identifier").
			When(
				dnrEpochIdentifierIs("monthly"),
				triggerEpoch("weekly", 1)).
			Then(
				expectDnREpoch(0),
			),
	}
	NewTestSuite(t).WithTestCases(tests...).Run()
}
