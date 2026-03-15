package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const jsonFilePath = "cmd.json"

var jsonFileFullPath string

func init() {
	absPath, err := filepath.Abs(jsonFilePath)
	if err != nil {
		jsonFileFullPath = jsonFilePath
	} else {
		jsonFileFullPath = absPath
	}
}

type Item struct {
	Cmd  string `json:"cmd"`
	Desc string `json:"desc"`
	Tag  string `json:"tag"`
}

func (i Item) Title() string       { return i.Cmd }
func (i Item) Description() string { return i.Desc }
func (i Item) FilterValue() string { return fmt.Sprintf("%s %s %s", i.Cmd, i.Desc, i.Tag) }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	if index == m.Index() {
		idStyle = idStyle.Background(lipgloss.Color("8"))
		cmdStyle = cmdStyle.Background(lipgloss.Color("8"))
	}

	fmt.Fprintf(w, "  %s %s",
		idStyle.Render(fmt.Sprintf("%d", index+1)),
		cmdStyle.Render(i.Cmd),
	)
}

type model struct {
	list         list.Model
	items        []Item
	textInput    textinput.Model
	showInput    bool
	mode         string // "list", "add", "add_field", "edit", "edit_field", "confirm_delete"
	editingIndex int
	editField    int // 0=cmd, 1=desc, 2=tag
	newItem      Item
	message      string
	messageTime  time.Time
	filterText   string
}

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

func saveItems(items []Item) error {
	data, err := json.MarshalIndent(items, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonFilePath, data, 0644)
}

