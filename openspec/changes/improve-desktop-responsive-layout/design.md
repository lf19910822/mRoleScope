# Design

## Context

The application shell reserves a 236px fixed navigation rail. The trend dashboard currently reserves another 420px for a 368px fixed curve configuration rail until the mobile breakpoint at 760px. This creates a severe compression band: at a 1024px viewport the trend card is approximately 349px wide, and at 900px it is approximately 225px wide. The existing mobile drawer behavior is useful and should be extended to compact desktop rather than shrinking every control.

The current large-screen visual language is intentional: an ocean-breeze dashboard with a persistent configuration rail, a large evidence chart, and a lower event table. The responsive change must preserve that hierarchy while changing only the spatial strategy at narrower widths.

## Goals / Non-Goals

**Goals:**

- Keep the established persistent right rail on wide screens.
- Introduce explicit standard and compact desktop density modes.
- Move the compact desktop configuration rail to an on-demand right drawer so the chart workspace gets the released width.
- Preserve existing curve interactions, trend drilldown, job detail, dark theme, keyboard focus, and mobile behavior.
- Validate geometry and overflow through public UI seams at representative viewport sizes.

**Non-Goals:**

- Do not change trend calculations, event semantics, API contracts, or data loading.
- Do not redesign the existing job list filter semantics or job card information architecture.
- Do not replace the current visual language, introduce a new component library, or solve layout by browser zoom or global font scaling.
- Do not make the configuration rail permanently disappear on compact desktop; it remains available through an explicit control.

## Decisions

### 1. Use four spatial modes, not continuous shrinking

- Wide desktop: `>= 1440px`; keep the current persistent rail and generous spacing.
- Standard desktop: `1180px–1439px`; keep the persistent rail, reduce rail width and local spacing, and preserve a minimum chart workspace.
- Compact desktop: `761px–1179px`; remove the reserved right workspace, show a floating configuration trigger, and open the rail as a bounded right drawer.
- Mobile: `<= 760px`; preserve the current navigation and bottom configuration sheet.

These breakpoints follow the observed failure range rather than device brand names. The critical boundary is the minimum width at which the desktop shell can keep navigation, chart, and a usable persistent rail at the same time.

### 2. Reuse the existing configuration rail DOM and state

The existing `mobileConfigurationOpen` state and rail markup already provide the needed close, list scrolling, footer actions, and focus behavior. The implementation will generalize the state naming only if required, but will keep one configuration surface rather than adding a second editor. CSS will switch its position and visibility at compact desktop.

The compact trigger will be rendered when curve rows exist. It will be positioned within the trend workspace rather than fixed over important chart controls, and its accessible label will identify the action and current curve count.

### 3. Keep the chart and event table as the primary reading surface

At compact desktop, the trend view receives the full content width after the app navigation. The chart height and event table padding receive bounded compact values; text is not globally reduced below the existing readable scale. The event table remains horizontally scrollable if its minimum data width cannot fit.

### 4. Make overlay bounds derive from the viewport

The compact configuration drawer and existing detail/drilldown overlays will use viewport-safe `max-height`, `max-width`, and independent content scrolling. The trend workspace right inset will be zero in compact mode so backdrop and detail bounds do not leave an invisible reserved strip.

### 5. Preserve the wide-screen plan and verify with geometry

Tests will assert observable CSS contracts and browser geometry: rail display mode, chart workspace width, trigger visibility, no horizontal overflow, usable overlay bounds, and mobile preservation. The acceptance matrix will use 1440, 1280, 1100, 900, and 390 CSS pixel widths.

## Risks / Trade-offs

- A compact drawer can cover part of the chart while open. This is intentional and reversible; the chart returns to full width when it closes.
- Moving the rail between fixed and drawer modes may affect transitions and scroll restoration. The implementation will keep the existing panel scroll container and explicitly test breakpoint changes.
- A persistent rail at 1180px may still feel dense for unusually large browser zoom settings. Browser zoom is not reliably distinguishable from CSS viewport width, so the standard safeguards focus on geometry and overflow.
- Existing source-based CSS tests are intentionally supplemented with browser-level geometry checks so the contract is not limited to selector text.

## Open Questions

None. The user approved the four-mode layout strategy and the compact desktop on-demand configuration drawer.
