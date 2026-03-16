package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	dataFileName    = "data.json"
	messageTimeout  = 3 * time.Second
	maxTextInputLen = 0
	textInputWidth  = 80
	paddingHeight   = 8
)

// Colors
const (
	colorDim    = "8"
	colorCyan   = "6"
	colorGreen  = "2"
	colorYellow = "3"
	colorRed    = "1"
	colorWhite  = "7"
	colorBlack  = "0"
	colorPurple = "5"
)

// Modes
const (
	modeList          = "list"
	modeAdd           = "add"
	modeAddField      = "add_field"
	modeEditField     = "edit_field"
	modeConfirmDelete = "confirm_delete"
	modeHelp          = "help"
)

// Fields
const (
	fieldCmd  = 0
	fieldDesc = 1
	fieldTag  = 2
)

var jsonFilePath string

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	jsonFilePath = filepath.Join(homeDir, ".cmdviewer", dataFileName)
	// jsonFilePath = "./" + dataFileName
}

// Item represents a command item
type Item struct {
	Cmd  string `json:"cmd"`
	Desc string `json:"desc"`
	Tag  string `json:"tag"`
}

func (i Item) Title() string       { return i.Cmd }
func (i Item) Description() string { return i.Desc }
func (i Item) FilterValue() string { return i.Cmd }

// itemDelegate handles list item rendering
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim)).Bold(true)
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan)).Bold(true)

	if index == m.Index() {
		bgColor := lipgloss.Color(colorDim)
		idStyle = idStyle.Background(bgColor)
		cmdStyle = cmdStyle.Background(bgColor)
	}

	fmt.Fprintf(w, "  %s %s",
		idStyle.Render(fmt.Sprintf("%d", index+1)),
		cmdStyle.Render(i.Cmd),
	)
}

// model holds the application state
type model struct {
	list                list.Model
	items               []Item
	textInput           textinput.Model
	mode                string
	editingIndex        int
	editField           int
	newItem             Item
	message             string
	messageTime         time.Time
	filterText          string
	customFilterEnabled bool
	windowWidth         int
}

// loadItems loads items from the JSON file
func loadItems() ([]Item, error) {
	data, err := os.ReadFile(jsonFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Item{}, nil
		}
		return nil, err
	}

	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// saveItems saves items to the JSON file
