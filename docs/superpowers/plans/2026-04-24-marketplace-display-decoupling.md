# Marketplace Display Decoupling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make HermesToken's marketplace a pure display surface: guests only see `default` display models, logged-in users see all display models, and actual model usage permissions stay controlled by channels and abilities.

**Architecture:** Add a Go-side parser for `HeaderNavModules.pricing`, then change `/api/pricing` from a "usable models" endpoint into a "marketplace display" endpoint that emits `display_groups` and only uses guest-vs-authenticated identity to shape visibility. On the frontend, gate `/pricing` by the marketplace switch, replace `usable_group`-driven filters with `display_groups`, and relabel model admin controls so `model_meta.status` is explicitly a marketplace-display toggle rather than a usage toggle.

**Tech Stack:** Go, Gin, GORM, React 18, React Router, Semi UI, node:test.

---

## File Structure / Responsibility Map

- `controller/header_nav_modules.go`
  - Parse `HeaderNavModules` inside Go with the same defaults and backward-compatibility rules as the frontend helper.
- `controller/header_nav_modules_test.go`
  - Lock default parsing, legacy `pricing: false`, and structured `pricing.enabled/requireAuth` parsing.
- `controller/pricing.go`
  - Gate the marketplace by `pricing.enabled`, filter guest results to `default`, keep logged-in results unfiltered by usable groups, and emit `display_groups`.
- `controller/pricing_marketplace_display_test.go`
  - Cover disabled marketplace behavior, guest-only `default` visibility, authenticated all-display visibility, and hidden-model exclusion.
- `web/src/helpers/headerNavModules.js`
  - Expose the full pricing module config so route gating can read `enabled` and `requireAuth` together.
- `web/src/App.jsx`
  - Pass both marketplace flags into `PublicRoutes`.
- `web/src/routes/PublicRoutes.jsx`
  - Render `NotFound` for `/pricing` when the marketplace is disabled, and preserve the existing auth gate when it is enabled.
- `web/src/hooks/model-pricing/useModelPricingData.jsx`
  - Read `display_groups` from `/api/pricing`, stop using `usable_group` to drive marketplace display, and pass display-group state downstream.
- `web/src/components/table/model-pricing/filter/PricingGroups.jsx`
  - Render the marketplace group selector from `display_groups` and rename the section to `模型分组`.
- `web/src/components/table/model-pricing/layout/PricingSidebar.jsx`
  - Pass `displayGroups` into the sidebar group filter.
- `web/src/components/table/model-pricing/modal/components/FilterModalContent.jsx`
  - Pass `displayGroups` into the mobile filter sheet.
- `web/src/components/table/model-pricing/layout/PricingPage.jsx`
  - Stop passing `usableGroup` / `autoGroups` into the model detail sheet once display-only grouping takes over.
- `web/src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx`
  - Receive `displayGroups` instead of `usableGroup`.
- `web/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx`
  - Build the model detail price table from `display_groups` and drop `auto` call-chain UI from the marketplace detail sheet.
- `web/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx`
  - Update marketplace copy so the switch text matches the new guest-vs-user display semantics.
- `web/src/components/table/models/modals/EditModelModal.jsx`
  - Rename the `status` switch to `模型广场展示`.
- `web/src/components/table/models/ModelsColumnDefs.jsx`
  - Rename the status column to `广场展示`.
- `web/tests/header-nav-modules.test.mjs`
  - Protect helper defaults and the updated settings copy.
- `web/tests/pricing-route-gating.test.mjs`
  - Protect `pricing.enabled` route gating in `App.jsx` and `PublicRoutes.jsx`.
- `web/tests/pricing-marketplace-display.test.mjs`
  - Protect `display_groups` usage in the marketplace hook/components and the admin copy changes.

### Task 1: Add Go-Side HeaderNavModules Pricing Parsing

**Files:**
- Create: `controller/header_nav_modules.go`
- Create: `controller/header_nav_modules_test.go`
- Test: `controller/header_nav_modules_test.go`

