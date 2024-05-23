package main

import (
	"context"
	"fmt"
	"math/big"

	"google.golang.org/grpc"

	types "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	compression "github.com/skip-mev/slinky/abci/strategies/codec"
	"github.com/skip-mev/slinky/abci/strategies/currencypair"
	oracletypes "github.com/skip-mev/slinky/x/oracle/types"
)

func main() {
	ctx := context.Background()
	conn, err := grpc.DialContext(
		ctx,
		"initia-testnet-grpc.polkachu.com:25790",
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	cmtClient := types.NewServiceClient(conn)
	oracleClient := oracletypes.NewQueryClient(conn)

	cpresp, err := oracleClient.GetAllCurrencyPairs(
		ctx,
		&oracletypes.GetAllCurrencyPairsRequest{},
	)
	cpresp.CurrencyPairs[0].String()

	resp, err := cmtClient.GetLatestBlock(
		ctx,
		&types.GetLatestBlockRequest{},
	)
	if err != nil {
		panic(err)
	}

	txs := resp.GetBlock().Data.Txs

	commitCodec := compression.NewCompressionExtendedCommitCodec(
		compression.NewDefaultExtendedCommitCodec(),
		compression.NewZStdCompressor(),
	)
	commit, err := commitCodec.Decode(txs[0])
	if err != nil {
		panic(err)
	}

	for _, vote := range commit.GetVotes() {
		addr := vote.Validator.Address
		cons := sdktypes.MustBech32ifyAddressBytes("initvalcons", addr)
		// acc := sdkTypes.MustBech32ifyAddressBytes("init", addr)
		fmt.Println(cons, ":", vote.BlockIdFlag, ":", len(vote.VoteExtension))
	}

	veCodec := compression.NewCompressionVoteExtensionCodec(
		compression.NewDefaultVoteExtensionCodec(),
		compression.NewZLibCompressor(),
	)

	b := commit.GetVotes()[4].VoteExtension
	ve, err := veCodec.Decode(b)
	if err != nil {
		panic(err)
	}

	for k, v := range ve.Prices {
		y, err := GetDecodedPrice(v)
		if err != nil {
			panic(err)
		}

		fmt.Println(k, ":", y)
	}

	id, err := currencypair.CurrencyPairToHashID("XRP/USD")
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
}

func GetDecodedPrice(priceBytes []byte) (*big.Int, error) {
	var price big.Int
	if err := price.GobDecode(priceBytes); err != nil {
		return nil, err
	}

	if price.Sign() < 0 {
		return nil, fmt.Errorf("price cannot be negative: %s", price.String())
	}

	return &price, nil
}