func saveItems(items []Item) error {
	dir := filepath.Dir(jsonFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(items, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(jsonFilePath, data, 0o644)
}

// updateListItems updates the list model with current items
func (m *model) updateListItems() {
	if m.customFilterEnabled && m.filterText != "" {
		// Apply exact contains filtering on cmd, desc, and tag
		var filtered []list.Item
		for _, item := range m.items {
			if strings.Contains(item.Cmd, m.filterText) || strings.Contains(item.Desc, m.filterText) || strings.Contains(item.Tag, m.filterText) {
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

// showMessage sets a temporary message to display
func (m *model) showMessage(msg string) {
	m.message = msg
	m.messageTime = time.Now()
}

// setupTextInput configures the text input component
func setupTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = maxTextInputLen
	ti.Width = textInputWidth
	ti.Prompt = ""
	return ti
}

// newListItem creates a new list model with default settings
func newListItem(items []list.Item) list.Model {
	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return l
}

func initialModel() (model, error) {
	m := model{mode: modeList}

	items, err := loadItems()
	if err != nil {
		return m, err
	}
	m.items = items

	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	m.list = newListItem(listItems)
	m.textInput = setupTextInput()

	return m, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

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

func (m *model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeHelp {
		m.mode = modeList
		return m, nil
	}

	if m.textInput.Focused() {
		return m.handleSearchInput(msg)
	}

	switch m.mode {
	case modeAdd:
		return m.handleAddInput(msg)
	case modeAddField:
		return m.handleAddFieldInput(msg)
	case modeEditField:
		return m.handleEditFieldInput(msg)
	case modeConfirmDelete:
		return m.handleDeleteConfirm(msg)
	}

	return m.handleListKeys(msg)
}

func (m *model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.textInput.Blur()
		m.textInput.SetValue("")
		m.filterText = ""
		m.customFilterEnabled = false
		m.updateListItems()
	case tea.KeyEnter:
		m.textInput.Blur()
	case tea.KeyDown, tea.KeyUp:
		m.textInput.Blur()
		m.list, _ = m.list.Update(msg)
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		m.filterText = m.textInput.Value()
		m.customFilterEnabled = true
		m.updateListItems()
		return m, cmd
	}
	return m, nil
}

func (m *model) handleAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.mode = modeList
		m.textInput.SetValue("")
		return m, nil
	case tea.KeyEnter:
		m.newItem.Cmd = m.textInput.Value()
		m.mode = modeAddField
		m.editField = fieldDesc
		m.textInput.Placeholder = "Enter description..."
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *model) handleAddFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.mode = modeList
		m.textInput.SetValue("")
		return m, nil
	case tea.KeyEnter:
		return m.completeAddField()
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *model) completeAddField() (tea.Model, tea.Cmd) {
	switch m.editField {
	case fieldDesc:
		m.newItem.Desc = m.textInput.Value()
		m.editField = fieldTag
		m.textInput.Placeholder = "Enter tag..."
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink
	case fieldTag:
		m.newItem.Tag = m.textInput.Value()
		m.items = append(m.items, m.newItem)

		if err := saveItems(m.items); err != nil {
			m.showMessage("Error saving: " + err.Error())
		}
		m.updateListItems()
		m.mode = modeList
		m.textInput.SetValue("")
		m.showMessage("Command added!")
		return m, nil
	}
	return m, nil
}

func (m *model) handleEditFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.mode = modeList
		m.textInput.SetValue("")
		return m, nil
	case tea.KeyEnter:
		return m.completeEditField()
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *model) completeEditField() (tea.Model, tea.Cmd) {
	switch m.editField {
	case fieldCmd:
		m.items[m.editingIndex].Cmd = m.textInput.Value()
		m.editField = fieldDesc
		m.textInput.Placeholder = "Edit description..."
		m.textInput.SetValue(m.items[m.editingIndex].Desc)
		m.textInput.CursorEnd()
		m.textInput.Focus()
		return m, textinput.Blink
	case fieldDesc:
		m.items[m.editingIndex].Desc = m.textInput.Value()
		m.editField = fieldTag
		m.textInput.Placeholder = "Edit tag..."
		m.textInput.SetValue(m.items[m.editingIndex].Tag)
		m.textInput.CursorEnd()
		m.textInput.Focus()
		return m, textinput.Blink
	case fieldTag:
		m.items[m.editingIndex].Tag = m.textInput.Value()

		if err := saveItems(m.items); err != nil {
			m.showMessage("Error saving: " + err.Error())
		}
		m.updateListItems()
		m.mode = modeList
		m.textInput.SetValue("")
		m.showMessage("Item updated!")
		return m, nil
	}
	return m, nil
}

func (m *model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		if msg.String() == "y" || msg.String() == "Y" {
			return m.deleteItem()
		}
		if msg.String() == "n" || msg.String() == "N" {
			m.mode = modeList
			return m, nil
		}
	case tea.KeyCtrlC, tea.KeyEsc:
		m.mode = modeList
		return m, nil
	}
	return m, nil
}

func (m *model) deleteItem() (tea.Model, tea.Cmd) {
	deletedCmd := m.items[m.editingIndex].Cmd
	m.items = append(m.items[:m.editingIndex], m.items[m.editingIndex+1:]...)

	if err := saveItems(m.items); err != nil {
		m.showMessage("Error saving: " + err.Error())
	}
	m.updateListItems()
	m.mode = modeList
	m.showMessage("Deleted: " + deletedCmd)
	return m, nil
}

func (m *model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "?" {
		m.mode = modeHelp
		return m, nil
	}

	if msg.String() == "q" || msg.String() == "Q" {
		return m, tea.Quit
	}

	if msg.Type == tea.KeyEsc {
		if m.customFilterEnabled {
			m.customFilterEnabled = false
			m.filterText = ""
			m.textInput.SetValue("")
			m.updateListItems()
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyRunes:
		return m.handleListRunes(msg)
	}

	// Pass through navigation keys (up, down, etc.) to the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) handleListRunes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c", "C":
		return m.copyToClipboard()
	case "/":
		m.textInput.Focus()
		m.textInput.SetValue("")
		m.filterText = ""
		m.customFilterEnabled = false
		m.updateListItems()
		return m, textinput.Blink
	case "a", "A":
		return m.startAdd()
	case "e", "E":
		return m.startEdit()
	case "d", "D":
		return m.startDelete()
	}
	return m, nil
}

func (m *model) copyToClipboard() (tea.Model, tea.Cmd) {
	if len(m.items) == 0 {
		return m, nil
	}

	selectedItem, ok := m.list.SelectedItem().(Item)
	if !ok {
		return m, nil
	}

	if err := clipboard.WriteAll(selectedItem.Cmd); err == nil {
		m.showMessage("Copied to clipboard: " + selectedItem.Cmd)
	} else {
		m.showMessage("Failed to copy: " + err.Error())
	}
	return m, nil
}

func (m *model) startAdd() (tea.Model, tea.Cmd) {
	m.mode = modeAdd
	m.newItem = Item{
		Desc: "New command",
		Tag:  "general",
	}
	m.textInput.Placeholder = "Enter command..."
	m.textInput.SetValue("")
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m *model) startEdit() (tea.Model, tea.Cmd) {
	selectedItem, ok := m.list.SelectedItem().(Item)
	if !ok {
		return m, nil
	}

	m.mode = modeEditField
	m.editingIndex = m.findItemIndex(selectedItem)
	m.editField = fieldCmd
	m.textInput.Placeholder = "Edit command..."
	m.textInput.SetValue(m.items[m.editingIndex].Cmd)
	m.textInput.CursorEnd()
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m *model) startDelete() (tea.Model, tea.Cmd) {
	selectedItem, ok := m.list.SelectedItem().(Item)
	if !ok {
		return m, nil
	}

	m.editingIndex = m.findItemIndex(selectedItem)
	m.mode = modeConfirmDelete
	return m, nil
}

func (m *model) findItemIndex(item Item) int {
	for i, it := range m.items {
		if it.Cmd == item.Cmd && it.Desc == item.Desc {
			return i
		}
	}
	return 0
}

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

func (m model) View() string {
	switch m.mode {
	case modeAdd:
		return m.viewAdd()
	case modeAddField:
		return m.viewAddField()
	case modeEditField:
		return m.viewEditField()
	case modeConfirmDelete:
		return m.viewConfirmDelete()
	case modeHelp:
		return m.viewHelp()
	}

	return m.viewList()
}

func (m *model) viewAdd() string {
	s := "\n  Add New Command\n\n"
	s += "  Command: " + m.textInput.View() + "\n\n"
	s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
	return s
}

func (m *model) viewAddField() string {
	fieldName := map[int]string{
		fieldDesc: "Description",
		fieldTag:  "Tag",
	}[m.editField]

	s := fmt.Sprintf("\n  Add New Command - Step %d\n\n", m.editField)
	s += "  Command: " + m.newItem.Cmd + "\n"
	s += "  " + fieldName + ": " + m.textInput.View() + "\n\n"
	s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
	return s
}

func (m *model) viewEditField() string {
	fieldName := map[int]string{
		fieldCmd:  "Command",
		fieldDesc: "Description",
		fieldTag:  "Tag",
	}[m.editField]

	s := fmt.Sprintf("\n  Edit Item - Step %d\n\n", m.editField+1)
	s += "  Command: " + m.items[m.editingIndex].Cmd + "\n"
	s += "  " + fieldName + ": " + m.textInput.View() + "\n\n"
	s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
	return s
}

func (m *model) viewConfirmDelete() string {
	selectedItem := m.items[m.editingIndex]
	s := fmt.Sprintf("\n  Delete \"%s\"?\n\n", selectedItem.Cmd)
	s += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).Bold(true).Render("Press Y to confirm, N to cancel")
	return s
}