- [ ] **Step 1: Write the failing parser tests**

```go
package controller

import "testing"

func TestGetPricingHeaderNavConfigDefaultsWhenOptionIsEmpty(t *testing.T) {
	config := getPricingHeaderNavConfig("")
	if !config.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false")
	}
}

func TestGetPricingHeaderNavConfigSupportsLegacyBooleanPricing(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":false}`)
	if config.Enabled {
		t.Fatal("Enabled = true, want false for legacy boolean config")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false for legacy boolean config")
	}
}

func TestGetPricingHeaderNavConfigSupportsStructuredPricingObject(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":{"enabled":true,"requireAuth":true}}`)
	if !config.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if !config.RequireAuth {
		t.Fatal("RequireAuth = false, want true")
	}
}
```

- [ ] **Step 2: Run the parser tests to verify they fail**

Run: `go test ./controller -run 'TestGetPricingHeaderNavConfigDefaultsWhenOptionIsEmpty|TestGetPricingHeaderNavConfigSupportsLegacyBooleanPricing|TestGetPricingHeaderNavConfigSupportsStructuredPricingObject'`

Expected: FAIL because `getPricingHeaderNavConfig` does not exist yet.

- [ ] **Step 3: Implement the Go-side pricing config parser**

```go
// controller/header_nav_modules.go
package controller

import (
	"encoding/json"
	"strings"
)

type pricingHeaderNavConfig struct {
	Enabled     bool
	RequireAuth bool
}

type pricingHeaderNavPayload struct {
	Enabled     *bool `json:"enabled"`
	RequireAuth bool  `json:"requireAuth"`
}

func getPricingHeaderNavConfig(raw string) pricingHeaderNavConfig {
	config := pricingHeaderNavConfig{
		Enabled:     true,
		RequireAuth: false,
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return config
	}

	pricingRaw, ok := root["pricing"]
	if !ok {
		return config
	}

	var legacy bool
	if err := json.Unmarshal(pricingRaw, &legacy); err == nil {
		config.Enabled = legacy
		return config
	}

	var payload pricingHeaderNavPayload
	if err := json.Unmarshal(pricingRaw, &payload); err != nil {
		return config
	}

	if payload.Enabled != nil {
		config.Enabled = *payload.Enabled
	}
	config.RequireAuth = payload.RequireAuth
	return config
}
```

- [ ] **Step 4: Run the parser tests to verify they pass**

Run: `go test ./controller -run 'TestGetPricingHeaderNavConfigDefaultsWhenOptionIsEmpty|TestGetPricingHeaderNavConfigSupportsLegacyBooleanPricing|TestGetPricingHeaderNavConfigSupportsStructuredPricingObject'`

Expected: PASS with all three tests green.

- [ ] **Step 5: Commit the parser helper**

```bash
git add controller/header_nav_modules.go controller/header_nav_modules_test.go
git commit -m "feat: parse pricing header nav config in go"
```

### Task 2: Convert `/api/pricing` into a Marketplace Display Endpoint

**Files:**
- Modify: `controller/pricing.go`
- Create: `controller/pricing_marketplace_display_test.go`
- Test: `controller/pricing_marketplace_display_test.go`
- Test: `controller/pricing_timing_test.go`

- [ ] **Step 1: Write the failing marketplace display tests**

```go
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type marketplacePricingResponse struct {
	Success       bool                    `json:"success"`
	Data          []model.Pricing         `json:"data"`
	DisplayGroups map[string]string       `json:"display_groups"`
	GroupRatio    map[string]float64      `json:"group_ratio"`
}

func withHeaderNavModulesOption(t *testing.T, raw string) {
	t.Helper()
	original := common.OptionMap["HeaderNavModules"]
	common.OptionMap["HeaderNavModules"] = raw
	t.Cleanup(func() {
		common.OptionMap["HeaderNavModules"] = original
	})
}

