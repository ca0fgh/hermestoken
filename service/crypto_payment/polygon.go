package crypto_payment

import (
	"context"
	"fmt"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const polygonChainID = 137

type PolygonScanner struct {
	config setting.CryptoPaymentNetworkConfig
	rpc    *evmRPCClient
	// Negotiated down against the RPC provider on the first "range too large"
	// rejection and reused from then on. Ankr's Polygon endpoint serves at most
	// 100 blocks per eth_getLogs, while its BSC endpoint — same key, same vendor
	// — happily serves 500, so this cannot be a shared constant.
	blockSpan int64
}

func NewPolygonScanner(config setting.CryptoPaymentNetworkConfig) *PolygonScanner {
	return &PolygonScanner{
		config:    config,
		rpc:       newEVMRPCClient(model.CryptoNetworkPolygonPOS, setting.CryptoRPCEndpoints(model.CryptoNetworkPolygonPOS)),
		blockSpan: evmMaxBlockSpan,
	}
}

func (s *PolygonScanner) Network() string { return model.CryptoNetworkPolygonPOS }

func (s *PolygonScanner) Verify(ctx context.Context) setting.CryptoNetworkHealth {
	return verifyEVMNetwork(ctx, s.Network(), s.rpc, polygonChainID, s.config)
}

func (s *PolygonScanner) ScanOnce(ctx context.Context) error {
	if s.rpc.pool.size() == 0 {
		return fmt.Errorf("Polygon RPC URL is not configured")
	}
	currentBlock, err := s.rpc.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	lastScanned := int64(0)
	if err == nil {
		lastScanned = state.LastScannedBlock
	}
	fromBlock := resumeFromBlock(s.Network(), lastScanned, currentBlock, currentBlock-int64(s.config.Confirmations)-60)
	if fromBlock < 0 {
		fromBlock = 0
	}
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if maxSafe < fromBlock {
		return nil
	}
	lastScanned, scanErr := scanEVMRange(ctx, &s.blockSpan, fromBlock, maxSafe,
		func(ctx context.Context, from int64, to int64) ([]bscRPCLog, error) {
			return s.rpc.transferLogs(ctx, s.config.Contract, s.config.ReceiveAddress, from, to)
		},
		func(logs []bscRPCLog) error {
			return recordEVMTransferLogs(ctx, s.Network(), s.config, s.rpc, logs, currentBlock)
		},
	)
	// Persist before surfacing scanErr: progress made ahead of a failing chunk is
	// still progress, and dropping it is what turned one bad request into a
	// two-month stall.
	if lastScanned >= fromBlock {
		if err := model.UpsertCryptoScannerState(s.Network(), lastScanned, maxSafe); err != nil {
			return err
		}
	}
	reportScannerProgress(s.Network(), lastScanned, maxSafe, currentBlock)
	return scanErr
}
