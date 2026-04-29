package setting

import (
	"encoding/json"
	"sort"
	"sync"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
)

var MarketplaceEnabled = true
var MarketplaceEnabledVendorTypes = DefaultMarketplaceEnabledVendorTypes()
var MarketplaceFeeRate = 0.0
var MarketplaceSellerIncomeHoldSeconds = 7 * 24 * 60 * 60
var MarketplaceMinFixedOrderQuota = 0
var MarketplaceMaxFixedOrderQuota = 0
var MarketplaceFixedOrderDefaultExpirySeconds = 30 * 24 * 60 * 60
var MarketplaceMaxSellerMultiplier = 10.0
var MarketplaceMaxCredentialConcurrency = 5

var marketplaceEnabledVendorTypesMutex sync.RWMutex

func DefaultMarketplaceEnabledVendorTypes() []int {
	vendorTypes := make([]int, 0, len(constant.ChannelTypeNames)-1)
	for vendorType := range constant.ChannelTypeNames {
		if vendorType == constant.ChannelTypeUnknown {
			continue
		}
		vendorTypes = append(vendorTypes, vendorType)
	}
	sort.Ints(vendorTypes)
	return vendorTypes
}

func MarketplaceEnabledVendorTypesToJSONString() string {
	marketplaceEnabledVendorTypesMutex.RLock()
	defer marketplaceEnabledVendorTypesMutex.RUnlock()

	vendorTypes := MarketplaceEnabledVendorTypes
	if vendorTypes == nil {
		vendorTypes = []int{}
	}
	jsonBytes, err := json.Marshal(vendorTypes)
	if err != nil {
		common.SysLog("error marshalling marketplace enabled vendor types: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateMarketplaceEnabledVendorTypesByJSONString(jsonStr string) error {
	var vendorTypes []int
	if err := json.Unmarshal([]byte(jsonStr), &vendorTypes); err != nil {
		return err
	}

	marketplaceEnabledVendorTypesMutex.Lock()
	defer marketplaceEnabledVendorTypesMutex.Unlock()
	MarketplaceEnabledVendorTypes = append([]int(nil), vendorTypes...)
	return nil
}

func IsMarketplaceVendorTypeEnabled(vendorType int) bool {
	marketplaceEnabledVendorTypesMutex.RLock()
	defer marketplaceEnabledVendorTypesMutex.RUnlock()

	for _, enabledType := range MarketplaceEnabledVendorTypes {
		if enabledType == vendorType {
			return true
		}
	}
	return false
}
