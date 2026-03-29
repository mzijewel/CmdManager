package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	dataFileName   = "data.json"
	configFileName = "config.json"
)

// Global colors
var (
	colorBorder   = tcell.ColorDarkCyan
	colorTitle    = tcell.ColorDarkCyan
	colorSelected = tcell.ColorDarkCyan
	colorNormal   = tcell.ColorWhite
	colorHelp     = tcell.ColorBlue
	colorError    = tcell.ColorRed
	colorSuccess  = tcell.ColorGreen
	colorStatusBg = tcell.ColorPurple
	colorStatusFg = tcell.ColorBlack
	colorWhite    = tcell.ColorWhite
	colorContent  = tcell.ColorBlack
)

var (
	jsonFilePath   string
	configFilePath string
	app            *tview.Application
	mainFlex       *tview.Flex
	commandList    *tview.List
	detailsText    *tview.TextView
	statusBar      *tview.TextView
	items          []Item
	filteredItems  []Item
	selectedIndex  int
	filterText     string
	currentMode    string
	message        string
	messageTimer   *time.Timer
	tempDataPath   string
	addForm        *tview.Flex
	editForm       *tview.Flex
	changePathFlex *tview.Flex
	helpModal      *tview.Flex
	deleteModal    *tview.Flex
	searchInput    *tview.Flex
	mainPages      *tview.Pages
)

// Item represents a command item
type Item struct {
	Cmd  string `json:"cmd"`
	Desc string `json:"desc"`
}

// Config represents the application configuration
type Config struct {
	DataPath string `json:"dataPath"`
}

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	configDir := filepath.Join(homeDir, ".cmdmanager")
	configFilePath = filepath.Join(configDir, configFileName)

	// Default data path
	jsonFilePath = filepath.Join(".", ".data.json")

	// Load config and override data path if set
	config, err := loadConfig()
	if err == nil && config.DataPath != "" {
		jsonFilePath = config.DataPath
	}
}

// loadConfig loads configuration from config file
func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{DataPath: ""}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// saveConfig saves configuration to config file
func saveConfig(config *Config) error {
	dir := filepath.Dir(configFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(config); err != nil {
		return err
	}

	return os.WriteFile(configFilePath, []byte(buf.String()), 0o644)
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

	// Sort items by Cmd in ascending order
	sort.Slice(items, func(i, j int) bool {
		return items[i].Cmd < items[j].Cmd
	})

	return items, nil
}

// saveItems saves items to the JSON file
func saveItems(items []Item) error {
	dir := filepath.Dir(jsonFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(items); err != nil {
		return err
	}

	return os.WriteFile(jsonFilePath, []byte(buf.String()), 0o644)
}

// updateList updates the command list with current items and filter
func updateList() {
	commandList.Clear()
	filteredItems = []Item{}

	for i, item := range items {
		if filterText == "" || strings.Contains(item.Cmd, filterText) || strings.Contains(item.Desc, filterText) {
			display := fmt.Sprintf("[gray][%d] [black]%s", i+1, item.Cmd)
			commandList.AddItem(display, "", 0, nil)
			filteredItems = append(filteredItems, item)
		}
	}

	// Update list title with item count
	total := len(items)
	filtered := len(filteredItems)
	if filtered != total {
		commandList.SetTitle(fmt.Sprintf(" Commands [%d/%d] ", filtered, total))
	} else {
		commandList.SetTitle(fmt.Sprintf(" Commands [%d] ", total))
	}
	commandList.SetTitleColor(colorTitle)

	// Reset selected index if out of bounds
	if selectedIndex >= len(filteredItems) {
		selectedIndex = len(filteredItems) - 1
	}
	if selectedIndex < 0 && len(filteredItems) > 0 {
		selectedIndex = 0
	}

	updateStatusBar()
	updateDetails()
}

// updateStatusBar updates the status bar text
func updateStatusBar() {
	statusBar.SetText(fmt.Sprintf(" [%s]<c> Copy   <a> Add   <e> Edit   <d> Delete   <p> Path   </> Search   <?> Help   <q> Quit ", colorHelp.String()))
}

// updateDetails updates the details panel with selected item
func updateDetails() {
	if len(filteredItems) == 0 {
		detailsText.SetText("")
		return
	}

	if selectedIndex >= 0 && selectedIndex < len(filteredItems) {
		item := filteredItems[selectedIndex]
		detailsText.SetText(fmt.Sprintf("[%s]%s", colorContent.String(), item.Desc))
	} else {
		detailsText.SetText("")
	}
}

// showMessage displays a temporary message in the status bar
func showMessage(msg string) {
	message = msg
	statusBar.SetText(fmt.Sprintf("[%s] %s [-]", colorSuccess.String(), msg))

	if messageTimer != nil {
		messageTimer.Stop()
	}

	messageTimer = time.AfterFunc(3*time.Second, func() {
		app.QueueUpdateDraw(func() {
			updateStatusBar()
		})
	})
}

// createMainFlex creates the main flex layout
func createMainFlex() *tview.Flex {
	commandList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(colorSelected)

	commandList.SetBorder(true).
		SetTitle(" Commands ").
		SetTitleColor(colorTitle)

	commandList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selectedIndex = index
		updateDetails()
	})

	commandList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		copyToClipboard()
	})

	detailsText = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true)

	detailsText.SetBorder(true).
		SetTitle(" Details ").
		SetTitleColor(colorTitle)

	statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// First row: 2 columns (list and details)
	firstRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(commandList, 0, 1, true).
		AddItem(detailsText, 0, 1, false)

	// Second row: status bar (full width)
	secondRow := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBar, 1, 0, false)

	mainFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(firstRow, 0, 1, true).
		AddItem(secondRow, 1, 0, false)

	return mainFlex
}

