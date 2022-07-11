package dex_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	keepertest "github.com/sei-protocol/sei-chain/testutil/keeper"
	"github.com/sei-protocol/sei-chain/x/dex/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const TEST_CONTRACT = "test"
const TEST_ACCOUNT = "accnt"

func TestEndBlockRollback(t *testing.T) {
	testApp := keepertest.TestApp()
	ctx := testApp.BaseApp.NewContext(false, tmproto.Header{})
	dexkeeper := testApp.DexKeeper
	pair := TEST_PAIR()
	// register contract and pair
	dexkeeper.SetContractAddress(ctx, TEST_CONTRACT, 123)
	dexkeeper.AddRegisteredPair(ctx, TEST_CONTRACT, pair)
	// place one order to a nonexistent contract
	dexkeeper.MemState.GetBlockOrders(types.ContractAddress(TEST_CONTRACT), types.GetPairString(&pair)).AddOrder(
		types.Order{
			Id:                1,
			Account:           TEST_ACCOUNT,
			ContractAddr:      TEST_CONTRACT,
			Price:             sdk.MustNewDecFromStr("1"),
			Quantity:          sdk.MustNewDecFromStr("1"),
			PriceDenom:        pair.PriceDenom,
			AssetDenom:        pair.AssetDenom,
			OrderType:         types.OrderType_LIMIT,
			PositionDirection: types.PositionDirection_LONG,
		},
	)
	testApp.EndBlocker(ctx, abci.RequestEndBlock{})
	// No state change should've been persisted
	require.Equal(t, 0, len(dexkeeper.GetOrdersByIds(ctx, TEST_CONTRACT, []uint64{1})))
	_, found := dexkeeper.GetLongBookByPrice(ctx, TEST_CONTRACT, sdk.MustNewDecFromStr("1"), pair.PriceDenom, pair.AssetDenom)
	require.False(t, found)
}
