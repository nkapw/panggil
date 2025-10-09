package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Time    time.Time
}

type App struct {
	app          *tview.Application
	pages        *tview.Pages
	methodDrop   *tview.DropDown
	urlInput     *tview.InputField
	authType     *tview.DropDown
	authToken    *tview.InputField
	authUser     *tview.InputField
	authPass     *tview.InputField
	authPanel    *tview.Flex
	headersText  *tview.TextArea
	bodyText     *tview.TextArea
	responseText *tview.TextView
	historyList  *tview.List
	statusText   *tview.TextView
	history      []Request
}

func NewApp() *App {
	return &App{
		app:     tview.NewApplication(),
		history: make([]Request, 0),
	}
}

func (a *App) Init() {
	// Main layout
	mainFlex := tview.NewFlex()

	// Left panel - Request
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow)

	// Method and URL
	topFlex := tview.NewFlex()

	a.methodDrop = tview.NewDropDown().
		SetLabel("Method: ").
		SetOptions([]string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}, nil).
		SetCurrentOption(0)
	a.methodDrop.SetBorder(true)

	a.urlInput = tview.NewInputField().
		SetLabel("URL: ").
		SetText("https://jsonplaceholder.typicode.com/posts/1").
		SetFieldBackgroundColor(tcell.ColorBlack)
	a.urlInput.SetBorder(true).SetTitle("URL")

	topFlex.AddItem(a.methodDrop, 20, 0, false)
	topFlex.AddItem(a.urlInput, 0, 1, false)

	// Authorization Panel
	a.authType = tview.NewDropDown().
		SetLabel("Auth: ").
		SetOptions([]string{"No Auth", "Bearer Token", "Basic Auth", "API Key"}, nil).
		SetCurrentOption(0)

	a.authToken = tview.NewInputField().
		SetLabel("Token: ").
		SetFieldBackgroundColor(tcell.ColorBlack)

	a.authUser = tview.NewInputField().
		SetLabel("Username: ").
		SetFieldBackgroundColor(tcell.ColorBlack)

	a.authPass = tview.NewInputField().
		SetLabel("Password: ").
		SetMaskCharacter('*').
		SetFieldBackgroundColor(tcell.ColorBlack)

	a.authPanel = tview.NewFlex()
	a.authPanel.SetBorder(true).SetTitle("Authorization")
	a.authPanel.AddItem(a.authType, 30, 0, false)

	// Update auth panel on type change
	a.authType.SetSelectedFunc(func(text string, index int) {
		a.updateAuthPanel(index)
	})

	a.updateAuthPanel(0) // Initialize with No Auth

	// Headers
	a.headersText = tview.NewTextArea().
		SetPlaceholder("Headers (JSON format):\n{\n  \"Content-Type\": \"application/json\"\n}")
	a.headersText.SetBorder(true).SetTitle("Headers")
	a.headersText.SetBackgroundColor(tcell.ColorBlack)

	// Body
	a.bodyText = tview.NewTextArea().
		SetPlaceholder("Request Body (for POST, PUT, PATCH)")
	a.bodyText.SetBorder(true).SetTitle("Body")
	a.bodyText.SetBackgroundColor(tcell.ColorBlack)

	// Send button area
	buttonFlex := tview.NewFlex()
	sendBtn := tview.NewButton("Send Request (F5)").SetSelectedFunc(func() {
		a.sendRequest()
	})
	sendBtn.SetBorder(true)

	clearBtn := tview.NewButton("Clear (F6)").SetSelectedFunc(func() {
		a.clearForm()
	})
	clearBtn.SetBorder(true)

	buttonFlex.AddItem(sendBtn, 0, 1, false)
	buttonFlex.AddItem(clearBtn, 0, 1, false)

	leftPanel.AddItem(topFlex, 3, 0, false)
	leftPanel.AddItem(a.authPanel, 3, 0, false)
	leftPanel.AddItem(a.headersText, 0, 1, false)
	leftPanel.AddItem(a.bodyText, 0, 1, false)
	leftPanel.AddItem(buttonFlex, 3, 0, false)

	// Right panel - Response and History
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow)

	// Status
	a.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]Ready to send request")
	a.statusText.SetBorder(true).SetTitle("Status")

	// Response
	a.responseText = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	a.responseText.SetBorder(true).SetTitle("Response")

	// History
	a.historyList = tview.NewList()
	a.historyList.SetBorder(true).SetTitle("History (F7)")
	a.historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index < len(a.history) {
			req := a.history[index]
			a.loadRequest(req)
		}
	})

	rightPanel.AddItem(a.statusText, 3, 0, false)
	rightPanel.AddItem(a.responseText, 0, 2, false)
	rightPanel.AddItem(a.historyList, 0, 1, false)

	// Main layout
	mainFlex.AddItem(leftPanel, 0, 1, true)
	mainFlex.AddItem(rightPanel, 0, 1, false)

	// Pages
	a.pages = tview.NewPages()
	a.pages.AddPage("main", mainFlex, true, true)

	// Help modal
	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetText(`[yellow]Keyboard Shortcuts:[-]

[green]F5[-]     - Send Request
[green]F6[-]     - Clear Form
[green]F7[-]     - Focus History
[green]F1[-]     - Show Help
[green]Ctrl+C[-] - Quit Application
[green]Tab[-]    - Navigate between fields
[green]Esc[-]    - Close Help

[yellow]Usage:[-]
1. Select HTTP method
2. Enter URL
3. Select authentication type (Bearer Token, Basic Auth, etc.)
4. Add headers in JSON format (optional)
5. Add request body for POST/PUT/PATCH (optional)
6. Press F5 or click Send Request
7. View response in the right panel
8. Access previous requests from History

[yellow]Authorization Types:[-]
- [green]No Auth[-]: No authentication
- [green]Bearer Token[-]: JWT or OAuth tokens
- [green]Basic Auth[-]: Username and password
- [green]API Key[-]: Add manually in headers

Press Esc to close this help.`)
	helpText.SetBorder(true).SetTitle("Help - HTTP Client")

	a.pages.AddPage("help", a.createModal(helpText, 60, 20), true, false)

	// Key bindings
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF5:
			a.sendRequest()
			return nil
		case tcell.KeyF6:
			a.clearForm()
			return nil
		case tcell.KeyF7:
			a.app.SetFocus(a.historyList)
			return nil
		case tcell.KeyF1:
			a.pages.ShowPage("help")
			return nil
		case tcell.KeyEsc:
			a.pages.HidePage("help")
			return nil
		}
		return event
	})

	a.app.SetRoot(a.pages, true).EnableMouse(true).SetFocus(a.urlInput)
}

