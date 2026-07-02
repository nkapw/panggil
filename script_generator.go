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

	server = a.replaceVariables(server)
	metaText = a.replaceVariables(metaText)
	bodyText = a.replaceVariables(bodyText)

	if server == "" || method == "" {
		return "# Error: Server and Method must be filled"
	}

	cmd := []string{"ghz", "--insecure", "-c 1", "-n 1"}

	if strings.TrimSpace(metaText) != "" {
		var metaObj map[string]interface{}
		if err := json.Unmarshal([]byte(metaText), &metaObj); err == nil {
			metaBytes, _ := json.Marshal(metaObj)
			cmd = append(cmd, fmt.Sprintf("-m '%s'", string(metaBytes)))
		} else {
			cmd = append(cmd, fmt.Sprintf("-m '%s'", strings.ReplaceAll(metaText, "\n", "")))
		}
	}

	if strings.TrimSpace(bodyText) != "" {
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

// generateGrpcurlCommand generates a bash command for grpcurl based on the current gRPC UI state.
func (a *App) generateGrpcurlCommand() string {
	server := a.grpcServerInput.GetText()
	method := a.grpcMethodInput.GetText()
	metaText := a.grpcRequestMeta.GetText()
	bodyText := a.grpcRequestBody.GetText()

	server = a.replaceVariables(server)
	metaText = a.replaceVariables(metaText)
	bodyText = a.replaceVariables(bodyText)

	if server == "" || method == "" {
		return "# Error: Server and Method must be filled"
	}

	cmd := []string{"grpcurl", "-plaintext"}

	if strings.TrimSpace(metaText) != "" {
		var metaObj map[string]interface{}
		if err := json.Unmarshal([]byte(metaText), &metaObj); err == nil {
			for k, v := range metaObj {
				cmd = append(cmd, fmt.Sprintf("-H '%s: %v'", k, v))
			}
		}
	}

	if strings.TrimSpace(bodyText) != "" {
		var bodyObj map[string]interface{}
		if err := json.Unmarshal([]byte(bodyText), &bodyObj); err == nil {
			bodyBytes, _ := json.Marshal(bodyObj)
			cmd = append(cmd, fmt.Sprintf("-d '%s'", string(bodyBytes)))
		} else {
			cmd = append(cmd, fmt.Sprintf("-d '%s'", strings.ReplaceAll(bodyText, "\n", "")))
		}
	}

	cmd = append(cmd, server)
	cmd = append(cmd, method)

	return strings.Join(cmd, " \\\n  ")
}

// generateCurlCommand generates a bash command for curl based on the current HTTP UI state.
func (a *App) generateCurlCommand() string {
	_, method := a.methodDrop.GetCurrentOption()
	url := a.urlInput.GetText()
	headersText := a.headersText.GetText()
	bodyText := a.bodyText.GetText()

	url = a.replaceVariables(url)
	headersText = a.replaceVariables(headersText)
	bodyText = a.replaceVariables(bodyText)

	if url == "" {
		return "# Error: URL must be filled"
	}

	cmd := []string{"curl", "-X " + method}

	// Handle Headers
	if strings.TrimSpace(headersText) != "" {
		var headersObj map[string]string
		if err := json.Unmarshal([]byte(headersText), &headersObj); err == nil {
			for k, v := range headersObj {
				cmd = append(cmd, fmt.Sprintf("-H '%s: %s'", k, v))
			}
		}
	}

	// Handle Auth
	_, authType := a.authType.GetCurrentOption()
	switch authType {
	case "Bearer Token":
		token := a.replaceVariables(a.authToken.GetText())
		if token != "" {
			cmd = append(cmd, fmt.Sprintf("-H 'Authorization: Bearer %s'", token))
		}
	case "Basic Auth":
		user := a.replaceVariables(a.authUser.GetText())
		pass := a.replaceVariables(a.authPass.GetText())
		if user != "" {
			cmd = append(cmd, fmt.Sprintf("-u '%s:%s'", user, pass))
		}
	}

	// Handle Body
	if strings.TrimSpace(bodyText) != "" && method != "GET" && method != "HEAD" {
		var bodyObj interface{}
		if err := json.Unmarshal([]byte(bodyText), &bodyObj); err == nil {
			bodyBytes, _ := json.Marshal(bodyObj)
			cmd = append(cmd, fmt.Sprintf("-d '%s'", string(bodyBytes)))
		} else {
			cmd = append(cmd, fmt.Sprintf("-d '%s'", strings.ReplaceAll(bodyText, "\n", "")))
		}
	}

	cmd = append(cmd, fmt.Sprintf("'%s'", url))

	return strings.Join(cmd, " \\\n  ")
}

// showGenerateScriptModal displays a modal with the generated script.
func (a *App) showGenerateScriptModal() {
	currentPage, _ := a.rootPages.GetFrontPage()

	var options []string
	var generators map[string]func() string

	if currentPage == "grpc" {
		options = []string{"ghz", "grpcurl"}
		generators = map[string]func() string{
			"ghz":     a.generateGhzCommand,
			"grpcurl": a.generateGrpcurlCommand,
		}
	} else {
		options = []string{"curl"}
		generators = map[string]func() string{
			"curl": a.generateCurlCommand,
		}
	}

	toolDrop := tview.NewDropDown().
		SetLabel("Tool: ").
		SetOptions(options, nil).
		SetCurrentOption(0)

	textArea := tview.NewTextArea()
	textArea.SetBorder(true).SetTitle(" Generated Script ")

	updateScript := func() {
		_, opt := toolDrop.GetCurrentOption()
		if gen, ok := generators[opt]; ok {
			textArea.SetText(gen(), false)
		}
	}

	toolDrop.SetSelectedFunc(func(text string, index int) {
		updateScript()
	})
	updateScript()

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
		AddItem(toolDrop, 3, 0, false).
		AddItem(textArea, 0, 1, true).
		AddItem(buttons, 1, 0, false)
	content.SetBorder(true).SetTitle(" Script Generator ")

	modal := a.createModal(content, 80, 20)
	a.rootPages.AddPage("scriptModal", modal, true, true)
	a.app.SetFocus(toolDrop)
}
