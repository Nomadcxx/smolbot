# SMOLBOT Installer UI Fix - Implementation Plan

## Goal
Rewrite the smolbot installer TUI to match jellywatch's UI/UX patterns exactly.

## Current State Analysis
The smolbot installer currently:
- Uses Catppuccin colors instead of monochrome
- Has broken ASCII header display
- Missing proper full-screen background handling
- No boxed content with rounded borders
- No help text footer
- Incorrect key bindings
- Missing animations (BeamsTextEffect, TypewriterTicker)

## Reference: JellyWatch Patterns

### 1. Theme System
- **Colors**: Monochrome (#1a1a1a bg, #ffffff primary, #cccccc secondary, #666666 muted)
- **Borders**: Rounded corners with #cccccc color
- **Padding**: 1 row vertical, 2 cols horizontal
- **Typography**: Bold headers, italic ticker/help

### 2. Layout Structure
- Terminal size check (80x24 minimum)
- Centered ASCII header with padding calculation
- Full-screen background with WithWhitespaceBackground()
- Boxed content with lipgloss.RoundedBorder()
- Help text footer centered at bottom

### 3. Navigation Patterns
- ↑/↓ or j/k: Navigate options
- Tab/Shift+Tab: Next/previous field
- Enter: Confirm/select
- Esc: Go back
- q: Quit (welcome/complete only)
- Single-letter shortcuts: T=Test, S=Skip, etc.

### 4. Form Input Patterns
- "▸ " prefix for focused field indicator
- Label on separate line, input indented with "    "
- PromptStyle, TextStyle, PlaceholderStyle, Cursor.Style with backgrounds
- Focus management with inputs[] slice and focusedInput index

### 5. Animation System
- **BeamsTextEffect**: Scanning light effect across ASCII header
- **TypewriterTicker**: Types out characters one by one
- Both use monochrome gradients

## Implementation Tasks

### Task 1: Fix Theme System
**Files**: theme.go

**Steps**:
1. Replace Catppuccin colors with monochrome palette
2. Update all style variables (BgBase, Primary, Secondary, etc.)
3. Ensure asciiHeaderLines uses proper escaping for backslashes
4. Add any missing style helpers

**Verification**: Colors match jellywatch exactly

### Task 2: Implement Animations
**Files**: animations.go (new)

**Steps**:
1. Create BeamsTextEffect struct and methods:
   - NewBeamsTextEffect(width, height int, lines []string)
   - Update() - advances beam position
   - Render() - returns styled string with beam effect
   - Uses monochrome gradient: ["#666666", "#999999", "#cccccc", "#ffffff"]

2. Create TypewriterTicker struct and methods:
   - NewTypewriterTicker(messages []string, interval time.Duration)
   - Update() - advances character position
   - Render(width int) - returns current visible text
   - Cycles through messages when complete

3. Add animation messages for smolbot context

**Verification**: Animations render correctly in terminal

### Task 3: Rewrite View System
**Files**: views.go

**Steps**:
1. Rewrite View() function:
   - Add terminal size check (80x24 minimum)
   - Center ASCII header with padding calculation
   - Add ticker display below header
   - Wrap content in boxed container with rounded borders
   - Add help text footer
   - Apply full-screen background with WithWhitespaceBackground()

2. Create getHelpText() function:
   - Returns help text string based on current step
   - Format: "↑/↓: Navigate • Enter: Continue • Esc: Back"

3. Update all step render functions:
   - welcomeView(): Simple centered text
   - prerequisitesView(): Checklist with status indicators
   - providerView(): List with selection indicator
   - configurationView(): Form with proper focus styling
   - channelsView(): Toggle options
   - serviceView(): Form with toggles and inputs
   - installingView(): Progress with task list
   - completeView(): Summary and exit

**Verification**: Layout matches jellywatch screenshot

### Task 4: Fix Input System
**Files**: types.go, main.go

**Steps**:
1. Update types.go:
   - Add beams *BeamsTextEffect field
   - Add ticker *TypewriterTicker field
   - Add tickerIndex int field
   - Add hasGo, hasGit bool fields
   - Ensure all step types defined

2. Rewrite main.go:
   - Initialize animations in newModel()
   - Add tickerCmd() for animation updates
   - Rewrite Update() with proper key filtering:
     * Define universalControlKeys map
     * Define stepControlKeys per step
     * Pass non-control keys to text inputs
   - Add step-specific key handlers:
     * handleWelcomeKeys()
     * handleProviderKeys()
     * handleConfigurationKeys()
     * handleServiceKeys()
     * handleInstallingKeys()
   - Implement focus management:
     * nextInput() / prevInput()
     * Focus/blur text inputs properly

**Verification**: All key bindings work correctly

### Task 5: Fix Form Styling
**Files**: views.go, theme.go

**Steps**:
1. Add input focus styles to theme.go:
   - PromptStyle: Background(BgElevated), Foreground(Secondary)
   - TextStyle: Background(BgElevated), Foreground(FgPrimary)
   - PlaceholderStyle: Background(BgElevated), Foreground(FgMuted)
   - Cursor.Style: Background(Primary), Foreground(BgBase)

2. Update form rendering in views.go:
   - Use "▸ " prefix for focused field
   - Indent inputs with "    "
   - Apply proper styles to textinput.Model
   - Show inline help like "[T] Test connection  [S] Skip"

**Verification**: Forms look identical to jellywatch

### Task 6: Fix Step Flow
**Files**: main.go

**Steps**:
1. Ensure proper step initialization:
   - initInputsForStep() called when entering form steps
   - Clear and recreate inputs[] slice
   - Focus first input

2. Fix step transitions:
   - Welcome → Provider (after checks)
   - Provider → Configuration
   - Configuration → Channels
   - Channels → Service
   - Service → Installing
   - Installing → Complete

3. Add back navigation:
   - Esc returns to previous step
   - Proper state cleanup

**Verification**: Can navigate forward and backward through all steps

### Task 7: Add Status Indicators
**Files**: views.go

**Steps**:
1. Add check/fail/skip marks:
   - checkMark: "[OK]" in SuccessColor
   - failMark: "[FAIL]" in ErrorColor
   - skipMark: "[SKIP]" in WarningColor

2. Update prerequisites view:
   - Show [OK] for detected tools
   - Show [FAIL] for missing required tools
   - Show spinner while detecting

3. Update installing view:
   - Show [OK] for completed tasks
   - Show [FAIL] for failed tasks
   - Show spinner for running tasks

**Verification**: Status indicators render correctly

## Verification Steps

1. **Build**: go build -o install-smolbot ./cmd/installer
2. **Run**: ./install-smolbot
3. **Check**: Terminal size warning if < 80x24
4. **Check**: ASCII header centered with beam animation
5. **Check**: Ticker types out messages
6. **Check**: Boxed content with rounded borders
7. **Check**: Help text footer visible
8. **Check**: All key bindings work (↑/↓, Tab, Enter, Esc, q)
9. **Check**: Can navigate through all steps
10. **Check**: Forms have proper focus styling

## Success Criteria

- [ ] Monochrome color scheme
- [ ] Centered ASCII header with animation
- [ ] Typewriter ticker below header
- [ ] Boxed content with rounded borders
- [ ] Help text footer on all screens
- [ ] Proper key bindings (↑/↓/j/k, Tab, Enter, Esc, q)
- [ ] Form inputs with "▸ " focus indicator
- [ ] Status indicators ([OK], [FAIL], [SKIP])
- [ ] Can complete full installation flow
- [ ] Can navigate back with Esc

## Notes

- Keep smolbot-specific content (providers, channels, etc.)
- Use jellywatch patterns for UI/UX only
- Maintain existing task system and installation logic
- ASCII art should use SMOLBOT.txt content