func initialModel() (model, error) {
	m := model{mode: "list", message: ""}

	items, err := loadItems()
	if err != nil {
		return m, err
	}
	m.items = items

	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	l := list.New(listItems, itemDelegate{}, 0, 0)
	l.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
	// l.Title = "Command Viewer"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	// l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	m.list = l

	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 50
	ti.Width = 30
	m.textInput = ti

	return m, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == "help" {
			m.mode = "list"
			return m, nil
		}

		if m.showInput {
			switch msg.Type {
			case tea.KeyEsc:
				m.showInput = false
				m.textInput.SetValue("")
				m.list.ResetFilter()
				m.filterText = ""
			case tea.KeyEnter:
				m.showInput = false
				m.filterText = m.textInput.Value()
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				m.list.SetFilterText(m.textInput.Value())
				m.filterText = m.textInput.Value()
				return m, cmd
			}
			return m, nil
		}

		if m.mode == "add" || m.mode == "edit" {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.mode = "list"
				m.textInput.SetValue("")
				return m, nil
			case tea.KeyEnter:
				if m.mode == "add" {
					m.newItem.Cmd = m.textInput.Value()
					m.mode = "add_field"
					m.editField = 1
					m.textInput.Placeholder = "Enter description..."
					m.textInput.SetValue("")
					m.textInput.Focus()
					return m, textinput.Blink
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		if m.mode == "add_field" {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.mode = "list"
				m.textInput.SetValue("")
				return m, nil
			case tea.KeyEnter:
				switch m.editField {
				case 1:
					m.newItem.Desc = m.textInput.Value()
					m.editField = 2
					m.textInput.Placeholder = "Enter tag..."
					m.textInput.SetValue("")
					m.textInput.Focus()
					return m, textinput.Blink
				case 2:
					m.newItem.Tag = m.textInput.Value()
					m.items = append(m.items, m.newItem)

					saveItems(m.items)
					listItems := make([]list.Item, len(m.items))
					for i, item := range m.items {
						listItems[i] = item
					}
					m.list.SetItems(listItems)
					if m.filterText != "" {
						m.list.SetFilterText(m.filterText)
					}
					m.list.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
					m.mode = "list"
					m.textInput.SetValue("")
					m.showMessage("Command added!")
					return m, nil
				}
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		if m.mode == "edit_field" {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.mode = "list"
				m.textInput.SetValue("")
				return m, nil
			case tea.KeyEnter:
				switch m.editField {
				case 0:
					m.items[m.editingIndex].Cmd = m.textInput.Value()
					m.editField = 1
					m.textInput.Placeholder = "Edit description..."
					m.textInput.SetValue(m.items[m.editingIndex].Desc)
					m.textInput.CursorEnd()
					m.textInput.Focus()
					return m, textinput.Blink
				case 1:
					m.items[m.editingIndex].Desc = m.textInput.Value()
					m.editField = 2
					m.textInput.Placeholder = "Edit tag..."
					m.textInput.SetValue(m.items[m.editingIndex].Tag)
					m.textInput.CursorEnd()
					m.textInput.Focus()
					return m, textinput.Blink
				case 2:
					m.items[m.editingIndex].Tag = m.textInput.Value()

					saveItems(m.items)
					listItems := make([]list.Item, len(m.items))
					for i, item := range m.items {
						listItems[i] = item
					}
					m.list.SetItems(listItems)
					if m.filterText != "" {
						m.list.SetFilterText(m.filterText)
					}
					m.list.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
					m.mode = "list"
					m.textInput.SetValue("")
					m.showMessage("Item updated!")
					return m, nil
				}
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		if m.mode == "confirm_delete" {
			switch msg.Type {
			case tea.KeyRunes:
				if msg.String() == "y" || msg.String() == "Y" {
					deletedCmd := m.items[m.editingIndex].Cmd
					m.items = append(m.items[:m.editingIndex], m.items[m.editingIndex+1:]...)

					saveItems(m.items)
					listItems := make([]list.Item, len(m.items))
					for i, item := range m.items {
						listItems[i] = item
					}
					m.list.SetItems(listItems)
					if m.filterText != "" {
						m.list.SetFilterText(m.filterText)
					}
					m.list.Title = "Command Viewer - C: Copy, A: Add, E: Edit, D: Delete, /: Search, ?: Help"
					m.mode = "list"
					m.showMessage("Deleted: " + deletedCmd)
					return m, nil
				}
				if msg.String() == "n" || msg.String() == "N" {
					m.mode = "list"
					return m, nil
				}
			case tea.KeyCtrlC, tea.KeyEsc:
				m.mode = "list"
				return m, nil
			}
		}

		if msg.String() == "?" {
			m.mode = "help"
			return m, nil
		}

		switch msg.Type {
		case tea.KeyRunes:
			if msg.String() == "c" || msg.String() == "C" {
				if len(m.items) > 0 {
					selectedItem, ok := m.list.SelectedItem().(Item)
					if ok {
						copyText := fmt.Sprintf("%s", selectedItem.Cmd)
						if err := clipboard.WriteAll(copyText); err == nil {
							m.showMessage("Copied to clipboard: " + selectedItem.Cmd)
						} else {
							m.showMessage("Failed to copy: " + err.Error())
						}
					}
				}
				return m, nil
			}
			if msg.String() == "/" {
				m.showInput = true
				m.textInput.Focus()
				return m, textinput.Blink
			}
			if msg.String() == "a" || msg.String() == "A" {
				m.mode = "add"
				m.newItem = Item{
					Desc: "New command",
					Tag:  "general",
				}
				m.textInput.Placeholder = "Enter command..."
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, textinput.Blink
			}
			if msg.String() == "e" || msg.String() == "E" {
				if selectedItem, ok := m.list.SelectedItem().(Item); ok {
					m.mode = "edit_field"
					for i, item := range m.items {
						if item.Cmd == selectedItem.Cmd && item.Desc == selectedItem.Desc {
							m.editingIndex = i
							break
						}
					}
					m.editField = 0
					m.textInput.Placeholder = "Edit command..."
					m.textInput.SetValue(m.items[m.editingIndex].Cmd)
					m.textInput.CursorEnd()
					m.textInput.Focus()
					return m, textinput.Blink
				}
			}
			if msg.String() == "d" || msg.String() == "D" {
				if selectedItem, ok := m.list.SelectedItem().(Item); ok {
					for i, item := range m.items {
						if item.Cmd == selectedItem.Cmd && item.Desc == selectedItem.Desc {
							m.editingIndex = i
							break
						}
					}
					m.mode = "confirm_delete"
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		// Account for: title line (1) + status bar (1) + description area (3) + padding/newline (2) = 7
		listHeight := msg.Height - 7
		if listHeight < 1 {
			listHeight = 1
		}
		m.list.SetSize(msg.Width, listHeight)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var s string

	if m.mode == "add" {
		s += "\n  Add New Command\n\n"
		s += "  Command: " + m.textInput.View() + "\n\n"
		s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
		return s
	}

	if m.mode == "add_field" {
		fieldName := ""
		switch m.editField {
		case 1:
			fieldName = "Description"
		case 2:
			fieldName = "Tag"
		}
		s += fmt.Sprintf("\n  Add New Command - Step %d\n\n", m.editField)
		s += "  Command: " + m.newItem.Cmd + "\n"
		s += "  " + fieldName + ": " + m.textInput.View() + "\n\n"
		s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
		return s
	}

	if m.mode == "edit_field" {
		fieldName := ""
		switch m.editField {
		case 0:
			fieldName = "Command"
		case 1:
			fieldName = "Description"
		case 2:
			fieldName = "Tag"
		}
		s += fmt.Sprintf("\n  Edit Item - Step %d\n\n", m.editField+1)
		s += "  Command: " + m.items[m.editingIndex].Cmd + "\n"
		s += "  " + fieldName + ": " + m.textInput.View() + "\n\n"
		s += "  (Enter to continue, Ctrl+C/Esc to cancel)"
		return s
	}

	if m.mode == "confirm_delete" {
		selectedItem := m.items[m.editingIndex]
		s += fmt.Sprintf("\n  Delete \"%s\"?\n\n", selectedItem.Cmd)
		s += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render("Press Y to confirm, N to cancel")
		return s
	}

	if m.mode == "help" {
		s += "\n  Help - Command Viewer\n\n"
		s += "  JSON file: " + jsonFileFullPath + "\n\n"
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

	if m.showInput {
		s += "\n  Search: " + m.textInput.View() + "\n\n"
	}

	s += m.list.View()

	// Status bar with total and shortcuts
	totalItems := len(m.items)
	filteredItems := len(m.list.VisibleItems())
	statusText := fmt.Sprintf(" Total: %d items", totalItems)
	if filteredItems != totalItems {
		statusText = fmt.Sprintf(" Showing: %d of %d items", filteredItems, totalItems)
	}
	statusBar := lipgloss.NewStyle().
		Background(lipgloss.Color("6")).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Render(statusText)
	s += "\n" + statusBar

	s += "\n"
	if len(m.items) > 0 {
		selectedItem, ok := m.list.SelectedItem().(Item)
		if ok {
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
			tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
			messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

			desc := descStyle.Render(selectedItem.Desc)
			tag := tagStyle.Render("Tag: " + selectedItem.Tag)

			if m.message != "" && time.Since(m.messageTime) < 3*time.Second {
				msg := messageStyle.Render(m.message)
				s += "\n  " + desc + "  │  " + msg + "\n  " + tag + "\n  "
			} else {
				s += "\n  " + desc + "\n  " + tag + "\n  "
			}
		}
	} else if m.message != "" && time.Since(m.messageTime) < 3*time.Second {
		s += "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(m.message) + "\n  "
	} else {
		s += "\n  \n  "
	}

	return s
}

func (m *model) showMessage(msg string) {
	m.message = msg
	m.messageTime = time.Now()
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Println("Command Viewer - TUI for managing commands")
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  cmdviewer [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -h, --help    Show this help message")
			fmt.Println()
			fmt.Println("JSON file path:")
			fmt.Printf("  %s\n", jsonFileFullPath)
			fmt.Println()
			fmt.Println("Keybindings:")
			fmt.Println("  c    Copy command to clipboard")
			fmt.Println("  a    Add new command")
			fmt.Println("  e    Edit command")
			fmt.Println("  d    Delete command")
			fmt.Println("  /    Search commands")
			fmt.Println("  q    Quit")
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