// createFormOverlay creates a centered overlay for forms
func createFormOverlay(content tview.Primitive, width, height int) *tview.Flex {
	overlay := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(content, height, 0, true).
				AddItem(nil, 0, 1, false), width, 0, true).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)

	return overlay
}

// EntryFormMode represents the mode of the entry form
type EntryFormMode int

const (
	ModeAdd EntryFormMode = iota
	ModeEdit
)

// EntryFormConfig holds configuration for the entry form
type EntryFormConfig struct {
	Mode          EntryFormMode
	Title         string
	PageName      string
	InitialCmd    string
	InitialDesc   string
	OriginalIndex int // For edit mode, the index in items slice
	FilteredIndex int // For edit mode, the index in filteredItems slice
	OnSave        func(cmd, desc string)
	OnCancel      func()
}

// createEntryForm creates a reusable form for adding or editing commands
func createEntryForm(config EntryFormConfig) *tview.Flex {
	cmdInput := tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorReset)

	cmdInput.SetBorder(true).
		SetTitle(fmt.Sprintf(" Command ----- [%s]<Enter> Next ", colorHelp.String())).
		SetTitleColor(colorTitle).
		SetTitleAlign(tview.AlignLeft)

	cmdInput.SetText(config.InitialCmd).SetFieldTextColor(tcell.ColorBlack)

	// Description text view for multiline input
	descText := tview.NewTextArea()
	descText.SetText(config.InitialDesc, true)
	descText.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack))

	descText.SetBorder(true).
		SetTitle(fmt.Sprintf(" Description ----- [%s]<Ctrl+S> Save, <Esc> Cancel ", colorHelp.String())).
		SetTitleColor(colorTitle).
		SetTitleAlign(tview.AlignLeft)

	// Container for input fields
	formContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cmdInput, 3, 0, false).
		AddItem(descText, 0, 1, true)

	formContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			mainPages.RemovePage(config.PageName)
			if config.OnCancel != nil {
				config.OnCancel()
			}
			app.SetFocus(commandList)
			return nil
		}

		// Ctrl+S to save
		if event.Key() == tcell.KeyCtrlS {
			cmd := cmdInput.GetText()
			desc := descText.GetText()

			if cmd == "" {
				return nil
			}

			if config.OnSave != nil {
				config.OnSave(cmd, desc)
			}

			mainPages.RemovePage(config.PageName)
			app.SetFocus(commandList)
			return nil
		}

		// Enter on command field moves to description
		if event.Key() == tcell.KeyEnter {
			if cmdInput.HasFocus() {
				cmd := cmdInput.GetText()
				if cmd != "" {
					app.SetFocus(descText)
				}
				return nil
			}
		}

		// Tab to switch between fields
		if event.Key() == tcell.KeyTab {
			if cmdInput.HasFocus() {
				cmd := cmdInput.GetText()
				if cmd != "" {
					app.SetFocus(descText)
				}
			} else if descText.HasFocus() {
				app.SetFocus(cmdInput)
			}
			return nil
		}

		// Shift+Tab to go back
		if event.Key() == tcell.KeyBacktab {
			if descText.HasFocus() {
				app.SetFocus(cmdInput)
			}
			return nil
		}

		return event
	})

	return createFormOverlay(formContent, 60, 15)
}

