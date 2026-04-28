package crypto_payment

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

type NetworkScanner interface {
	Network() string
	ScanOnce(ctx context.Context) error
}

func StartCryptoPaymentScanners() {
	if !setting.CryptoScannerEnabled || !setting.CryptoPaymentEnabled {
		return
	}
	owner := fmt.Sprintf("%s-%d", common.GetRandomString(8), os.Getpid())
	for _, scanner := range BuildConfiguredScanners() {
		go runScannerLoop(context.Background(), scanner, owner)
	}
}

func BuildConfiguredScanners() []NetworkScanner {
	scanners := make([]NetworkScanner, 0, 4)
	for _, network := range setting.GetEnabledCryptoPaymentNetworks() {
		switch network.Network {
		case model.CryptoNetworkBSCERC20:
			scanners = append(scanners, NewBSCScanner(network))
		case model.CryptoNetworkPolygonPOS:
			scanners = append(scanners, NewPolygonScanner(network))
		case model.CryptoNetworkTronTRC20:
			scanners = append(scanners, NewTronScanner(network))
		case model.CryptoNetworkSolana:
			scanners = append(scanners, NewSolanaScanner(network))
		}
	}
	return scanners
}

func runScannerLoop(ctx context.Context, scanner NetworkScanner, owner string) {
	if !common.RedisEnabled || common.RDB == nil {
		common.SysLog("crypto scanner paused because Redis is not enabled")
		return
	}
	lock := NewScannerLock(NewRedisLockStore(common.RDB), "crypto:scanner:"+scanner.Network(), owner, 30*time.Second)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	ownsLock := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var err error
			if ownsLock {
				ownsLock, err = lock.Renew(ctx)
			} else {
				ownsLock, err = lock.Acquire(ctx)
			}
			if err != nil || !ownsLock {
				continue
			}
			if err := scanner.ScanOnce(ctx); err != nil {
				common.SysLog("crypto scanner error: " + err.Error())
			}
			if _, err := model.CompleteReadyCryptoOrders(scanner.Network()); err != nil {
				common.SysLog("crypto completion error: " + err.Error())
			}
			_, _ = lock.Renew(ctx)
		}
	}
}
