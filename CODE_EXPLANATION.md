# Command Viewer - Code Explanation

## Overview

A Terminal UI (TUI) application built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework for managing and viewing commands. Supports searching, adding, editing, and deleting commands with clipboard integration.

---

## Project Structure

```
cmd/
├── main.go              # Main application code
├── data.json            # Command storage file (example)
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
├── CODE_EXPLANATION.md  # This file - technical documentation
└── README.md            # User documentation
```

---

## Key Components

### 1. Data Model

```go
type Item struct {
    Cmd  string `json:"cmd"`   // The command itself
    Desc string `json:"desc"`  // Command description
}

// Item implements list.Item interface
func (i Item) Title() string       { return i.Cmd }
func (i Item) Description() string { return i.Desc }
func (i Item) FilterValue() string { return i.Cmd }
```

Each command item has two fields:
- **Cmd**: The actual shell command
- **Desc**: Human-readable description

### 2. Custom List Item Delegate

```go
type itemDelegate struct{}

func (d itemDelegate) Height() int  { return 1 }
func (d itemDelegate) Spacing() int { return 0 }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
    // Renders each item with numbered index and styled command text
    // Selected item has different background color
}
```

The custom delegate controls how list items are displayed:
- Shows item number in dim color
- Shows command in cyan
- Highlights selected item with background color

---

### 3. Application States (Modes)

```go
const (
    modeList          = "list"          // Main list view
    modeAdd           = "add"           // Adding new command (first field)
    modeAddField      = "add_field"     // Adding description field
    modeEdit          = "edit"          // Editing command field
    modeEditField     = "edit_field"    // Editing description field
    modeConfirmDelete = "confirm_delete"// Delete confirmation
    modeHelp          = "help"          // Help screen
)
```

---

### 4. Main Model Structure

```go
type model struct {
    list                list.Model      // Bubble Tea list component
    items               []Item          // All stored commands
    textInput           textinput.Model // Search/add/edit input
    mode                string          // Current application mode
    editingIndex        int             // Index of item being edited
    editField           int             // Which field is being edited (0=cmd, 1=desc)
    newItem             Item            // Command being added
    editingItem         Item            // Copy of item being edited
    message             string          // Status message
    messageTime         time.Time       // When message was shown
    filterText          string          // Current search query
    customFilterEnabled bool          // Is filter active?
    windowWidth         int             // Terminal width for responsive UI
}
```

---

## Search/Filter Functionality

### How It Works

```go
func (m *model) updateListItems() {
    if m.customFilterEnabled && m.filterText != "" {
        // Filter commands by cmd and desc
        var filtered []list.Item
        for _, item := range m.items {
            if strings.Contains(item.Cmd, m.filterText) ||
               strings.Contains(item.Desc, m.filterText) {
                filtered = append(filtered, item)
            }
        }
        m.list.SetItems(filtered)
    } else {
        // Show all items
        listItems := make([]list.Item, len(m.items))
        for i, item := range m.items {
            listItems[i] = item
        }
        m.list.SetItems(listItems)
    }
}
```

### Search Behavior

| Action | Result |
|--------|--------|
| Press `/` | Open search input |
| Type text | Filter updates in real-time |
| Press `↑` or `↓` | Exit search, keep filter, navigate list |
| Press `Enter` | Exit search, keep filter applied |
| Press `Esc` (in search) | Clear filter, show all items |
| Press `Esc` (filtered list) | Clear filter, return to normal |

### Key Features

1. **Substring Matching**: Uses `strings.Contains()` for exact substring matching (not fuzzy)
2. **Multi-Field Search**: Searches in command and description
3. **Dynamic Width**: Input box expands to fit terminal width
4. **No Character Limit**: Can type long search queries

---

## Key Bindings

| Key | Action |
|-----|--------|
| `c` | Copy command to clipboard |
| `a` | Add new command |
| `e` | Edit selected command |
| `d` | Delete selected command |
| `/` | Search/filter commands |
| `?` | Show help screen |
| `q` | Quit application |
| `Esc` | Clear filter / Cancel operation |
| `↑` `↓` | Navigate list |
| `y`/`n` | Confirm/cancel deletion |

---

## Data Persistence

### Load Commands

