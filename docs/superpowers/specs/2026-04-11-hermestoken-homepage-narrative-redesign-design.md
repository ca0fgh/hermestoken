# Hermestoken Homepage Narrative Redesign Design

> Date: 2026-04-11
> Status: Approved for planning

## Goal

Refactor the homepage content area below the existing top navigation into a minimal, narrative landing page that presents Hermestoken as infrastructure for freely trading LLM token usage rights between Agent and Human participants.

The redesigned homepage must:

- keep the current top navigation unchanged
- use a lighter, editorial visual direction inspired by `https://www.hermestoken.top/`
- remove all extra homepage sections beyond the hero and three narrative cards
- follow the site's existing language switching behavior
- avoid product-entry UI such as Base URL inputs, action buttons, or extra language toggles

## User-approved positioning

### Primary homepage statement

The homepage is not a developer onboarding screen and not an investor memo. It is a concise market-definition page.

The central positioning is:

`面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施`

### Approved supporting definition copy

The hero body copy should be reduced to a two-line definition, not a long explanatory paragraph.

Line 1:

`HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。`

Line 2:

`Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。`

## Audience and homepage role

The page is a public-facing brand and market-definition homepage for users who need to understand what Hermestoken is.

It is not optimized around:

- immediate API onboarding
- console entry
- investor fundraising detail
- explaining every system capability

It is optimized around:

- establishing a differentiated market thesis
- framing Hermestoken as infrastructure, not a model directory
- making the Agent and Human dual-participant concept immediately legible

## Scope

### In scope

- homepage content below the top menu
- hero section redesign
- three-card narrative section directly below the hero
- updated homepage wording to match the approved positioning
- adapting layout and copy for existing i18n language switching
- responsive behavior for desktop and mobile

### Out of scope

- changing the top navigation structure
- changing login/register/language controls outside the homepage content area
- adding CTA buttons
- adding Base URL, docs, console, or pricing entry widgets
- adding long-form sections such as "市场结构" or "核心原则"
- adding investor-specific sections such as business model, milestones, or fundraising content

## Reference direction

The redesign should borrow visual cues from the current live homepage at `https://www.hermestoken.top/`, specifically:

- soft, airy, light-toned background treatment
- oversized, high-impact headline typography
- restrained palette with blue accent usage
- clean card surfaces with generous rounded corners
- minimal but deliberate composition

The redesign should not copy the live site's content blocks literally. It should reuse the mood and rhythm while using the approved Hermestoken narrative.

## Final information architecture

The homepage content area consists of exactly two content blocks below the existing top navigation.

### 1. Hero

Contents:

- a small positioning kicker
- the primary large headline
- the approved two-line definition copy
- a compact row of descriptive tags

The hero ends there.

### 2. Three-card narrative section

Directly below the hero, show three equally important narrative cards.

Approved card themes:

1. `LLM Token 使用权正在被标准化`
2. `AI 能力开始进入资源化阶段`
3. `买方、卖方、Agent 在同一结构里协作`

The page ends after these three cards.

Removed from earlier explorations:

- market-structure mega section
- core-principles section
- any additional lower-page continuation

## Section design details

### Hero section

#### Purpose

Create a strong first-screen definition of Hermestoken as a new market layer.

#### Content structure

- Kicker: short label describing the category
- Headline: the approved main statement
- Definition copy: the approved two-line definition
- Tag row: short descriptors such as `交易层`, `执行层`, `结算层`, `Human 与 Agent`

#### Tone

- declarative
- category-defining
- minimal
- non-promotional

#### Things to avoid

- long explanatory paragraphs
- operational instructions
- signup-style messaging
- language mixing in a single localized render

### Three-card narrative section

#### Purpose

Extend the hero by giving three short, positive narrative lenses on the market without turning the page into a long explainer.

#### Card 1

Title:

`LLM Token 使用权正在被标准化`

Intent:

Explain that token usage rights are moving from informal circulation toward structured, rule-bearing resource objects.

#### Card 2

Title:

`AI 能力开始进入资源化阶段`

Intent:

Explain that AI usage is now behaving more like a managed resource category, requiring clearer execution and settlement boundaries.

#### Card 3

Title:

`买方、卖方、Agent 在同一结构里协作`

Intent:

Explain the shared market structure across participant types, without reverting to API-key language or negative framing.

#### Tone constraints for all cards

- positive and forward-looking
- concise
- structural, not marketing-heavy
- no "problem statement" framing

## Language and i18n behavior

The page must use the project's existing i18n system and follow the site's active language state.

Requirements:

- no extra language switcher added to the homepage
- Chinese renders fully in Chinese
- English renders fully in English
- no mixed-language helper labels within one localized view
- copy length should be adapted per language where needed rather than mechanically literal

## Layout behavior

### Desktop

- hero occupies the visual focus with large centered typography
- three cards appear in a three-column row beneath the hero
- spacing remains generous but not empty

### Mobile

- hero headline wraps cleanly without breaking readability
- definition copy remains concise
- tags wrap naturally
- three cards stack vertically with consistent spacing

## Visual system

### Typography

- strong oversized display style for the hero headline
- refined, lighter body text for the definition
- card titles should feel deliberate and editorial, not dashboard-like

### Color

- light background atmosphere
- dark navy headline color
- restrained blue accent for kicker text and small labels
- avoid heavy dark sections

### Surfaces

- rounded cards
- soft borders
- subtle depth via light shadows
- no visually heavy information panels

## Existing codebase alignment

The redesign should fit the current frontend architecture.

Likely primary implementation touchpoint:

- `web/src/pages/Home/index.jsx`

Likely supporting touchpoints:

- existing locale files under `web/src/i18n/locales/` for new homepage strings

No layout restructuring is required if the top navigation remains unchanged.

The implementation should avoid touching `PageLayout` unless absolutely necessary.

## Behavior requirements

- homepage continues to render inside the current app shell
- current top navigation remains visible and unchanged
- no new interactive dependencies are required
- no custom homepage-content iframe flow is needed for this redesign

## Content requirements summary

### Must say

- Hermestoken defines LLM token usage rights as a standard resource
- Agent and Human participate in the same market
- execution and settlement happen around real usage

### Must not say

- long negative market problem diagnosis
- investor fundraising language
- product onboarding instructions
- mixed-language labels in one render

## Testing expectations for implementation

When implementation begins, testing should cover:

- homepage renders new hero copy
- homepage renders exactly three narrative cards
- removed sections no longer appear
- current top navigation still renders
- language switching changes homepage copy correctly
- mobile layout stacks correctly without overflow

## Risks and mitigations

### Risk: headline and copy become too dense

Mitigation:

- keep hero copy to the approved two-line definition
- resist adding explanatory paragraphs below the hero

### Risk: page feels too empty after removing lower sections

Mitigation:

- rely on stronger hero composition and fuller card copy
- use confident spacing, not filler sections

### Risk: wording becomes too abstract

Mitigation:

- preserve concrete nouns such as `LLM Token 使用权`, `Agent`, `Human`, `执行边界`, and `结算`

## Acceptance criteria

The redesign is successful when:

- the homepage immediately communicates Hermestoken's category in one screen
- the top navigation remains unchanged
- the page ends after the three narrative cards
- "市场结构" and "核心原则" sections are absent
- the approved hero definition copy is used
- the page feels visually aligned with the live site's tone while remaining materially simpler
