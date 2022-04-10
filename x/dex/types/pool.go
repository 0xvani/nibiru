package types

import (
	"errors"
	fmt "fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

/*
Returns the *base* denomination of a pool share token for a given poolId.

args:
  poolId: the pool id number

ret:
  poolDenom: the pool denomination name of the poolId
*/
func GetPoolShareBaseDenom(poolId uint64) (poolDenom string) {
	return fmt.Sprintf("matrix/pool/%d", poolId)
}

/*
Returns the *display* denomination of a pool share token for a given poolId.
Display denom means the denomination showed to the user, which could be many exponents greater than the base denom.
e.g. 1 atom is the display denom, but 10^6 uatom is the base denom.

In Matrix, a display denom is 10^18 base denoms.

args:
  poolId: the pool id number

ret:
  poolDenom: the pool denomination name of the poolId
*/
func GetPoolShareDisplayDenom(poolId uint64) (poolDenom string) {
	return fmt.Sprintf("MATRIX-POOL-%d", poolId)
}

// setInitialPoolAssets sets the PoolAssets in the pool.
// It is only designed to be called at the pool's creation.
// If the same denom's PoolAsset exists, will return error.
// The list of PoolAssets must be sorted. This is done to enable fast searching for a PoolAsset by denomination.
func (p *Pool) setInitialPoolAssets(poolAssets []PoolAsset) (err error) {
	exists := make(map[string]bool)

	newTotalWeight := sdk.ZeroInt()
	scaledPoolAssets := make([]PoolAsset, 0, len(poolAssets))

	for _, asset := range poolAssets {
		if err = asset.Validate(); err != nil {
			return err
		}

		if exists[asset.Token.Denom] {
			return fmt.Errorf("same PoolAsset already exists")
		}
		exists[asset.Token.Denom] = true

		// Scale weight from the user provided weight to the correct internal weight
		asset.Weight = asset.Weight.MulRaw(GuaranteedWeightPrecision)
		scaledPoolAssets = append(scaledPoolAssets, asset)
		newTotalWeight = newTotalWeight.Add(asset.Weight)
	}

	p.PoolAssets = scaledPoolAssets
	sortPoolAssetsByDenom(p.PoolAssets)

	p.TotalWeight = newTotalWeight

	return nil
}

/*
Creates a new pool and sets the initial assets.

args:
  poolId: the pool numeric id
  poolAccountAddr: the pool's account address for holding funds
  poolParams: pool configuration options
  poolAssets: the initial pool assets and weights

ret:
  pool: a new pool
  err: error if any
*/
func NewPool(
	poolId uint64,
	poolAccountAddr sdk.Address,
	poolParams PoolParams,
	poolAssets []PoolAsset,
) (pool Pool, err error) {
	pool = Pool{
		Id:          poolId,
		Address:     poolAccountAddr.String(),
		PoolParams:  poolParams,
		PoolAssets:  nil,
		TotalWeight: sdk.ZeroInt(),
		TotalShares: sdk.NewCoin(GetPoolShareBaseDenom(poolId), InitPoolSharesSupply),
	}

	err = pool.setInitialPoolAssets(poolAssets)
	if err != nil {
		return Pool{}, err
	}

	return pool, nil
}

/*
Adds tokens to a pool and updates the pool balances (i.e. liquidity).

args:
  - tokensIn: the tokens to add to the pool

ret:
  - numShares: the number of LP shares given to the user for the deposit
  - remCoins: the number of coins remaining after the deposit
  - err: error if any
*/
func (pool *Pool) JoinPool(tokensIn sdk.Coins) (numShares sdk.Int, remCoins sdk.Coins, err error) {
	if tokensIn.Len() != len(pool.PoolAssets) {
		return sdk.ZeroInt(), sdk.NewCoins(), errors.New("wrong number of assets to deposit into the pool")
	}

	// Add all exact coins we can (no swap)
	numShares, remCoins, err = pool.maximalSharesFromExactRatioJoin(tokensIn)
	if err != nil {
		return sdk.ZeroInt(), sdk.NewCoins(), err
	}

	if err := pool.updateLiquidity(numShares, tokensIn.Sub(remCoins)); err != nil {
		return sdk.ZeroInt(), sdk.NewCoins(), err
	}

	return numShares, remCoins, nil
}