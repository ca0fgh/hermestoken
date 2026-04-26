package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
)

// TaskModelPricing describes task/video billing that combines a fixed
// per-request component with a duration-based component.
type TaskModelPricing struct {
	PerRequest         float64 `json:"per_request,omitempty"`
	PerSecond          float64 `json:"per_second,omitempty"`
	IncludeOtherRatios *bool   `json:"include_other_ratios,omitempty"`
}

var taskModelPricingMap = types.NewRWMap[string, TaskModelPricing]()

func TaskModelPricing2JSONString() string {
	return taskModelPricingMap.MarshalJSONString()
}

func UpdateTaskModelPricingByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(taskModelPricingMap, jsonStr, InvalidateExposedDataCache)
}

func GetTaskModelPricing(name string) (TaskModelPricing, bool) {
	name = FormatMatchingModelName(name)
	if pricing, ok := taskModelPricingMap.Get(name); ok && pricing.Enabled() {
		return pricing, true
	}
	if strings.HasSuffix(name, CompactModelSuffix) {
		if price, ok := taskModelPricingMap.Get(CompactWildcardModelKey); ok && price.Enabled() {
			return price, true
		}
	}
	return TaskModelPricing{}, false
}

func GetTaskModelPricingCopy() map[string]TaskModelPricing {
	return taskModelPricingMap.ReadAll()
}

func (p TaskModelPricing) Enabled() bool {
	return p.PerRequest > 0 || p.PerSecond > 0
}

func (p TaskModelPricing) ShouldIncludeOtherRatios() bool {
	return p.IncludeOtherRatios == nil || *p.IncludeOtherRatios
}
