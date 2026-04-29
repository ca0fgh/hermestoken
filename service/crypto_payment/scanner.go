package crypto_payment

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

type NetworkScanner interface {
	Network() string
	ScanOnce(ctx context.Context) error
}

type scannerLauncher func(context.Context, NetworkScanner, string)

type managedScanner struct {
	signature string
	cancel    context.CancelFunc
}

type cryptoScannerManager struct {
	root     context.Context
	owner    string
	launch   scannerLauncher
	mu       sync.Mutex
	running  map[string]managedScanner
	started  bool
	shutdown context.CancelFunc
}

var defaultCryptoScannerManager = newCryptoScannerManager(context.Background(), func(ctx context.Context, scanner NetworkScanner, owner string) {
	go runScannerLoop(ctx, scanner, owner)
})

func StartCryptoPaymentScanners() {
	defaultCryptoScannerManager.startSupervisor()
}

func EnsureCryptoPaymentScanners() {
	defaultCryptoScannerManager.reconcile()
}

func newCryptoScannerManager(root context.Context, launch scannerLauncher) *cryptoScannerManager {
	if root == nil {
		root = context.Background()
	}
	if launch == nil {
		launch = func(ctx context.Context, scanner NetworkScanner, owner string) {
			go runScannerLoop(ctx, scanner, owner)
		}
	}
	return &cryptoScannerManager{
		root:    root,
		owner:   fmt.Sprintf("%s-%d", common.GetRandomString(8), os.Getpid()),
		launch:  launch,
		running: make(map[string]managedScanner),
	}
}

func (m *cryptoScannerManager) startSupervisor() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(m.root)
	m.shutdown = cancel
	m.started = true
	m.mu.Unlock()

	go func() {
		m.reconcile()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				m.stopAll()
				return
			case <-ticker.C:
				m.reconcile()
			}
		}
	}()
}

func (m *cryptoScannerManager) reconcile() {
	desired := map[string]NetworkScanner{}
	if setting.CryptoScannerEnabled && setting.CryptoPaymentEnabled {
		for _, scanner := range BuildConfiguredScanners() {
			desired[scanner.Network()] = scanner
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for network, managed := range m.running {
		scanner, ok := desired[network]
		if !ok || managed.signature != scannerSignature(scanner) {
			managed.cancel()
			delete(m.running, network)
			common.SysLog("crypto scanner stopped: " + network)
		}
	}

	for network, scanner := range desired {
		if _, ok := m.running[network]; ok {
			continue
		}
		scannerCtx, cancel := context.WithCancel(m.root)
		m.running[network] = managedScanner{
			signature: scannerSignature(scanner),
			cancel:    cancel,
		}
		m.launch(scannerCtx, scanner, m.owner)
		common.SysLog("crypto scanner started: " + network)
	}
}

func (m *cryptoScannerManager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for network, managed := range m.running {
		managed.cancel()
		delete(m.running, network)
	}
}

func (m *cryptoScannerManager) runningNetworks() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	networks := make([]string, 0, len(m.running))
	for network := range m.running {
		networks = append(networks, network)
	}
	sort.Strings(networks)
	return networks
}

func scannerSignature(scanner NetworkScanner) string {
	switch s := scanner.(type) {
	case *BSCScanner:
		return networkConfigSignature(s.config)
	case *PolygonScanner:
		return networkConfigSignature(s.config)
	case *TronScanner:
		return networkConfigSignature(s.config)
	case *SolanaScanner:
		return networkConfigSignature(s.config)
	default:
		return scanner.Network()
	}
}

func networkConfigSignature(config setting.CryptoPaymentNetworkConfig) string {
	parts := []string{
		config.Network,
		strings.ToLower(strings.TrimSpace(config.Contract)),
		strings.ToLower(strings.TrimSpace(config.ReceiveAddress)),
		fmt.Sprintf("%d", config.Decimals),
		fmt.Sprintf("%d", config.Confirmations),
	}
	return strings.Join(parts, "|")
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
