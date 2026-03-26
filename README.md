# Command Manager

A terminal-based UI application for managing and viewing command snippets. Built with Go and Bubble Tea.

## Features

- View and organize command snippets with descriptions and tags
- Search/filter commands quickly
- Copy commands to clipboard
- Add, edit, and delete commands
- Clean TUI interface with keyboard navigation

## Installation

```bash
go build -o cmdmanager
```

## Usage

```bash
./cmdmanager
```

### Help

```bash
./cmdmanager --help
```

## Keybindings

| Key | Action |
|-----|--------|
| `c` | Copy command to clipboard |
| `a` | Add new command |
| `e` | Edit command |
| `d` | Delete command |
| `/` | Search commands |
| `?` | Show help |
| `q` | Quit |

## Data Storage

Commands are stored in `home/.cmdmanager/data.json` in the application directory.

**JSON Structure:**
```json
[
  {
    "cmd": "adb devices",
    "desc": "show connected devices"
  }
]
```


## Demo

<img src="demo.gif" height=600/>
