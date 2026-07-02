package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// generateGhzCommand generates a bash command for ghz based on the current gRPC UI state.
func (a *App) generateGhzCommand() string {
	server := a.grpcServerInput.GetText()
	method := a.grpcMethodInput.GetText()
	metaText := a.grpcRequestMeta.GetText()
	bodyText := a.grpcRequestBody.GetText()

	// Apply environment variables
	server = a.replaceVariables(server)
	metaText = a.replaceVariables(metaText)
	bodyText = a.replaceVariables(bodyText)

	if server == "" || method == "" {
		return "# Error: Server and Method must be filled"
	}

	cmd := []string{"ghz", "--insecure", "-c 1", "-n 1"}

	// Handle metadata
	if strings.TrimSpace(metaText) != "" {
		// Clean up metadata to ensure it's on a single line for bash
		var metaObj map[string]interface{}
		if err := json.Unmarshal([]byte(metaText), &metaObj); err == nil {
			metaBytes, _ := json.Marshal(metaObj)
			cmd = append(cmd, fmt.Sprintf("-m '%s'", string(metaBytes)))
		} else {
			cmd = append(cmd, fmt.Sprintf("-m '%s'", strings.ReplaceAll(metaText, "\n", "")))
		}
	}

	// Handle body
	if strings.TrimSpace(bodyText) != "" {
		// Clean up body to ensure it's on a single line for bash
		var bodyObj map[string]interface{}
		if err := json.Unmarshal([]byte(bodyText), &bodyObj); err == nil {
			bodyBytes, _ := json.Marshal(bodyObj)
			cmd = append(cmd, fmt.Sprintf("-d '%s'", string(bodyBytes)))
		} else {
			cmd = append(cmd, fmt.Sprintf("-d '%s'", strings.ReplaceAll(bodyText, "\n", "")))
		}
	}

	cmd = append(cmd, fmt.Sprintf("--call %s", method))
	cmd = append(cmd, server)

	return strings.Join(cmd, " \\\n  ")
}

// showGenerateScriptModal displays a modal with the generated script.
func (a *App) showGenerateScriptModal() {
	currentPage, _ := a.rootPages.GetFrontPage()
	var script string

	if currentPage == "grpc" {
		script = a.generateGhzCommand()
	} else {
		script = "# HTTP load test script generation is not implemented yet.\n# Please switch to gRPC mode to generate ghz script."
	}

	textArea := tview.NewTextArea().SetText(script, false)
	textArea.SetBorder(true).SetTitle(" Load Test Script (ghz) ")

	copyBtn := tview.NewButton("Copy (Ctrl+C)").SetSelectedFunc(func() {
		a.copyTextAreaToClipboard(textArea)
	})
	closeBtn := tview.NewButton("Close (Esc)").SetSelectedFunc(func() {
		a.rootPages.RemovePage("scriptModal")
	})

	buttons := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(copyBtn, 15, 0, false).
		AddItem(closeBtn, 15, 0, false)

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(textArea, 0, 1, true).
		AddItem(buttons, 1, 0, false)
	content.SetBorder(true)

	modal := a.createModal(content, 80, 20)
	a.rootPages.AddPage("scriptModal", modal, true, true)
	a.app.SetFocus(textArea)
}