func (m *model) viewHelp() string {
	s := "\n  Help - Command Viewer\n\n"
	s += "  JSON file: " + jsonFilePath + "\n\n"
	s += "  Keybindings:\n"
	s += "    c    Copy command to clipboard\n"
	s += "    a    Add new command\n"
	s += "    e    Edit command\n"
	s += "    d    Delete command\n"
	s += "    /    Search commands\n"
	s += "    ?    Show this help\n"
	s += "    q    Quit\n\n"
	s += "  (Press any key to close)"
	return s
}

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

func (m *model) renderStatusBar() string {
	totalItems := len(m.items)
	filteredItems := len(m.list.VisibleItems())

	statusText := fmt.Sprintf(" Total: %d items", totalItems)
	if filteredItems != totalItems {
		statusText = fmt.Sprintf(" Showing: %d of %d items", filteredItems, totalItems)
	}

	return "\n" + lipgloss.NewStyle().
		Background(lipgloss.Color(colorCyan)).
		Foreground(lipgloss.Color(colorBlack)).
		Bold(true).
		Render(statusText)
}

func (m *model) renderItemDetails() string {
	if len(m.items) == 0 {
		return m.renderEmptyOrMessage()
	}

	selectedItem, ok := m.list.SelectedItem().(Item)
	if !ok {
		return "\n  \n  "
	}

	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow))
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))

	desc := descStyle.Render(selectedItem.Desc)
	tag := tagStyle.Render(selectedItem.Tag)

	if m.hasActiveMessage() {
		msg := messageStyle.Render(m.message)
		return "\n  " + desc + "  │  " + msg + "\n  " + tag + "\n  "
	}

	return "\n  " + desc + "\n  " + tag + "\n  "
}

func (m *model) renderEmptyOrMessage() string {
	if m.hasActiveMessage() {
		return "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Render(m.message) + "\n  "
	}
	return "\n  \n  "
}

func (m *model) hasActiveMessage() bool {
	return m.message != "" && time.Since(m.messageTime) < messageTimeout
}

func printHelp() {
	fmt.Println("Command Viewer - TUI for managing commands")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cmdviewer [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help    Show this help message")
	fmt.Println()
	fmt.Println("JSON file path:")
	fmt.Printf("  %s\n", jsonFilePath)
	fmt.Println()
	fmt.Println("Keybindings:")
	fmt.Println("  c    Copy command to clipboard")
	fmt.Println("  a    Add new command")
	fmt.Println("  e    Edit command")
	fmt.Println("  d    Delete command")
	fmt.Println("  /    Search commands")
	fmt.Println("  q    Quit")
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			printHelp()
			os.Exit(0)
		}
	}

	m, err := initialModel()
	if err != nil {
		fmt.Printf("Error initializing: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
