package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createGrpcPage builds the layout and all interactive components for the gRPC view.
// createGrpcPage membangun layout dan semua komponen interaktif untuk view gRPC.
func (a *App) createGrpcPage() {
	grpcFlex := tview.NewFlex()

	a.grpcMethodInput = tview.NewInputField().SetLabel("Method: ").SetFieldBackgroundColor(tcell.ColorBlack)
	clearMethodButton := tview.NewButton("X").SetSelectedFunc(func() {
		a.grpcMethodInput.SetText("")
		a.app.SetFocus(a.grpcMethodInput)
	})
	methodInputRow := tview.NewFlex().AddItem(a.grpcMethodInput, 0, 1, true).AddItem(clearMethodButton, 3, 0, false)

	a.grpcMethodList = tview.NewList().ShowSecondaryText(false)
	a.grpcMethodSelector = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(methodInputRow, 1, 1, true).
		AddItem(a.grpcMethodList, 0, 1, false)
	a.grpcMethodSelector.SetBorder(true).SetTitle("Service Method")

	// Live search functionality for gRPC methods.
	// Fungsionalitas live search untuk method gRPC.
	a.grpcMethodInput.SetChangedFunc(func(text string) {
		a.updateGrpcMethodList(text)
	})

	// Handle 'Enter' to select the top search result, and 'Esc' to hide the list.
	// Menangani 'Enter' untuk memilih hasil pencarian teratas, dan 'Esc' untuk menyembunyikan list.
	a.grpcMethodInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if len(a.grpcAvailableMethods) > 0 {
				selectedMethod := a.grpcAvailableMethods[0]
				a.selectGrpcMethod(selectedMethod)
			}
		case tcell.KeyEsc:
			a.hideMethodList()
			a.app.SetFocus(a.grpcRequestBody)
		}
	})

	// Handle navigation between the method input field and the results list.
	// Menangani navigasi antara input field method dan list hasil.
	a.grpcMethodInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown && a.grpcMethodList.GetItemCount() > 0 {
			a.app.SetFocus(a.grpcMethodList)
			return nil
		}
		if event.Key() == tcell.KeyEsc {
			a.hideMethodList()
			return nil
		}
		return event
	})

	// Handle selection of a method from the list.
	// Menangani pemilihan method dari list.
	a.grpcMethodList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if mainText == "[gray]No results found" {
			return
		}
		a.selectGrpcMethod(mainText)
	})

	a.grpcMethodList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.hideMethodList()
			a.app.SetFocus(a.grpcMethodInput)
			return nil
		}
		if event.Key() == tcell.KeyUp && a.grpcMethodList.GetCurrentItem() == 0 {
			a.app.SetFocus(a.grpcMethodInput)
			return nil
		}
		return event
	})

	mainContent := tview.NewFlex().SetDirection(tview.FlexRow)

	topRow := tview.NewFlex()
	serverInputFlex := tview.NewFlex().
		AddItem(a.grpcServerInput, 0, 1, true).
		AddItem(tview.NewButton("Connect").SetSelectedFunc(func() { a.grpcConnect(nil) }), 12, 0, false)
	serverInputFlex.SetBorder(true).SetTitle("Server")

	a.grpcStatusText = tview.NewTextView().SetDynamicColors(true).SetText("[yellow]Not connected")
	a.grpcStatusText.SetBorder(true).SetTitle("Status")
	topRow.AddItem(serverInputFlex, 40, 0, true).AddItem(a.grpcMethodSelector, 0, 1, false)

	bottomRow := tview.NewFlex()
	middlePanel := tview.NewFlex().SetDirection(tview.FlexRow)

	// Metadata section with buttons
	a.grpcRequestMeta = tview.NewTextArea().SetPlaceholder("Metadata (JSON format)...")
	metaBeautifyBtn := tview.NewButton("Beautify").SetSelectedFunc(func() {
		a.beautifyJSON(a.grpcRequestMeta)
	})
	metaClearBtn := tview.NewButton("Clear").SetSelectedFunc(func() {
		a.grpcRequestMeta.SetText("", true)
	})
	metaButtons := tview.NewFlex().AddItem(tview.NewBox(), 0, 1, false).AddItem(metaBeautifyBtn, 10, 0, false).AddItem(metaClearBtn, 7, 0, false)
	metaLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(metaButtons, 1, 0, false).
		AddItem(a.grpcRequestMeta, 0, 1, false)
	metaLayout.SetBorder(true).SetTitle(" Metadata ")

	// Request Body section with buttons
	a.grpcRequestBody = tview.NewTextArea().SetPlaceholder("Select a service method to see the request body template...")

	bodyLayout := tview.NewFlex().SetDirection(tview.FlexRow)
	grpcGenerateBtn := tview.NewButton("Generate").SetSelectedFunc(func() {
		a.generateGrpcBodyTemplate(a.grpcCurrentService, a.grpcRequestBody.GetText())
	})
	grpcBeautifyBtn := tview.NewButton("Beautify").SetSelectedFunc(func() {
		a.beautifyJSON(a.grpcRequestBody)
	})
	grpcClearBtn := tview.NewButton("Clear").SetSelectedFunc(func() {
		a.grpcRequestBody.SetText("", true)
	})
	grpcBodyButtons := tview.NewFlex().AddItem(tview.NewBox(), 0, 1, false).AddItem(grpcGenerateBtn, 10, 0, false).AddItem(grpcBeautifyBtn, 10, 0, false).AddItem(grpcClearBtn, 7, 0, false)
	bodyLayout.AddItem(grpcBodyButtons, 1, 0, false).AddItem(a.grpcRequestBody, 0, 1, false)
	bodyLayout.SetBorder(true).SetTitle(" Request Body ")
	middlePanel.AddItem(metaLayout, 0, 1, false).AddItem(bodyLayout, 0, 2, false)

	a.grpcResponseView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWordWrap(true)
	a.grpcResponseView.SetBorder(true).SetTitle("Response")
	bottomRow.AddItem(middlePanel, 0, 1, false).AddItem(a.grpcResponseView, 0, 1, false)

	mainContent.AddItem(topRow, 3, 0, true).AddItem(a.grpcStatusText, 3, 0, false).AddItem(bottomRow, 0, 1, false)
	grpcFlex.AddItem(mainContent, 0, 1, false)
	a.rootPages.AddPage("grpc", grpcFlex, true, false)
}

// hideMethodList collapses the gRPC method search results list.
// hideMethodList menciutkan list hasil pencarian method gRPC.
func (a *App) hideMethodList() {
	a.grpcMethodSelector.ResizeItem(a.grpcMethodList, 0, 0)
}

// selectGrpcMethod is called when a gRPC method is chosen. It updates the UI,
// caches the previous request body, and generates a new request template. /
// selectGrpcMethod dipanggil saat sebuah method gRPC dipilih. Ini akan memperbarui UI,
// menyimpan cache body dari request sebelumnya, dan membuat template request baru.
func (a *App) selectGrpcMethod(methodName string) {
	if a.grpcCurrentService != "" && a.grpcCurrentService != methodName {
		a.grpcBodyCache[a.grpcCurrentService] = a.grpcRequestBody.GetText()
	}
	a.grpcCurrentService = methodName
	a.grpcMethodInput.SetText(methodName)
	a.grpcStatusText.SetText(fmt.Sprintf("Selected: [green]%s", methodName))

	a.grpcResponseView.SetText("")
	a.grpcRequestMeta.SetText("", true)
	a.grpcRequestBody.SetText("", true)
	a.generateGrpcBodyTemplate(methodName, "")

	a.hideMethodList()
	a.app.SetFocus(a.grpcRequestBody)
}
