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
	// grpc
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

	// pair mapping
	id2cp := make(map[uint64]string)

	cpresp, err := oracleClient.GetAllCurrencyPairs(
		ctx,
		&oracletypes.GetAllCurrencyPairsRequest{},
	)
	for _, cp := range cpresp.CurrencyPairs {
		id, err := currencypair.CurrencyPairToHashID(cp.String())
		if err != nil {
			panic(err)
		}
		id2cp[id] = cp.String()
	}

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
	veCodec := compression.NewCompressionVoteExtensionCodec(
		compression.NewDefaultVoteExtensionCodec(),
		compression.NewZLibCompressor(),
	)

	type Info struct {
		blockIdFlag string
		veBytesLen  int
		prices      map[string]*big.Int
	}
	result := make(map[string]*Info)

	commit, err := commitCodec.Decode(txs[0])
	if err != nil {
		panic(err)
	}

	for _, vote := range commit.GetVotes() {
		addr := vote.Validator.Address
		cons := sdktypes.MustBech32ifyAddressBytes("initvalcons", addr)
		// acc := sdkTypes.MustBech32ifyAddressBytes("init", addr)

		info := &Info{
			blockIdFlag: vote.BlockIdFlag.String(),
			veBytesLen:  len(vote.VoteExtension),
			prices:      make(map[string]*big.Int),
		}

		ve, err := veCodec.Decode(vote.VoteExtension)
		if err != nil {
			panic(err)
		}

		for k, priceBytes := range ve.Prices {
			price, err := GetDecodedPrice(priceBytes)
			if err != nil {
				panic(err)
			}
			info.prices[id2cp[k]] = price
		}

		result[cons] = info
	}

	for cons, info := range result {
		fmt.Println("========================================")
		fmt.Println(cons)
		fmt.Println(info.blockIdFlag, ":", info.veBytesLen)
		for cp, price := range info.prices {
			fmt.Println(cp, price)
		}
		fmt.Println("========================================")
	}
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
