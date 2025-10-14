# panggil - A TUI-based HTTP and gRPC Client

**panggil** is a powerful and easy-to-use Terminal User Interface (TUI) application for making HTTP and gRPC requests directly from your terminal. It's designed for developers who want a fast, keyboard-driven workflow without leaving their command-line environment.

---

## 🇮🇩 panggil - Klien HTTP dan gRPC Berbasis TUI

**panggil** adalah aplikasi Terminal User Interface (TUI) yang andal dan mudah digunakan untuk membuat request HTTP dan gRPC langsung dari terminal Anda. Aplikasi ini dirancang untuk developer yang menginginkan alur kerja yang cepat dan berbasis keyboard tanpa harus meninggalkan lingkungan command-line mereka.

---

## Features / Fitur

- **Dual Mode**: Seamlessly switch between HTTP and gRPC modes.
- **HTTP Client**:
    - Supports common methods (GET, POST, PUT, DELETE, etc.).
    - JSON body and header editor.
    - Support for various authentication methods (Bearer Token, Basic Auth).
- **gRPC Client**:
    - Connect to gRPC servers and automatically discover services and methods using server reflection.
    - Live search for gRPC methods.
    - Auto-generates JSON request body templates.
- **Collections & History**:
    - Save your requests into organized collections and folders.
    - Quickly access and re-run requests from your history.
- **Keyboard-Driven**: Designed for a fast, mouse-free workflow with intuitive keybindings.

---

## Installation / Instalasi

### Option 1: One-Liner Script (Recommended for macOS & Linux)

You can install `panggil` with a single command using `curl`. This will automatically download the correct version for your system and install it.

```sh
curl -sSL https://raw.githubusercontent.com/nkapw/panggil/main/install.sh | bash
```
**Note:** You might be prompted for your password (`sudo`) to move the binary to `/usr/local/bin`.

### Manual Download

Alternatively, you can download a pre-compiled binary for your operating system from the GitHub Releases page and place it in a directory within your system's `PATH`.

### Option 2: Building from Source

If you have Go installed, you can build `panggil` from source:

```sh
git clone https://github.com/nkapw/panggil.git
cd panggil
go build -o panggil
./panggil
```
---

## Usage & Keybindings / Penggunaan & Keybindings

The application is designed to be controlled primarily with the keyboard.

| Key(s)      | Action                               |
|-------------|--------------------------------------|
| `F1`        | Show Help                            |
| `F5`        | Send Request                         |
| `F6`        | Clear Form (HTTP Mode)               |
| `F7`        | Focus History Panel                  |
| `F8`        | Save Current Request to Collection   |
| `F9`        | Focus Collections Panel              |
| `F12`       | Switch between HTTP and gRPC modes   |
| `Ctrl+E`    | Toggle Explorer (Collections/History)|
| `Ctrl+C`    | Quit Application                     |
| `Tab`       | Navigate between fields              |
| `Esc`       | Close modals or popups               |

---