```go
func loadItems() ([]Item, error) {
    data, err := os.ReadFile(jsonFilePath)
    if err != nil {
        if os.IsNotExist(err) {
            return []Item{}, nil  // Empty list if file doesn't exist
        }
        return nil, err
    }
    
    var items []Item
    json.Unmarshal(data, &items)
    return items, nil
}
```

### Save Commands

```go
func saveItems(items []Item) error {
    dir := filepath.Dir(jsonFilePath)
    os.MkdirAll(dir, 0o755)  // Create directory if needed

    var buf strings.Builder
    encoder := json.NewEncoder(&buf)
    encoder.SetEscapeHTML(false)  // Prevent HTML escaping
    encoder.SetIndent("", "    ")
    encoder.Encode(items)

    return os.WriteFile(jsonFilePath, []byte(buf.String()), 0o644)
}
```

**Storage Location**: `~/.cmdviewer/data.json`

---

## State Machine Flow

### Adding a Command

```
modeList → modeAdd → modeAddField → modeList
     ↓         ↓          ↓
   press     press     press
    'a'     Enter      Enter
           (cmd)      (desc)
```

### Editing a Command

```
modeList → modeEdit → modeEditField → modeList
     ↓         ↓           ↓
   press     press      press
    'e'     Enter       Enter
           (cmd)      (desc)
```

### Deleting a Command

```
modeList → modeConfirmDelete → modeList
     ↓            ↓
   press       'y' or 'n'
    'd'
```

---

## UI Rendering

### View Components

```go
func (m model) View() string {
    switch m.mode {
    case modeAdd:
        return m.viewAdd()
    case modeAddField:
        return m.viewAddField()
    case modeEdit:
        return m.viewEdit()
    case modeEditField:
        return m.viewEditField()
    case modeConfirmDelete:
        return m.viewConfirmDelete()
    case modeHelp:
        return m.viewHelp()
    }
    return m.viewList()
}
```

### List View Implementation

```go
func (m *model) viewList() string {
    // Update list title based on search state
    if m.textInput.Focused() {
        m.list.Title = "Search: " + m.textInput.View()
    } else {
        m.list.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
    }

    var s string
    s += m.list.View()
    s += m.renderStatusBar()
    s += "\n"
    s += m.renderItemDetails()

    return s
}
```

**Key Design:**
- Title dynamically switches between keybindings and search input
- Uses `m.textInput.Focused()` to determine search state
- No extra vertical space when search is inactive

### List View Layout

**Normal View (search inactive):**
```
┌─────────────────────────────────────────────────────────┐
│  Command Viewer - C: Copy, A: Add, E: Edit, D: Delete  │  ← Title with keybindings
├─────────────────────────────────────────────────────────┤
│  1  git status                                          │
│  2  git commit -m "message"                             │  ← Command list
│  3  git push                                            │
├─────────────────────────────────────────────────────────┤
│  Total: 3 items                                         │  ← Status bar
├─────────────────────────────────────────────────────────┤
│  Shows current branch status                            │  ← Description
└─────────────────────────────────────────────────────────┘
```

**Search Active (press `/`):**
```
┌─────────────────────────────────────────────────────────┐
│  Search: git status                                     │  ← Search input replaces title
├─────────────────────────────────────────────────────────┤
│  1  git status                                          │
│  2  git commit -m "message"                             │  ← Filtered command list
├─────────────────────────────────────────────────────────┤
│  Showing: 2 of 3 items                                  │  ← Status bar shows filtered count
├─────────────────────────────────────────────────────────┤
│  Shows current branch status                            │  ← Description
└─────────────────────────────────────────────────────────┘
```

**Key Changes:**
- Search input appears **in the title area** when `/` is pressed (no extra height)
- Title dynamically switches between keybindings and search input
- No vertical space consumed when search is inactive
- Description displayed at bottom for selected command

---

## Event Handling

