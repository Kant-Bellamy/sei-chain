package dex_test

import (
	"io/ioutil"
	"testing"
	"time"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	keepertest "github.com/sei-protocol/sei-chain/testutil/keeper"
	"github.com/sei-protocol/sei-chain/x/dex/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const TEST_CONTRACT = "test"
const TEST_ACCOUNT = "accnt"
const GOOD_CONTRACT_INSTANTIATE = `{"whitelist": ["sei1h9yjz89tl0dl6zu65dpxcqnxfhq60wxx8s5kag"],
    "use_whitelist":false,"admin":"sei1h9yjz89tl0dl6zu65dpxcqnxfhq60wxx8s5kag",
	"limit_order_fee":{"decimal":"0.0001","negative":false},
	"market_order_fee":{"decimal":"0.0001","negative":false},
	"liquidation_order_fee":{"decimal":"0.0001","negative":false},
	"margin_ratio":{"decimal":"0.0625","negative":false},
	"max_leverage":{"decimal":"4","negative":false}}`

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

func TestEndBlockPartialRollback(t *testing.T) {
	testApp := keepertest.TestApp()
	ctx := testApp.BaseApp.NewContext(false, tmproto.Header{Time: time.Now()})
	// BAD CONTRACT
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
	// GOOD CONTRACT
	testAccount, _ := sdk.AccAddressFromBech32("sei1yezq49upxhunjjhudql2fnj5dgvcwjj87pn2wx")
	amounts := sdk.NewCoins(sdk.NewCoin("usei", sdk.NewInt(1000000)))
	bankkeeper := testApp.BankKeeper
	bankkeeper.MintCoins(ctx, minttypes.ModuleName, amounts)
	bankkeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, testAccount, amounts)
	wasm, err := ioutil.ReadFile("./testdata/clearing_house.wasm")
	if err != nil {
		panic(err)
	}
	wasmKeeper := testApp.WasmKeeper
	contractKeeper := wasmkeeper.NewDefaultPermissionKeeper(&wasmKeeper)
	var perm *wasmtypes.AccessConfig
	codeId, err := contractKeeper.Create(ctx, testAccount, wasm, perm)
	if err != nil {
		panic(err)
	}
	contractAddr, _, err := contractKeeper.Instantiate(ctx, codeId, testAccount, testAccount, []byte(GOOD_CONTRACT_INSTANTIATE), "test",
		sdk.NewCoins(sdk.NewCoin("usei", sdk.NewInt(100000))))
	if err != nil {
		panic(err)
	}
	dexkeeper.SetContractAddress(ctx, contractAddr.String(), 123)
	dexkeeper.AddRegisteredPair(ctx, contractAddr.String(), pair)
	// place one order to a nonexistent contract
	dexkeeper.MemState.GetBlockOrders(types.ContractAddress(contractAddr.String()), types.GetPairString(&pair)).AddOrder(
		types.Order{
			Id:                2,
			Account:           testAccount.String(),
			ContractAddr:      contractAddr.String(),
			Price:             sdk.MustNewDecFromStr("1"),
			Quantity:          sdk.MustNewDecFromStr("1"),
			PriceDenom:        pair.PriceDenom,
			AssetDenom:        pair.AssetDenom,
			OrderType:         types.OrderType_LIMIT,
			PositionDirection: types.PositionDirection_LONG,
		},
	)

	testApp.EndBlocker(ctx, abci.RequestEndBlock{})
	// No state change should've been persisted for bad contract
	require.Equal(t, 0, len(dexkeeper.GetOrdersByIds(ctx, TEST_CONTRACT, []uint64{1})))
	_, found := dexkeeper.GetLongBookByPrice(ctx, TEST_CONTRACT, sdk.MustNewDecFromStr("1"), pair.PriceDenom, pair.AssetDenom)
	require.False(t, found)
	// state change should've been persisted for good contract
	require.Equal(t, 1, len(dexkeeper.GetOrdersByIds(ctx, contractAddr.String(), []uint64{2})))
	_, found = dexkeeper.GetLongBookByPrice(ctx, contractAddr.String(), sdk.MustNewDecFromStr("1"), pair.PriceDenom, pair.AssetDenom)
	require.True(t, found)
}