// createChangePathOverlay creates the overlay for changing data path
func createChangePathOverlay() *tview.Flex {
	// Path input field with border and title
	pathInput := tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorReset).
		SetText(jsonFilePath).SetFieldTextColor(tcell.ColorBlack)

	pathInput.SetBorder(true).
		SetTitle(fmt.Sprintf(" New Path ----- [%s]<Enter> save) ", colorHelp.String())).
		SetTitleColor(colorTitle).
		SetTitleAlign(tview.AlignLeft)

	pathInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			mainPages.RemovePage("changepath")
			app.SetFocus(commandList)
			return nil
		}

		if event.Key() == tcell.KeyEnter {
			newPath := pathInput.GetText()

			if newPath == "" {
				showMessage("Path change cancelled")
				mainPages.RemovePage("changepath")
				app.SetFocus(commandList)
				return nil
			}

			config := &Config{DataPath: newPath}
			if err := saveConfig(config); err != nil {
				showMessage("Error saving config: " + err.Error())
			} else {
				jsonFilePath = newPath

				// Reload items
				loadedItems, err := loadItems()
				if err != nil {
					showMessage("Error loading from new path: " + err.Error())
				} else {
					items = loadedItems
					selectedIndex = 0
					updateList()
					showMessage("Data path changed to: " + newPath)
				}
			}

			mainPages.RemovePage("changepath")
			app.SetFocus(commandList)
			return nil
		}

		return event
	})

	overlay := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(pathInput, 3, 0, true).
				AddItem(nil, 0, 1, false), 60, 0, true).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)

	return overlay
}

// createHelpModal creates the help modal as a centered overlay
func createHelpModal() *tview.Flex {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(
			"[black]c     Copy command to clipboard\n" +
				"[black]a     Add new command\n" +
				"[black]e     Edit command\n" +
				"[black]d     Delete command\n" +
				"[black]p     Change data path\n" +
				"[black]/     Search/filter commands\n" +
				"[black]?     Show this help\n" +
				"[black]q     Quit",
		)

	text.SetBorder(true).
		SetTitle(" Keybindings ").
		SetTitleColor(colorTitle)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Any key closes the help modal
		mainPages.RemovePage("help")
		app.SetFocus(commandList)
		return nil
	})

	// Centered overlay
	overlay := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(text, 15, 0, true).
				AddItem(nil, 0, 1, false), 60, 0, true).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)

	return overlay
}