### Update Loop

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeyPress(msg)
    case tea.WindowSizeMsg:
        return m.handleWindowSize(msg)
    }
    
    var cmd tea.Cmd
    m.list, cmd = m.list.Update(msg)
    return m, cmd
}
```

### Key Press Handler

```go
func (m *model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // 1. Check if in help mode
    if m.mode == modeHelp {
        m.mode = modeList
        return m, nil
    }

    // 2. Handle mode-specific input
    switch m.mode {
    case modeAdd:
        return m.handleAddInput(msg)
    case modeAddField:
        return m.handleAddFieldInput(msg)
    case modeEdit:
        return m.handleEditInput(msg)
    case modeEditField:
        return m.handleEditFieldInput(msg)
    case modeConfirmDelete:
        return m.handleDeleteConfirm(msg)
    }

    // 3. Check if search input is focused
    if m.textInput.Focused() {
        return m.handleSearchInput(msg)
    }

    // 4. Handle list navigation
    return m.handleListKeys(msg)
}
```

### Window Size Handler

```go
func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
    m.windowWidth = msg.Width
    listHeight := msg.Height - paddingHeight
    if listHeight < 7 {
        listHeight = 7
    }
    m.list.SetSize(msg.Width, listHeight)
    m.textInput.Width = msg.Width - 20
    return m, nil
}
```

---

## Configuration Constants

```go
const (
    dataFileName    = "data.json"      // Storage file name
    messageTimeout  = 3 * time.Second  // Status message duration
    maxTextInputLen = 0                // 0 = no limit
    textInputWidth  = 80               // Initial input width
    paddingHeight   = 8                // UI padding for list height
)

// Color constants for lipgloss styling
const (
    colorDim    = "8"   // Dim gray
    colorCyan   = "6"   // Cyan
    colorGreen  = "2"   // Green
    colorYellow = "3"   // Yellow
    colorRed    = "1"   // Red
    colorWhite  = "7"   // White
    colorBlack  = "0"   // Black
    colorPurple = "5"   // Purple
)

// Field constants for edit operations
const (
    fieldCmd  = 0  // Command field
    fieldDesc = 1  // Description field
)
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/bubbles/list` | List component |
| `github.com/charmbracelet/bubbles/textinput` | Text input component |
| `github.com/charmbracelet/lipgloss` | Styling and colors |
| `github.com/atotto/clipboard` | Copy to system clipboard |

---

## Building and Running

```bash
# Build
go build -o cmdviewer

# Run
./cmdviewer

# Show help
./cmdviewer --help
```

---

## Example data.json

```json
[
    {
        "cmd": "git status",
        "desc": "Show current branch and changes"
    },
    {
        "cmd": "docker ps",
        "desc": "List running containers"
    },
    {
        "cmd": "kubectl get pods",
        "desc": "List Kubernetes pods"
    }
]
```

---

## Tips for Understanding the Code

1. **Follow the State**: Track `m.mode` to understand which code path executes
2. **Update → View**: Bubble Tea calls `Update()` for logic, then `View()` for rendering
3. **Tea.Cmd**: Special commands returned from `Update()` for side effects (quit, clipboard, textinput.Blink)
4. **Filter Logic**: Search creates a filtered copy of items, not modifying the original list
5. **Persistence**: All changes save to JSON immediately after modification
6. **State Copies**: When editing, a copy (`editingItem`) is made to allow cancellation
7. **Helper Methods**: Complex operations are split into helper methods:
   - `handle*()` methods process input for each mode
   - `view*()` methods render each view
   - `start*()` methods initialize add/edit/delete operations
   - `complete*()` methods finalize add/edit operations

---

## Helper Functions

### Data Loading/Saving

| Function | Purpose |
|----------|---------|
| `loadItems()` | Load commands from JSON file |
| `saveItems()` | Save commands to JSON file |
| `updateListItems()` | Refresh list with filtered/unfiltered items |

### UI Helpers

| Function | Purpose |
|----------|---------|
| `setupTextInput()` | Configure text input component |
| `newListItem()` | Create list with default settings |
| `showMessage()` | Display temporary status message |
| `hasActiveMessage()` | Check if message is still visible |
| `renderStatusBar()` | Render status bar with item count |
| `renderItemDetails()` | Render selected item description |

### State Transition Helpers

| Function | Purpose |
|----------|---------|
| `startAdd()` | Initialize add command mode |
| `startEdit()` | Initialize edit command mode |
| `startDelete()` | Initialize delete confirmation mode |
| `completeAddField()` | Finalize adding new command |
| `completeEditField()` | Finalize editing command |
| `deleteItem()` | Remove item and save |
| `copyToClipboard()` | Copy selected command to clipboard |
| `findItemIndex()` | Find index of item in items slice |
