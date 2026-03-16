# Command Viewer - Code Explanation

## Overview

A Terminal UI (TUI) application built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework for managing and viewing commands. Supports searching, adding, editing, and deleting commands with clipboard integration.

---

## Project Structure

```
cmd/
├── main.go           # Main application code
├── data.json         # Command storage file
├── go.mod            # Go module definition
├── go.sum            # Dependency checksums
└── README.md         # User documentation
```

---

## Key Components

### 1. Data Model

```go
type Item struct {
    Cmd  string `json:"cmd"`   // The command itself
    Desc string `json:"desc"`  // Command description
}
```

Each command item has two fields:
- **Cmd**: The actual shell command
- **Desc**: Human-readable description

---

### 2. Application States (Modes)

```go
const (
    modeList          = "list"          // Main list view
    modeAdd           = "add"           // Adding new command
    modeAddField      = "add_field"     // Filling command fields
    modeEditField     = "edit_field"    // Editing existing command
    modeConfirmDelete = "confirm_delete"// Delete confirmation
    modeHelp          = "help"          // Help screen
)
```

---

### 3. Main Model Structure

```go
type model struct {
    list              list.Model      // Bubble Tea list component
    items             []Item          // All stored commands
    textInput         textinput.Model // Search/add/edit input
    mode              string          // Current application mode
    editingIndex      int             // Index of item being edited
    editField         int             // Which field is being edited
    newItem           Item            // Command being added
    message           string          // Status message
    messageTime       time.Time       // When message was shown
    filterText        string          // Current search query
    customFilterEnabled bool          // Is filter active?
    windowWidth       int             // Terminal width for responsive UI
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
    
    data, err := json.MarshalIndent(items, "", "    ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(jsonFilePath, data, 0o644)
}
```

**Storage Location**: `~/.cmdviewer/data.json`

---

## State Machine Flow

### Adding a Command

```
modeList → modeAdd → modeAddField (desc) → modeList
     ↓         ↓            ↓
   press     press       press
    'a'     Enter       Enter
```

### Editing a Command

```
modeList → modeEditField (cmd) → modeEditField (desc) → modeList
     ↓            ↓                    ↓
   press        press                press
    'e'        Enter                Enter
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

    // 2. Check if search input is focused
    if m.textInput.Focused() {
        return m.handleSearchInput(msg)
    }

    // 3. Handle mode-specific input
    switch m.mode {
    case modeAdd:
        return m.handleAddInput(msg)
    case modeEditField:
        return m.handleEditFieldInput(msg)
    // ...
    }

    // 4. Handle list navigation
    return m.handleListKeys(msg)
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
    paddingHeight   = 7                // UI padding for list height
)
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `bubbletea` | TUI framework |
| `bubbles/list` | List component |
| `bubbles/textinput` | Text input component |
| `lipgloss` | Styling and colors |
| `clipboard` | Copy to system clipboard |

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
3. **Tea.Cmd**: Special commands returned from `Update()` for side effects (quit, clipboard, etc.)
4. **Filter Logic**: Search creates a filtered copy of items, not modifying the original list
5. **Persistence**: All changes save to JSON immediately after modification
