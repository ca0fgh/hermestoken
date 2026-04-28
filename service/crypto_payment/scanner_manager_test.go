package crypto_payment

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScannerManagerStartsEnabledNetworkAfterRuntimeConfigChange(t *testing.T) {
	mgr := newCryptoScannerManager(context.Background(), func(context.Context, NetworkScanner, string) {})

	withCryptoScannerSettings(t, func() {
		setting.CryptoPaymentEnabled = false
		setting.CryptoScannerEnabled = true
		setting.CryptoBSCEnabled = true
		setting.CryptoBSCReceiveAddress = "0x1111111111111111111111111111111111111111"
		setting.CryptoBSCUSDTContract = "0x55d398326f99059fF775485246999027B3197955"
		require.Empty(t, BuildConfiguredScanners())

		mgr.reconcile()
		assert.Empty(t, mgr.runningNetworks())

		setting.CryptoPaymentEnabled = true
		require.Len(t, BuildConfiguredScanners(), 1)

		mgr.reconcile()
		assert.Equal(t, []string{model.CryptoNetworkBSCERC20}, mgr.runningNetworks())
	})
}

func TestScannerManagerRestartsNetworkWhenConfigChanges(t *testing.T) {
	starts := 0
	mgr := newCryptoScannerManager(context.Background(), func(context.Context, NetworkScanner, string) {
		starts++
	})

	withCryptoScannerSettings(t, func() {
		setting.CryptoPaymentEnabled = true
		setting.CryptoScannerEnabled = true
		setting.CryptoBSCEnabled = true
		setting.CryptoBSCReceiveAddress = "0x1111111111111111111111111111111111111111"
		setting.CryptoBSCUSDTContract = "0x55d398326f99059fF775485246999027B3197955"
		setting.CryptoBSCConfirmations = 15

		mgr.reconcile()
		assert.Equal(t, []string{model.CryptoNetworkBSCERC20}, mgr.runningNetworks())
		assert.Equal(t, 1, starts)

		setting.CryptoBSCConfirmations = 20
		mgr.reconcile()
		assert.Equal(t, []string{model.CryptoNetworkBSCERC20}, mgr.runningNetworks())
		assert.Equal(t, 2, starts)
	})
}

func TestScannerManagerStopsDisabledNetwork(t *testing.T) {
	mgr := newCryptoScannerManager(context.Background(), func(context.Context, NetworkScanner, string) {})

	withCryptoScannerSettings(t, func() {
		setting.CryptoPaymentEnabled = true
		setting.CryptoScannerEnabled = true
		setting.CryptoBSCEnabled = true
		setting.CryptoBSCReceiveAddress = "0x1111111111111111111111111111111111111111"
		setting.CryptoBSCUSDTContract = "0x55d398326f99059fF775485246999027B3197955"

		mgr.reconcile()
		assert.Equal(t, []string{model.CryptoNetworkBSCERC20}, mgr.runningNetworks())

		setting.CryptoBSCEnabled = false
		mgr.reconcile()
		assert.Empty(t, mgr.runningNetworks())
	})
}

func withCryptoScannerSettings(t *testing.T, fn func()) {
	t.Helper()

	originalPaymentEnabled := setting.CryptoPaymentEnabled
	originalScannerEnabled := setting.CryptoScannerEnabled
	originalBSCEnabled := setting.CryptoBSCEnabled
	originalBSCReceiveAddress := setting.CryptoBSCReceiveAddress
	originalBSCUSDTContract := setting.CryptoBSCUSDTContract
	originalBSCConfirmations := setting.CryptoBSCConfirmations
	t.Cleanup(func() {
		setting.CryptoPaymentEnabled = originalPaymentEnabled
		setting.CryptoScannerEnabled = originalScannerEnabled
		setting.CryptoBSCEnabled = originalBSCEnabled
		setting.CryptoBSCReceiveAddress = originalBSCReceiveAddress
		setting.CryptoBSCUSDTContract = originalBSCUSDTContract
		setting.CryptoBSCConfirmations = originalBSCConfirmations
	})

	fn()
}
