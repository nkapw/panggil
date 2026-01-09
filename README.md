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
    - Support for various authentication methods (Bearer Token, Basic Auth, API Key).
- **gRPC Client**:
    - Connect to gRPC servers and automatically discover services and methods using server reflection.
    - Live search for gRPC methods.
    - Auto-generates JSON request body templates.
- **Collections & History**:
    - Save your requests into organized collections and folders.
    - Visual indicators: 🌐 HTTP/REST, 🔌 gRPC, 📁 Folder.
    - Quickly access and re-run requests from your history.
    - Auto-switch between HTTP/gRPC pages when loading a request.
- **Clipboard Support**: Copy text from any field using `Ctrl+C`.
- **Keyboard-Driven**: Designed for a fast, mouse-free workflow with intuitive keybindings.
- **Cross-Platform**: Works on Linux, macOS, and Windows.

---

## Installation / Instalasi

### Linux & macOS

You can install `panggil` with a single command using `curl`:

```sh
curl -sSL https://raw.githubusercontent.com/nkapw/panggil/main/install.sh | bash
```

**Note:** You might be prompted for your password (`sudo`) to move the binary to `/usr/local/bin`.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/nkapw/panggil/main/install.ps1 | iex
```

Or download and run manually:
```powershell
.\install.ps1
```

### Manual Download

Download a pre-compiled binary for your operating system from the [GitHub Releases](https://github.com/nkapw/panggil/releases) page.

### Building from Source

If you have Go installed:

```sh
git clone https://github.com/nkapw/panggil.git
cd panggil
make build
./panggil
```

Or without make:
```sh
go build -o panggil .
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
| `Ctrl+F`    | Search Collections (Telescope)       |
| `Ctrl+C`    | Copy text from focused field         |
| `Ctrl+Q`    | Quit Application                     |
| `Tab`       | Navigate between fields              |
| `Esc`       | Close modals or popups               |

---

## Make Commands

```sh
make help       # Show all available commands
make build      # Build the binary
make run        # Build and run the application
make install    # Install to /usr/local/bin
make uninstall  # Remove from /usr/local/bin
make release    # Build release binaries for all platforms
make version    # Show version information
make clean      # Remove build artifacts
make test       # Run tests
make lint       # Run linters
```

---

## License

MIT License - See [LICENSE](LICENSE) for details.
