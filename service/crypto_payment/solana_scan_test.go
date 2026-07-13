package crypto_payment

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	solanaReceiveWallet = "8hYEP92cyDTjGpLRi7bwJ47FpTAQGCxYJH7i9gtFZLsY"
	solanaUSDTMint      = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	solanaReceiveATA    = "A6zaWWqFzgxuJCDss2Qt3qT76gYs6MuZ9YfLvmC2gfPn"
)

// solanaFakeRPC records which addresses were asked about and replies with canned
// results keyed by method.
type solanaFakeRPC struct {
	server            *httptest.Server
	signatureRequests []string
	responses         map[string]string
	pages             map[string][]string
}

func newSolanaFakeRPC(t *testing.T) *solanaFakeRPC {
	t.Helper()
	fake := &solanaFakeRPC{responses: map[string]string{}, pages: map[string][]string{}}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var request struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		require.NoError(t, common.Unmarshal(body, &request))

		if request.Method == "getSignaturesForAddress" {
			address, _ := request.Params[0].(string)
			fake.signatureRequests = append(fake.signatureRequests, address)
			pages := fake.pages[address]
			page := 0
			if options, ok := request.Params[1].(map[string]any); ok {
				if before, ok := options["before"].(string); ok && before != "" {
					page = fake.pageAfter(address, before)
				}
			}
			if page >= len(pages) {
				fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":[]}`)
				return
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, pages[page])
			return
		}
		response, ok := fake.responses[request.Method]
		require.True(t, ok, "unexpected RPC method %s", request.Method)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, response)
	}))
	t.Cleanup(fake.server.Close)

	original := setting.CryptoSolanaRPCURL
	setting.CryptoSolanaRPCURL = fake.server.URL
	t.Cleanup(func() { setting.CryptoSolanaRPCURL = original })
	return fake
}

// pageAfter maps a "before" cursor back to the index of the next page.
func (f *solanaFakeRPC) pageAfter(address string, before string) int {
	for index, page := range f.pages[address] {
		if strings.Contains(page, `"signature":"`+before+`"`) {
			return index + 1
		}
	}
	return len(f.pages[address])
}

func solanaTestScanner() *SolanaScanner {
	return NewSolanaScanner(setting.CryptoPaymentNetworkConfig{
		Network:        "solana",
		Contract:       solanaUSDTMint,
		ReceiveAddress: solanaReceiveWallet,
		Decimals:       6,
		Confirmations:  32,
	})
}

// The bug: an SPL transfer credits the receiver's associated token account and
// never lists the owner wallet among the transaction's account keys, so
// getSignaturesForAddress(wallet) — all the old scanner ever called — returns
// nothing for an incoming deposit. Verified against a real mainnet USDT transfer:
// the wallet's last 1000 signatures did not contain it; the ATA's did.
func TestSolanaScannerAsksTheTokenAccountNotJustTheWallet(t *testing.T) {
	fake := newSolanaFakeRPC(t)
	fake.responses["getTokenAccountsByOwner"] = `{"value":[{"pubkey":"` + solanaReceiveATA + `"}]}`

	addresses, err := solanaTestScanner().signatureAddresses(context.Background())
	require.NoError(t, err)

	assert.Contains(t, addresses, solanaReceiveATA, "the token account is where a deposit actually lands")
	assert.Contains(t, addresses, solanaReceiveWallet, "the wallet still matters: the transfer that creates the token account lists the owner")
}

// Before the token account exists, a deposit's transaction creates it and does
// list the owner — so the wallet alone is the correct subject, and the scanner
// must not fail just because there is no token account yet.
func TestSolanaScannerFallsBackToWalletBeforeTheTokenAccountExists(t *testing.T) {
	fake := newSolanaFakeRPC(t)
	fake.responses["getTokenAccountsByOwner"] = `{"value":[]}`

	addresses, err := solanaTestScanner().signatureAddresses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{solanaReceiveWallet}, addresses)
}

func TestSolanaScannerSkipsFailedAndOutOfRangeSignatures(t *testing.T) {
	fake := newSolanaFakeRPC(t)
	fake.pages[solanaReceiveATA] = []string{`[
		{"signature":"tooNew","slot":900},
		{"signature":"failed","slot":500,"err":{"InstructionError":[0,"Custom"]}},
		{"signature":"good","slot":400},
		{"signature":"alreadyScanned","slot":100}
	]`}

	signatures, err := solanaTestScanner().signaturesForAddress(context.Background(), solanaReceiveATA, 200, 800)
	require.NoError(t, err)
	require.Len(t, signatures, 1)
	assert.Equal(t, "good", signatures[0].Signature)
}

// getSignaturesForAddress returns newest-first and cannot be given a slot range,
// so covering [fromSlot, maxSafe] means paging backwards until the cursor is
// passed. The old code took one unpaged page and then advanced the cursor to the
// chain tip anyway, permanently skipping everything it had not looked at.
func TestSolanaScannerPagesBackwardsUntilItReachesTheCursor(t *testing.T) {
	fake := newSolanaFakeRPC(t)
	newest := make([]string, 0, solanaSignaturePageLimit)
	for i := 0; i < solanaSignaturePageLimit; i++ {
		newest = append(newest, fmt.Sprintf(`{"signature":"page1-%d","slot":%d}`, i, 5000-i))
	}
	fake.pages[solanaReceiveATA] = []string{
		"[" + strings.Join(newest, ",") + "]",
		`[{"signature":"page2-deposit","slot":3500},{"signature":"page2-old","slot":900}]`,
	}

	signatures, err := solanaTestScanner().signaturesForAddress(context.Background(), solanaReceiveATA, 1000, 6000)
	require.NoError(t, err)

	assert.Len(t, fake.signatureRequests, 2, "a full page must not be treated as the end of the history")
	found := false
	for _, signature := range signatures {
		if signature.Signature == "page2-deposit" {
			found = true
		}
	}
	assert.True(t, found, "a deposit below the first page must still be collected")
}

func TestSolanaScannerRefusesToClaimCoverageItCannotProve(t *testing.T) {
	fake := newSolanaFakeRPC(t)
	page := make([]string, 0, solanaSignaturePageLimit)
	for i := 0; i < solanaSignaturePageLimit; i++ {
		page = append(page, fmt.Sprintf(`{"signature":"sig-%d","slot":%d}`, i, 900000-i))
	}
	full := "[" + strings.Join(page, ",") + "]"
	for i := 0; i <= solanaMaxSignaturePages; i++ {
		fake.pages[solanaReceiveATA] = append(fake.pages[solanaReceiveATA], full)
	}

	// Every page is full and none reaches the cursor, so coverage is unprovable and
	// the scan must fail rather than let the cursor skip the gap.
	_, err := solanaTestScanner().signaturesForAddress(context.Background(), solanaReceiveATA, 1, 1000000)
	require.Error(t, err)
	assert.ErrorContains(t, err, "deeper than")
}
