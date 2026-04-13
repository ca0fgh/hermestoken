package common

import (
	"encoding/json"
	"strings"
	"sync"
)

const subscriptionReferralGroupRateMaxBps = 10000

var (
	subscriptionReferralGroupRates           = make(map[string]int)
	subscriptionReferralGroupRatesConfigured bool
	subscriptionReferralGroupRatesMutex      sync.RWMutex
)

func SubscriptionReferralGroupRates2JSONString() string {
	subscriptionReferralGroupRatesMutex.RLock()
	defer subscriptionReferralGroupRatesMutex.RUnlock()

	jsonBytes, err := json.Marshal(subscriptionReferralGroupRates)
	if err != nil {
		SysError("error marshalling subscription referral group rates: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func UpdateSubscriptionReferralGroupRatesByJSONString(jsonStr string) error {
	next := make(map[string]int)
	if strings.TrimSpace(jsonStr) != "" {
		if err := json.Unmarshal([]byte(jsonStr), &next); err != nil {
			return err
		}
	}

	normalized := make(map[string]int, len(next))
	for group, rate := range next {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		normalized[trimmedGroup] = normalizeSubscriptionReferralGroupRate(rate)
	}

	subscriptionReferralGroupRatesMutex.Lock()
	defer subscriptionReferralGroupRatesMutex.Unlock()
	subscriptionReferralGroupRates = normalized
	subscriptionReferralGroupRatesConfigured = len(normalized) > 0
	return nil
}

func GetSubscriptionReferralGroupRate(group string) int {
	subscriptionReferralGroupRatesMutex.RLock()
	defer subscriptionReferralGroupRatesMutex.RUnlock()
	return subscriptionReferralGroupRates[strings.TrimSpace(group)]
}

func GetSubscriptionReferralGroupRatesCopy() map[string]int {
	subscriptionReferralGroupRatesMutex.RLock()
	defer subscriptionReferralGroupRatesMutex.RUnlock()

	copyMap := make(map[string]int, len(subscriptionReferralGroupRates))
	for group, rate := range subscriptionReferralGroupRates {
		copyMap[group] = rate
	}
	return copyMap
}

func HasSubscriptionReferralGroupRatesConfigured() bool {
	subscriptionReferralGroupRatesMutex.RLock()
	defer subscriptionReferralGroupRatesMutex.RUnlock()
	return subscriptionReferralGroupRatesConfigured
}

func normalizeSubscriptionReferralGroupRate(rate int) int {
	if rate < 0 {
		return 0
	}
	if rate > subscriptionReferralGroupRateMaxBps {
		return subscriptionReferralGroupRateMaxBps
	}
	return rate
}