// createDeleteModal creates the delete confirmation modal (no bg color, no buttons)
// index is the index in filteredItems
func createDeleteModal(filteredIndex int) *tview.Flex {
	filteredItem := filteredItems[filteredIndex]
	// Find the original index in items slice
	originalIndex := -1
	for i, item := range items {
		if item.Cmd == filteredItem.Cmd && item.Desc == filteredItem.Desc {
			originalIndex = i
			break
		}
	}

	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf(" [%s]%s[-]\"?", colorError.String(), filteredItem.Cmd))

	text.SetBorder(true).
		SetTitle(fmt.Sprintf(" Confirm Delete ----- [%s] <Enter> Yes ", colorHelp.String())).
		SetTitleColor(colorTitle)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			mainPages.RemovePage("delete")
			app.SetFocus(commandList)
			return nil
		}

		if event.Key() == tcell.KeyEnter {
			if originalIndex >= 0 {
				deletedCmd := items[originalIndex].Cmd
				items = append(items[:originalIndex], items[originalIndex+1:]...)

				if err := saveItems(items); err != nil {
					showMessage("Error saving: " + err.Error())
				} else {
					showMessage("Deleted: " + deletedCmd)
					updateList()
					if selectedIndex >= len(filteredItems) {
						selectedIndex = len(filteredItems) - 1
					}
				}
			}

			mainPages.RemovePage("delete")
			app.SetFocus(commandList)
			return nil
		}

		return event
	})

	// Centered overlay
	overlay := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(text, 5, 0, true).
				AddItem(nil, 0, 1, false), 50, 0, true).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)

	return overlay
}

// createSearchInput creates the search input field as an overlay
func createSearchInput() *tview.Flex {
	input := tview.NewInputField().SetFieldBackgroundColor(tcell.ColorReset)

	input.SetBorder(true).
		SetTitle(" Search ").
		SetTitleColor(colorTitle).
		SetTitleAlign(tview.AlignLeft)

	input.SetFieldTextColor(tcell.ColorBlack)

	input.SetChangedFunc(func(text string) {
		filterText = text
		updateList()
	})

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEsc {
			mainPages.RemovePage("search")
			app.SetFocus(commandList)
			return nil
		}

		// Up/Down arrow to close search and focus list
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			mainPages.RemovePage("search")
			app.SetFocus(commandList)
			// Pass the key event to the list for navigation
			app.QueueEvent(event)
			return nil
		}

		return event
	})

	overlay := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(input, 3, 0, true).
				AddItem(nil, 0, 1, false), 60, 0, true).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)

	return overlay
}

// copyToClipboard copies the selected command to clipboard
func copyToClipboard() {
	if len(filteredItems) == 0 {
		return
	}

	if selectedIndex >= 0 && selectedIndex < len(filteredItems) {
		cmd := filteredItems[selectedIndex].Cmd
		if err := clipboard.WriteAll(cmd); err == nil {
			showMessage("Copied: " + cmd)
		} else {
			showMessage("Failed to copy: " + err.Error())
		}
	}
}

// handleGlobalKeys handles global key events
func handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// Don't handle if we're in a modal or form
	if mainPages.HasPage("add") || mainPages.HasPage("edit") ||
		mainPages.HasPage("changepath") || mainPages.HasPage("delete") ||
		mainPages.HasPage("help") || mainPages.HasPage("search") {
		return event
	}

	// Esc to reset filter
	if event.Key() == tcell.KeyEsc {
		if filterText != "" {
			filterText = ""
			selectedIndex = 0
			updateList()
			return nil
		}
	}

	switch event.Rune() {
	case 'q', 'Q':
		app.Stop()
		return nil
	case 'c', 'C':
		copyToClipboard()
		return nil
	case 'a', 'A':
		showAddForm()
		return nil
	case 'e', 'E':
		showEditForm()
		return nil
	case 'd', 'D':
		showDeleteModal()
		return nil
	case 'p', 'P':
		showChangePathForm()
		return nil
	case '/':
		showSearch()
		return nil
	case '?':
		showHelp()
		return nil
	}

	return event
}

// showAddForm shows the add form modal
func showAddForm() {
	addForm = createEntryForm(EntryFormConfig{
		Mode:        ModeAdd,
		Title:       "Add New Command",
		PageName:    "add",
		InitialCmd:  "",
		InitialDesc: "",
		OnSave: func(cmd, desc string) {
			newItem := Item{Cmd: cmd, Desc: desc}
			items = append(items, newItem)

			if err := saveItems(items); err != nil {
				showMessage("Error saving: " + err.Error())
			} else {
				showMessage("Command added!")
				updateList()
			}
		},
		OnCancel: func() {
			// No action needed on cancel for add
		},
	})

	mainPages.AddPage("add", addForm, true, true)
	formContent := addForm.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	cmdInput := formContent.GetItem(0).(*tview.InputField)
	app.SetFocus(cmdInput)
}

