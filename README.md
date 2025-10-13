# panggil - A TUI-based HTTP and gRPC Client

**panggil** is a powerful and easy-to-use Terminal User Interface (TUI) application for making HTTP and gRPC requests directly from your terminal. It's designed for developers who want a fast, keyboard-driven workflow without leaving their command-line environment.

---

## 🇮🇩 panggil - Klien HTTP dan gRPC Berbasis TUI

**panggil** adalah aplikasi Terminal User Interface (TUI) yang andal dan mudah digunakan untuk membuat request HTTP dan gRPC langsung dari terminal Anda. Aplikasi ini dirancang untuk developer yang menginginkan alur kerja yang cepat dan berbasis keyboard tanpa harus meninggalkan lingkungan command-line mereka.

---

## ✨ Features / Fitur

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

## 🚀 Installation / Instalasi

You need to have Go (version 1.18 or newer) installed on your system.

1.  **Clone the repository:**
    *(Clone repositori ini:)*
    ```sh
    git clone https://github.com/nkapw/panggil.git
    cd panggil
    ```

2.  **Build the application:**
    *(Build aplikasi:)*
    ```sh
    go build
    ```

3.  **Run the application:**
    *(Jalankan aplikasi:)*
    ```sh
    ./panggil
    ```

You can also move the binary to a directory in your `PATH` (e.g., `/usr/local/bin`) for easy access from anywhere.
*(Anda juga bisa memindahkan file binary ke direktori yang ada di `PATH` Anda (misalnya, `/usr/local/bin`) agar mudah diakses dari mana saja.)*

---

## ⌨️ Usage & Keybindings / Penggunaan & Keybindings

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

