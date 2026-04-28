package crypto_payment

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
)

func TestBuildConfiguredScannersIncludesPolygonAndSolana(t *testing.T) {
	originalPaymentEnabled := setting.CryptoPaymentEnabled
	originalBSCEnabled := setting.CryptoBSCEnabled
	originalPolygonEnabled := setting.CryptoPolygonEnabled
	originalSolanaEnabled := setting.CryptoSolanaEnabled
	originalBSCReceiveAddress := setting.CryptoBSCReceiveAddress
	originalPolygonReceiveAddress := setting.CryptoPolygonReceiveAddress
	originalSolanaReceiveAddress := setting.CryptoSolanaReceiveAddress
	t.Cleanup(func() {
		setting.CryptoPaymentEnabled = originalPaymentEnabled
		setting.CryptoBSCEnabled = originalBSCEnabled
		setting.CryptoPolygonEnabled = originalPolygonEnabled
		setting.CryptoSolanaEnabled = originalSolanaEnabled
		setting.CryptoBSCReceiveAddress = originalBSCReceiveAddress
		setting.CryptoPolygonReceiveAddress = originalPolygonReceiveAddress
		setting.CryptoSolanaReceiveAddress = originalSolanaReceiveAddress
	})

	setting.CryptoPaymentEnabled = true
	setting.CryptoBSCEnabled = true
	setting.CryptoBSCReceiveAddress = "0x1111111111111111111111111111111111111111"
	setting.CryptoPolygonEnabled = true
	setting.CryptoPolygonReceiveAddress = "0x2222222222222222222222222222222222222222"
	setting.CryptoSolanaEnabled = true
	setting.CryptoSolanaReceiveAddress = "7YttLkHDoWJYNNe7U2s1owz8FC6xk4kZqGSPdU2ovbYW"

	scanners := BuildConfiguredScanners()
	networks := make([]string, 0, len(scanners))
	for _, scanner := range scanners {
		networks = append(networks, scanner.Network())
	}

	assert.Contains(t, networks, model.CryptoNetworkBSCERC20)
	assert.Contains(t, networks, model.CryptoNetworkPolygonPOS)
	assert.Contains(t, networks, model.CryptoNetworkSolana)
}