// showEditForm shows the edit form modal
func showEditForm() {
	if len(filteredItems) == 0 {
		return
	}

	filteredItem := filteredItems[selectedIndex]
	// Find the original index in items slice
	originalIndex := -1
	for i, item := range items {
		if item.Cmd == filteredItem.Cmd && item.Desc == filteredItem.Desc {
			originalIndex = i
			break
		}
	}

	editForm = createEntryForm(EntryFormConfig{
		Mode:          ModeEdit,
		Title:         "Edit Command",
		PageName:      "edit",
		InitialCmd:    filteredItem.Cmd,
		InitialDesc:   filteredItem.Desc,
		OriginalIndex: originalIndex,
		FilteredIndex: selectedIndex,
		OnSave: func(cmd, desc string) {
			if originalIndex >= 0 {
				items[originalIndex] = Item{Cmd: cmd, Desc: desc}

				if err := saveItems(items); err != nil {
					showMessage("Error saving: " + err.Error())
				} else {
					showMessage("Item updated!")
					updateList()
				}
			}
		},
		OnCancel: func() {
			// No action needed on cancel for edit
		},
	})

	mainPages.AddPage("edit", editForm, true, true)
	formContent := editForm.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	cmdInput := formContent.GetItem(0).(*tview.InputField)
	app.SetFocus(cmdInput)
}

// showDeleteModal shows the delete confirmation modal
func showDeleteModal() {
	if len(filteredItems) == 0 {
		return
	}

	deleteModal = createDeleteModal(selectedIndex)
	mainPages.AddPage("delete", deleteModal, true, true)

	textItem := deleteModal.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.TextView)
	app.SetFocus(textItem)
}

// showChangePathForm shows the change path form
func showChangePathForm() {
	changePathFlex = createChangePathOverlay()
	mainPages.AddPage("changepath", changePathFlex, true, true)

	pathInput := changePathFlex.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.InputField)
	app.SetFocus(pathInput)
}

// showHelp shows the help modal
func showHelp() {
	helpModal = createHelpModal()
	mainPages.AddPage("help", helpModal, true, true)

	textItem := helpModal.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.TextView)
	app.SetFocus(textItem)
}

// showSearch shows the search input
func showSearch() {
	searchInput = createSearchInput()
	input := searchInput.GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.InputField)
	input.SetText(filterText)

	mainPages.AddPage("search", searchInput, true, true)
	app.SetFocus(input)
}

func printHelp() {
	fmt.Println("Command Manager - TUI for managing commands")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cmdmanager [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help    Show this help message")
	fmt.Println()
	fmt.Println("Data file path:")
	fmt.Printf("  %s\n", jsonFilePath)
	fmt.Println()
	fmt.Println("Keybindings:")
	fmt.Println("  c    Copy command to clipboard")
	fmt.Println("  a    Add new command")
	fmt.Println("  e    Edit command")
	fmt.Println("  d    Delete command")
	fmt.Println("  p    Change data path")
	fmt.Println("  /    Search commands")
	fmt.Println("  ?    Show help")
	fmt.Println("  q    Quit")
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			printHelp()
			os.Exit(0)
		}
	}

	// Load items
	var err error
	items, err = loadItems()
	if err != nil {
		fmt.Printf("Error loading items: %v\n", err)
		os.Exit(1)
	}

	// Create application
	app = tview.NewApplication()
	tview.Styles.BorderColor = colorBorder
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorReset

	// Create main layout
	mainFlex = createMainFlex()
	updateList()

	// Create pages container
	mainPages = tview.NewPages().
		AddPage("main", mainFlex, true, true)

	// Set up global key handler
	app.SetInputCapture(handleGlobalKeys)

	// Set root and run
	if err := app.SetRoot(mainPages, true).EnableMouse(true).Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}
