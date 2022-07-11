package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/sei-protocol/sei-chain/x/dex/types"
	"go.opentelemetry.io/otel/attribute"
	otrace "go.opentelemetry.io/otel/trace"
)

func (k *Keeper) HandleEBCancelOrders(ctx context.Context, sdkCtx sdk.Context, tracer *otrace.Tracer, contractAddr string, registeredPairs []types.Pair) error {
	_, span := (*tracer).Start(ctx, "SudoCancelOrders")
	span.SetAttributes(attribute.String("contractAddr", contractAddr))

	typedContractAddr := types.ContractAddress(contractAddr)
	msg := k.getCancelSudoMsg(typedContractAddr, registeredPairs)
	if _, err := k.CallContractSudo(sdkCtx, contractAddr, msg); err != nil {
		return err
	}
	span.End()
	return nil
}

func (k *Keeper) getCancelSudoMsg(typedContractAddr types.ContractAddress, registeredPairs []types.Pair) types.SudoOrderCancellationMsg {
	idsToCancel := []uint64{}
	for _, pair := range registeredPairs {
		typedPairStr := types.GetPairString(&pair)
		for _, cancel := range *k.MemState.GetBlockCancels(typedContractAddr, typedPairStr) {
			idsToCancel = append(idsToCancel, cancel.Id)
		}
	}
	return types.SudoOrderCancellationMsg{
		OrderCancellations: types.OrderCancellationMsgDetails{
			IdsToCancel: idsToCancel,
		},
	}
}
