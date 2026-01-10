package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createHttpPage builds the layout and all interactive components for the HTTP view.
// createHttpPage membangun layout dan semua komponen interaktif untuk view HTTP.
func (a *App) createHttpPage() *tview.Flex {
	httpFlex := tview.NewFlex()

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow)

	topFlex := tview.NewFlex()

	a.methodDrop = tview.NewDropDown().
		SetLabel("Method: ").
		SetOptions([]string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}, nil).
		SetCurrentOption(0)
	a.methodDrop.SetBorder(true)

	a.urlInput = tview.NewInputField().
		SetLabel("URL: ").
		SetText("").
		SetFieldBackgroundColor(tcell.ColorBlack)
	a.urlInput.SetBorder(true).SetTitle("URL")

	topFlex.AddItem(a.methodDrop, 20, 0, false)
	topFlex.AddItem(a.urlInput, 0, 1, false)

	a.createAuthPanel()

	// Headers section with buttons
	a.headersText = tview.NewTextArea().
		SetPlaceholder("Headers (JSON format):\n{\n  \"Content-Type\": \"application/json\"\n}")
	a.headersText.SetBackgroundColor(tcell.ColorBlack)
	httpHeadersBeautifyBtn := tview.NewButton("Beautify").SetSelectedFunc(func() {
		a.beautifyJSON(a.headersText)
	})
	httpHeadersClearBtn := tview.NewButton("Clear").SetSelectedFunc(func() {
		a.headersText.SetText("", true)
	})
	httpHeadersButtons := tview.NewFlex().AddItem(tview.NewBox(), 0, 1, false).AddItem(httpHeadersBeautifyBtn, 10, 0, false).AddItem(httpHeadersClearBtn, 7, 0, false)
	httpHeadersLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(httpHeadersButtons, 1, 0, false).
		AddItem(a.headersText, 0, 1, false)
	httpHeadersLayout.SetBorder(true).SetTitle(" Headers ")

	// Body section with buttons
	a.bodyText = tview.NewTextArea().
		SetPlaceholder("Request Body (for POST, PUT, PATCH)")
	a.bodyText.SetBackgroundColor(tcell.ColorBlack)
	httpBodyLayout := tview.NewFlex().SetDirection(tview.FlexRow)
	httpBeautifyBtn := tview.NewButton("Beautify").SetSelectedFunc(func() {
		a.beautifyJSON(a.bodyText)
	})
	httpClearBtn := tview.NewButton("Clear").SetSelectedFunc(func() {
		a.bodyText.SetText("", true)
	})
	httpBodyButtons := tview.NewFlex().AddItem(tview.NewBox(), 0, 1, false).AddItem(httpBeautifyBtn, 10, 0, false).AddItem(httpClearBtn, 7, 0, false)
	httpBodyLayout.AddItem(httpBodyButtons, 1, 0, false).AddItem(a.bodyText, 0, 1, false)
	httpBodyLayout.SetBorder(true).SetTitle(" Body ")

	leftPanel.AddItem(topFlex, 3, 0, false)
	leftPanel.AddItem(a.authPanel, 3, 0, false)
	leftPanel.AddItem(httpHeadersLayout, 0, 1, false)
	leftPanel.AddItem(httpBodyLayout, 0, 1, false)

	a.httpRightPanel = tview.NewFlex().SetDirection(tview.FlexRow)

	a.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]Ready to send request")
	a.statusText.SetBorder(true).SetTitle("Status")

	a.responseText = tview.NewTextArea()
	a.responseText.SetPlaceholder("Response will appear here...")

	// Response panel with Copy button
	httpCopyResponseBtn := tview.NewButton("Copy").SetSelectedFunc(func() {
		a.copyTextAreaToClipboard(a.responseText)
	})
	httpResponseButtons := tview.NewFlex().AddItem(tview.NewBox(), 0, 1, false).AddItem(httpCopyResponseBtn, 6, 0, false)
	httpResponseLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(httpResponseButtons, 1, 0, false).
		AddItem(a.responseText, 0, 1, false)
	httpResponseLayout.SetBorder(true).SetTitle(" Response ")

	a.httpRightPanel.AddItem(a.statusText, 3, 0, false).AddItem(httpResponseLayout, 0, 1, false)
	httpFlex.AddItem(leftPanel, 0, 1, true).AddItem(a.httpRightPanel, 0, 1, false)

	return httpFlex
}

// createAuthPanel builds the authorization selection and input fields panel.
// createAuthPanel membangun panel untuk pemilihan otorisasi dan input fields-nya.
func (a *App) createAuthPanel() {
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

	a.authType.SetSelectedFunc(func(text string, index int) {
		a.updateAuthPanel(index)
	})

	a.updateAuthPanel(0) // Initialize with No Auth
}