func seedPricingModelMeta(t *testing.T, db *gorm.DB, modelName string, status int) {
	t.Helper()
	record := &model.Model{
		ModelName:    modelName,
		Status:       status,
		SyncOfficial: 1,
		NameRule:     model.NameRuleExact,
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to create model meta: %v", err)
	}
}

func decodeMarketplacePricingResponse(t *testing.T, recorder *httptest.ResponseRecorder) marketplacePricingResponse {
	t.Helper()
	var response marketplacePricingResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode marketplace pricing response: %v", err)
	}
	return response
}

func newAuthenticatedPricingContext(t *testing.T, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestGetPricingReturnsEmptyMarketplaceDataWhenMarketplaceDisabled(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(t, `{}`, `{"default":1}`, `{"default":{"default":1}}`)
	withHeaderNavModulesOption(t, `{"pricing":{"enabled":false,"requireAuth":false}}`)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingModelMeta(t, db, "gpt-default", 1)
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if !response.Success {
		t.Fatal("expected success response when marketplace is disabled")
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected no marketplace data, got %d items", len(response.Data))
	}
	if len(response.DisplayGroups) != 0 {
		t.Fatalf("expected no display groups, got %#v", response.DisplayGroups)
	}
}

func TestGetPricingShowsOnlyDefaultMarketplaceModelsToGuests(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(t, `{}`, `{"default":1,"vip":2}`, `{"default":{"default":0.75,"vip":2}}`)
	withHeaderNavModulesOption(t, `{"pricing":{"enabled":true,"requireAuth":false}}`)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingAbility(t, db, "vip", "gpt-vip")
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingModelMeta(t, db, "gpt-vip", 1)
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if len(response.Data) != 1 || response.Data[0].ModelName != "gpt-default" {
		t.Fatalf("guest marketplace data = %#v, want only gpt-default", response.Data)
	}
	if len(response.DisplayGroups) != 1 || response.DisplayGroups["default"] == "" {
		t.Fatalf("guest display groups = %#v, want only default", response.DisplayGroups)
	}
	if response.GroupRatio["default"] != 0.75 {
		t.Fatalf("guest default ratio = %v, want 0.75", response.GroupRatio["default"])
	}
}

func TestGetPricingShowsAllDisplayModelsToAuthenticatedUsers(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(t, `{}`, `{"default":1,"vip":2}`, `{"member":{"default":1,"vip":2}}`)
	withHeaderNavModulesOption(t, `{"pricing":{"enabled":true,"requireAuth":false}}`)
	user := &model.User{Username: "member-user", Password: "secret", Group: "member", Status: common.UserStatusEnabled}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingAbility(t, db, "vip", "gpt-vip")
	seedPricingAbility(t, db, "default", "gpt-hidden")
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingModelMeta(t, db, "gpt-vip", 1)
	seedPricingModelMeta(t, db, "gpt-hidden", 0)
	model.RefreshPricing()

	ctx, recorder := newAuthenticatedPricingContext(t, user.Id)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if len(response.Data) != 2 {
		t.Fatalf("expected two visible marketplace models, got %d", len(response.Data))
	}
	if _, ok := response.DisplayGroups["default"]; !ok {
		t.Fatalf("display groups = %#v, want default", response.DisplayGroups)
	}
	if _, ok := response.DisplayGroups["vip"]; !ok {
		t.Fatalf("display groups = %#v, want vip", response.DisplayGroups)
	}
}
```

- [ ] **Step 2: Run the marketplace display tests to verify they fail**

Run: `go test ./controller -run 'TestGetPricingReturnsEmptyMarketplaceDataWhenMarketplaceDisabled|TestGetPricingShowsOnlyDefaultMarketplaceModelsToGuests|TestGetPricingShowsAllDisplayModelsToAuthenticatedUsers|TestGetPricingSetsServerTimingHeader'`

Expected: FAIL because `display_groups` is not emitted yet, guests still use `usable_group`, logged-in users are still filtered by usable groups, and the disabled marketplace path does not exist.

- [ ] **Step 3: Implement marketplace display filtering in the pricing controller**

```go
// controller/pricing.go
func filterPricingBySpecificGroup(pricing []model.Pricing, group string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, group) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func buildDisplayGroups(pricing []model.Pricing, guest bool) map[string]string {
	displayGroups := make(map[string]string)
	for _, item := range pricing {
		for _, group := range item.EnableGroup {
			if group == "" {
				continue
			}
			if guest && group != "default" {
				continue
			}
			displayGroups[group] = group
		}
	}
	return displayGroups
}

func filterGroupRatiosByDisplayGroups(groupRatio map[string]float64, displayGroups map[string]string) map[string]float64 {
	filtered := make(map[string]float64, len(displayGroups))
	for group := range displayGroups {
		if ratio, ok := groupRatio[group]; ok {
			filtered[group] = ratio
		}
	}
	return filtered
}

func emptyMarketplacePricingResponse() gin.H {
	return gin.H{
		"success":            true,
		"data":               []model.Pricing{},
		"vendors":            []model.PricingVendor{},
		"group_ratio":        map[string]float64{},
		"usable_group":       map[string]string{},
		"display_groups":     map[string]string{},
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        []string{},
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	}
}

func GetPricing(c *gin.Context) {
	totalStart := time.Now()
	pricingConfig := getPricingHeaderNavConfig(common.OptionMap["HeaderNavModules"])
	if !pricingConfig.Enabled {
		setServerTiming(c, serverTimingMetric{name: "pricing_total", dur: float64(time.Since(totalStart).Microseconds()) / 1000})
		c.JSON(200, emptyMarketplacePricingResponse())
		return
	}

	modelStart := time.Now()
	pricing := model.GetPricing()
	modelDuration := time.Since(modelStart)

	contextStart := time.Now()
	userID, exists := c.Get("id")
	currentUserID := 0
	displayGroup := "default"
	groupRatio := map[string]float64{}
	for group, ratio := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[group] = ratio
	}

	if exists {
		currentUserID = userID.(int)
		user, err := model.GetUserCache(currentUserID)
		if err == nil && user.Group != "" {
			displayGroup = user.Group
		}
		for group := range groupRatio {
			if ratio, ok := ratio_setting.GetGroupGroupRatio(displayGroup, group); ok {
				groupRatio[group] = ratio
			}
		}
	}

	usableGroup := service.GetUserUsableGroupsForUser(currentUserID, displayGroup)
	autoGroups := service.GetUserAutoGroupForUser(currentUserID, displayGroup)
	contextDuration := time.Since(contextStart)

	filterStart := time.Now()
	if !exists {
		pricing = filterPricingBySpecificGroup(pricing, "default")
	}
	displayGroups := buildDisplayGroups(pricing, !exists)
	groupRatio = filterGroupRatiosByDisplayGroups(groupRatio, displayGroups)
	filterDuration := time.Since(filterStart)

	responsePayload := gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"display_groups":     displayGroups,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        autoGroups,
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	}

	setServerTiming(c,
		serverTimingMetric{name: "pricing_model", dur: float64(modelDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_context", dur: float64(contextDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_filter", dur: float64(filterDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_total", dur: float64(time.Since(totalStart).Microseconds()) / 1000},
	)
	c.JSON(200, responsePayload)
}
```

- [ ] **Step 4: Run the backend pricing tests to verify they pass**

Run: `go test ./controller -run 'TestGetPricingReturnsEmptyMarketplaceDataWhenMarketplaceDisabled|TestGetPricingShowsOnlyDefaultMarketplaceModelsToGuests|TestGetPricingShowsAllDisplayModelsToAuthenticatedUsers|TestGetPricingSetsServerTimingHeader'`

Expected: PASS with the disabled, guest, authenticated, and timing cases all green.

- [ ] **Step 5: Commit the backend marketplace behavior**

```bash
git add controller/pricing.go controller/pricing_marketplace_display_test.go
git commit -m "feat: decouple marketplace pricing display from usage"
```

### Task 3: Gate `/pricing` by the Marketplace Switch on the Frontend

**Files:**
- Modify: `web/src/helpers/headerNavModules.js`
- Modify: `web/src/App.jsx`
- Modify: `web/src/routes/PublicRoutes.jsx`
- Create: `web/tests/pricing-route-gating.test.mjs`
- Test: `web/tests/pricing-route-gating.test.mjs`

- [ ] **Step 1: Write the failing route gating source tests**

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const helperPath = new URL('../src/helpers/headerNavModules.js', import.meta.url);
const appPath = new URL('../src/App.jsx', import.meta.url);
const publicRoutesPath = new URL('../src/routes/PublicRoutes.jsx', import.meta.url);

test('header nav helper exposes the full pricing config object', async () => {
  const source = await readFile(helperPath, 'utf8');
  assert.match(source, /export function getPricingModuleConfig/);
  assert.match(source, /return normalizeHeaderNavModules\(modules\)\.pricing;/);
});

test('app forwards pricingEnabled and pricingRequireAuth into PublicRoutes', async () => {
  const source = await readFile(appPath, 'utf8');
  assert.match(source, /const pricingConfig = useMemo/);
  assert.match(source, /pricingEnabled=\{pricingConfig\.enabled\}/);
  assert.match(source, /pricingRequireAuth=\{pricingConfig\.requireAuth\}/);
});

test('public routes render NotFound for pricing when the marketplace is disabled', async () => {
  const source = await readFile(publicRoutesPath, 'utf8');
  assert.match(source, /function PublicRoutes\(\{ pricingEnabled = true, pricingRequireAuth = false \}\)/);
  assert.match(source, /path='\/pricing'/);
  assert.match(source, /pricingEnabled \?/);
  assert.match(source, /renderWithSuspense\(<NotFound \/>/);
});
```

- [ ] **Step 2: Run the route gating tests to verify they fail**

Run: `cd web && node --test tests/pricing-route-gating.test.mjs`

Expected: FAIL because the helper does not expose the full pricing config yet, `App.jsx` only forwards `pricingRequireAuth`, and `PublicRoutes` does not guard `/pricing` by `pricingEnabled`.

- [ ] **Step 3: Implement frontend marketplace route gating**

```js
// web/src/helpers/headerNavModules.js
export function getPricingModuleConfig(modules) {
  return normalizeHeaderNavModules(modules).pricing;
}

export function getPricingRequireAuth(modules) {
  return getPricingModuleConfig(modules).requireAuth === true;
}
```

```jsx
// web/src/App.jsx
import { getPricingModuleConfig } from './helpers/headerNavModules';

const pricingConfig = useMemo(() => {
  return getPricingModuleConfig(statusState?.status?.HeaderNavModules);
}, [statusState?.status?.HeaderNavModules]);

<RoutesComponent
  pricingEnabled={pricingConfig.enabled}
  pricingRequireAuth={pricingConfig.requireAuth}
/>
```

```jsx
// web/src/routes/PublicRoutes.jsx
function PublicRoutes({
  pricingEnabled = true,
  pricingRequireAuth = false,
}) {
  // ...
  <Route
    path='/pricing'
    element={
      pricingEnabled ? (
        pricingRequireAuth ? (
          <PrivateRoute>{renderWithSuspense(<Pricing />)}</PrivateRoute>
        ) : (
          renderWithSuspense(<Pricing />)
        )
      ) : (
        renderWithSuspense(<NotFound />, 'pricing-not-found')
      )
    }
  />
}
```

- [ ] **Step 4: Run the route gating tests to verify they pass**

Run: `cd web && node --test tests/pricing-route-gating.test.mjs`

Expected: PASS with all three source assertions green.

- [ ] **Step 5: Commit the route gating changes**

```bash
git add web/src/helpers/headerNavModules.js web/src/App.jsx web/src/routes/PublicRoutes.jsx web/tests/pricing-route-gating.test.mjs
git commit -m "feat: gate marketplace routes by display settings"
```

### Task 4: Switch the Marketplace UI from `usable_group` to `display_groups`

**Files:**
- Modify: `web/src/hooks/model-pricing/useModelPricingData.jsx`
- Modify: `web/src/components/table/model-pricing/filter/PricingGroups.jsx`
- Modify: `web/src/components/table/model-pricing/layout/PricingSidebar.jsx`
- Modify: `web/src/components/table/model-pricing/modal/components/FilterModalContent.jsx`
- Modify: `web/src/components/table/model-pricing/layout/PricingPage.jsx`
- Modify: `web/src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx`
- Modify: `web/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx`
- Create: `web/tests/pricing-marketplace-display.test.mjs`
- Test: `web/tests/pricing-marketplace-display.test.mjs`

- [ ] **Step 1: Write the failing marketplace display source tests**

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const hookPath = new URL('../src/hooks/model-pricing/useModelPricingData.jsx', import.meta.url);
const groupsPath = new URL('../src/components/table/model-pricing/filter/PricingGroups.jsx', import.meta.url);
const sidebarPath = new URL('../src/components/table/model-pricing/layout/PricingSidebar.jsx', import.meta.url);
const modalFilterPath = new URL('../src/components/table/model-pricing/modal/components/FilterModalContent.jsx', import.meta.url);
const detailPath = new URL('../src/components/table/model-pricing/modal/components/ModelPricingTable.jsx', import.meta.url);

test('pricing hook stores display groups from the pricing API', async () => {
  const source = await readFile(hookPath, 'utf8');
  assert.match(source, /const \[displayGroups, setDisplayGroups\] = useState\(\{\}\);/);
  assert.match(source, /display_groups,/);
  assert.match(source, /setDisplayGroups\(display_groups \|\| \{\}\);/);
});

test('pricing groups component uses displayGroups and the 模型分组 label', async () => {
  const source = await readFile(groupsPath, 'utf8');
  assert.match(source, /displayGroups = \{\}/);
  assert.match(source, /title=\{t\('模型分组'\)\}/);
  assert.doesNotMatch(source, /usableGroup/);
});

test('sidebar surfaces and filter modal both pass displayGroups into PricingGroups', async () => {
  const sidebarSource = await readFile(sidebarPath, 'utf8');
  const filterSource = await readFile(modalFilterPath, 'utf8');
  assert.match(sidebarSource, /displayGroups=\{categoryProps\.displayGroups\}/);
  assert.match(filterSource, /displayGroups=\{categoryProps\.displayGroups\}/);
});

test('model pricing detail uses displayGroups and no longer renders auto call-chain state', async () => {
  const source = await readFile(detailPath, 'utf8');
  assert.match(source, /displayGroups/);
  assert.doesNotMatch(source, /usableGroup/);
  assert.doesNotMatch(source, /autoGroups/);
  assert.doesNotMatch(source, /auto分组调用链路/);
});
```

- [ ] **Step 2: Run the marketplace display source tests to verify they fail**

Run: `cd web && node --test tests/pricing-marketplace-display.test.mjs`

Expected: FAIL because the hook still stores `usableGroup`, the group filter still says `可用令牌分组`, and the detail sheet still uses `usableGroup` / `autoGroups`.

- [ ] **Step 3: Implement `display_groups` plumbing across the marketplace UI**

```jsx
// web/src/hooks/model-pricing/useModelPricingData.jsx
const [displayGroups, setDisplayGroups] = useState({});

const loadPricing = async () => {
  setLoading(true);
  const res = await API.get('/api/pricing');
  const {
    success,
    message,
    data,
    vendors,
    group_ratio,
    display_groups,
    supported_endpoint,
  } = res.data;

  if (success) {
    setGroupRatio(group_ratio || {});
    setDisplayGroups(display_groups || {});
    setSelectedGroup('all');
    setFilterGroup('all');
    // existing vendor / endpoint / model formatting stays the same
  } else {
    showError(message);
  }
  setLoading(false);
};

return {
  // ...
  groupRatio,
  displayGroups,
  endpointMap,
  // usableGroup / autoGroups removed from marketplace display state
};
```

```jsx
// web/src/components/table/model-pricing/filter/PricingGroups.jsx
const PricingGroups = ({
  filterGroup,
  setFilterGroup,
  displayGroups = {},
  groupRatio = {},
  models = [],
  loading = false,
  t,
}) => {
  const groups = ['all', ...Object.keys(displayGroups).filter((key) => key !== '')];

  return (
    <SelectableButtonGroup
      title={t('模型分组')}
      items={items}
      activeValue={filterGroup}
      onChange={setFilterGroup}
      loading={loading}
      variant='teal'
      t={t}
    />
  );
};
```

```jsx
// web/src/components/table/model-pricing/layout/PricingSidebar.jsx
<PricingGroups
  filterGroup={filterGroup}
  setFilterGroup={handleGroupClick}
  displayGroups={categoryProps.displayGroups}
  groupRatio={categoryProps.groupRatio}
  models={groupCountModels}
  loading={loading}
  t={t}
/>
```

```jsx
// web/src/components/table/model-pricing/modal/components/FilterModalContent.jsx
<PricingGroups
  filterGroup={filterGroup}
  setFilterGroup={setFilterGroup}
  displayGroups={categoryProps.displayGroups}
  groupRatio={categoryProps.groupRatio}
  models={groupCountModels}
  loading={loading}
  t={t}
/>
```

```jsx
// web/src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx
<ModelPricingTable
  modelData={modelData}
  groupRatio={groupRatio}
  currency={currency}
  siteDisplayType={siteDisplayType}
  tokenUnit={tokenUnit}
  displayPrice={displayPrice}
  showRatio={showRatio}
  displayGroups={displayGroups}
  t={t}
/>
```

```jsx
// web/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx
const ModelPricingTable = ({
  modelData,
  groupRatio,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  showRatio,
  displayGroups,
  t,
}) => {
  const modelEnableGroups = Array.isArray(modelData?.enable_groups)
    ? modelData.enable_groups
    : [];

  const availableGroups = Object.keys(displayGroups || {})
    .filter((g) => g !== '')
    .filter((g) => modelEnableGroups.includes(g));

  // render tableData from availableGroups only
};
```

- [ ] **Step 4: Run the marketplace display source tests to verify they pass**

Run: `cd web && node --test tests/pricing-marketplace-display.test.mjs`

Expected: PASS with the hook, filter, sidebar, modal, and detail assertions all green.

- [ ] **Step 5: Commit the display-group UI changes**

```bash
git add web/src/hooks/model-pricing/useModelPricingData.jsx web/src/components/table/model-pricing/filter/PricingGroups.jsx web/src/components/table/model-pricing/layout/PricingSidebar.jsx web/src/components/table/model-pricing/modal/components/FilterModalContent.jsx web/src/components/table/model-pricing/layout/PricingPage.jsx web/src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx web/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx web/tests/pricing-marketplace-display.test.mjs
git commit -m "refactor: drive marketplace filters from display groups"
```

### Task 5: Clarify Marketplace Display Copy in Settings and Model Admin

**Files:**
- Modify: `web/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx`
- Modify: `web/src/components/table/models/modals/EditModelModal.jsx`
- Modify: `web/src/components/table/models/ModelsColumnDefs.jsx`
- Modify: `web/tests/header-nav-modules.test.mjs`
- Modify: `web/tests/pricing-marketplace-display.test.mjs`
- Test: `web/tests/header-nav-modules.test.mjs`
- Test: `web/tests/pricing-marketplace-display.test.mjs`

- [ ] **Step 1: Extend the failing source tests for the new marketplace copy**

```js
// web/tests/header-nav-modules.test.mjs
test('settings header nav source explains the guest default rule and logged-in all-display rule', async () => {
  const source = await readFile(
    new URL('../src/pages/Setting/Operation/SettingsHeaderNavModules.jsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /t\('控制是否显示模型广场入口'\)/);
  assert.match(source, /t\('关闭后游客按 default 分组浏览模型广场，登录用户可查看全部展示模型'\)/);
});
```

```js
// web/tests/pricing-marketplace-display.test.mjs
test('model admin copy treats status as marketplace display only', async () => {
  const modalSource = await readFile(
    new URL('../src/components/table/models/modals/EditModelModal.jsx', import.meta.url),
    'utf8',
  );
  const columnsSource = await readFile(
    new URL('../src/components/table/models/ModelsColumnDefs.jsx', import.meta.url),
    'utf8',
  );

  assert.match(modalSource, /label=\{t\('模型广场展示'\)\}/);
  assert.match(columnsSource, /title: t\('广场展示'\)/);
});
```

- [ ] **Step 2: Run the copy tests to verify they fail**

Run: `cd web && node --test tests/header-nav-modules.test.mjs tests/pricing-marketplace-display.test.mjs`

Expected: FAIL because the settings copy still describes only the guest-default rule and the model admin UI still says `状态`.

- [ ] **Step 3: Update marketplace settings and model admin terminology**

```jsx
// web/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx
{
  key: 'pricing',
  title: t('模型广场'),
  description: t('控制是否显示模型广场入口'),
  hasSubConfig: true,
}

{t('关闭后游客按 default 分组浏览模型广场，登录用户可查看全部展示模型')}
```

```jsx
// web/src/components/table/models/modals/EditModelModal.jsx
<Form.Switch
  field='status'
  label={t('模型广场展示')}
  extraText={t('关闭后，该模型不会在模型广场展示，但不会影响真实调用与路由')}
  size='large'
/>
```

```jsx
// web/src/components/table/models/ModelsColumnDefs.jsx
{
  title: t('广场展示'),
  dataIndex: 'status',
  render: (val) => (
    <Tag size='small' shape='circle' color={val === 1 ? 'green' : 'grey'}>
      {val === 1 ? t('展示中') : t('已隐藏')}
    </Tag>
  ),
}
```

- [ ] **Step 4: Run the copy tests to verify they pass**

Run: `cd web && node --test tests/header-nav-modules.test.mjs tests/pricing-route-gating.test.mjs tests/pricing-marketplace-display.test.mjs`

Expected: PASS with all updated marketplace source assertions green.

- [ ] **Step 5: Commit the terminology cleanup**

```bash
git add web/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx web/src/components/table/models/modals/EditModelModal.jsx web/src/components/table/models/ModelsColumnDefs.jsx web/tests/header-nav-modules.test.mjs web/tests/pricing-marketplace-display.test.mjs
git commit -m "docs: clarify marketplace display terminology"
```

## Review Checklist

- Task 1 covers Go-side `pricing.enabled` / `requireAuth` parsing so backend gating matches the frontend helper.
- Task 2 covers the core backend behavior from the spec:
  - marketplace disabled returns empty data
  - guests only see `default`
  - logged-in users see all display models
  - hidden `model_meta.status` models remain hidden
- Task 3 covers route and entry gating for `/pricing`.
- Task 4 covers the `display_groups` frontend migration and removal of `usable_group` / `auto` display semantics from the marketplace UI.
- Task 5 covers the wording changes that turn `model_meta.status` into an explicitly display-only control.

- Placeholder scan completed:
  - no `TODO`
  - no `TBD`
  - no "write tests for the above" placeholders

- Type and naming consistency checked:
  - backend field: `display_groups`
  - frontend state/prop: `displayGroups`
  - route prop: `pricingEnabled`
  - admin label: `模型广场展示`