func (a *App) updateAuthPanel(authType int) {
	a.authPanel.Clear()
	a.authPanel.AddItem(a.authType, 30, 0, false)

	switch authType {
	case 0: // No Auth
		noAuthText := tview.NewTextView().
			SetText("No authentication required").
			SetTextColor(tcell.ColorGray)
		a.authPanel.AddItem(noAuthText, 0, 1, false)

	case 1: // Bearer Token
		a.authPanel.AddItem(a.authToken, 0, 1, false)

	case 2: // Basic Auth
		basicFlex := tview.NewFlex()
		basicFlex.AddItem(a.authUser, 0, 1, false)
		basicFlex.AddItem(a.authPass, 0, 1, false)
		a.authPanel.AddItem(basicFlex, 0, 1, false)

	case 3: // API Key
		apiKeyInput := tview.NewInputField().
			SetLabel("API Key: ").
			SetFieldBackgroundColor(tcell.ColorBlack)
		a.authPanel.AddItem(apiKeyInput, 0, 1, false)
	}
}

func (a *App) createModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

func (a *App) sendRequest() {
	_, method := a.methodDrop.GetCurrentOption()
	url := a.urlInput.GetText()

	if url == "" {
		a.statusText.SetText("[red]Error: URL is required")
		return
	}

	a.statusText.SetText("[yellow]Sending request...")

	// Parse headers
	headers := make(map[string]string)
	headersJSON := a.headersText.GetText()
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			a.statusText.SetText(fmt.Sprintf("[red]Error parsing headers: %v", err))
			return
		}
	}

	// Create request
	var bodyReader io.Reader
	bodyText := a.bodyText.GetText()
	if bodyText != "" {
		bodyReader = bytes.NewBufferString(bodyText)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		a.statusText.SetText(fmt.Sprintf("[red]Error creating request: %v", err))
		return
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add authorization
	_, authType := a.authType.GetCurrentOption()
	switch authType {
	case "Bearer Token":
		token := a.authToken.GetText()
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "Basic Auth":
		username := a.authUser.GetText()
		password := a.authPass.GetText()
		if username != "" {
			req.SetBasicAuth(username, password)
		}
	case "API Key":
		// For API Key, user should add it manually in headers
		// as the format varies (X-API-Key, api_key, etc.)
	}

	// Send request
	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		a.statusText.SetText(fmt.Sprintf("[red]Error: %v", err))
		a.responseText.SetText(fmt.Sprintf("[red]Error: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.statusText.SetText(fmt.Sprintf("[red]Error reading response: %v", err))
		return
	}

	// Format response
	statusColor := "[green]"
	if resp.StatusCode >= 400 {
		statusColor = "[red]"
	} else if resp.StatusCode >= 300 {
		statusColor = "[yellow]"
	}

	a.statusText.SetText(fmt.Sprintf("%s%s[-] | Duration: [cyan]%v[-]",
		statusColor, resp.Status, duration))

	// Try to format JSON
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, body, "", "  "); err == nil {
		body = formatted.Bytes()
	}

	responseText := fmt.Sprintf("[yellow]Status:[-] %s%s[-]\n", statusColor, resp.Status)
	responseText += fmt.Sprintf("[yellow]Duration:[-] [cyan]%v[-]\n", duration)
	responseText += fmt.Sprintf("[yellow]Content-Length:[-] %d bytes\n\n", len(body))
	responseText += "[yellow]Headers:[-]\n"

	for k, v := range resp.Header {
		responseText += fmt.Sprintf("  [cyan]%s:[-] %s\n", k, strings.Join(v, ", "))
	}

	responseText += fmt.Sprintf("\n[yellow]Body:[-]\n%s", string(body))

	a.responseText.SetText(responseText)
	a.responseText.ScrollToBeginning()

	// Add to history
	historyReq := Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    bodyText,
		Time:    time.Now(),
	}
	a.history = append([]Request{historyReq}, a.history...)

	a.historyList.InsertItem(0,
		fmt.Sprintf("%s %s", method, url),
		historyReq.Time.Format("15:04:05"),
		0, nil)
}

func (a *App) clearForm() {
	a.urlInput.SetText("")
	a.headersText.SetText("", true)
	a.bodyText.SetText("", true)
	a.responseText.SetText("")
	a.statusText.SetText("[yellow]Ready to send request")
	a.methodDrop.SetCurrentOption(0)
	a.authType.SetCurrentOption(0)
	a.authToken.SetText("")
	a.authUser.SetText("")
	a.authPass.SetText("")
	a.updateAuthPanel(0)
}

func (a *App) loadRequest(req Request) {
	// Find method index
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for i, m := range methods {
		if m == req.Method {
			a.methodDrop.SetCurrentOption(i)
			break
		}
	}

	a.urlInput.SetText(req.URL)

	if len(req.Headers) > 0 {
		headersJSON, _ := json.MarshalIndent(req.Headers, "", "  ")
		a.headersText.SetText(string(headersJSON), true)
	}

	if req.Body != "" {
		a.bodyText.SetText(req.Body, true)
	}

	a.app.SetFocus(a.urlInput)
}

func (a *App) Run() error {
	return a.app.Run()
}

func main() {
	app := NewApp()
	app.Init()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
