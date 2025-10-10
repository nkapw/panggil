package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
)

type Request struct {
	Name    string
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Time    time.Time
}

type CollectionNode struct {
	Name     string            `json:"name"`
	IsFolder bool              `json:"is_folder"`
	Request  *Request          `json:"request,omitempty"`
	Children []*CollectionNode `json:"children,omitempty"`
	Expanded bool              `json:"-"` // Dikecualikan dari JSON
}

type App struct {
	app             *tview.Application
	rootPages       *tview.Pages // Halaman utama untuk beralih antara HTTP dan gRPC
	rightPages      *tview.Pages
	pages           *tview.Pages
	methodDrop      *tview.DropDown
	urlInput        *tview.InputField
	authType        *tview.DropDown
	authToken       *tview.InputField
	authUser        *tview.InputField
	authPass        *tview.InputField
	authPanel       *tview.Flex
	headersText     *tview.TextArea
	bodyText        *tview.TextArea
	responseText    *tview.TextView
	historyList     *tview.List
	collectionsTree *tview.TreeView
	statusText      *tview.TextView
	history         []Request
	collectionsRoot *CollectionNode

	// gRPC components
	grpcPages          *tview.Pages
	grpcServerInput    *tview.InputField
	grpcServiceTree    *tview.TreeView
	grpcRequestMeta    *tview.TextArea
	grpcRequestBody    *tview.TextArea
	grpcResponseView   *tview.TextView
	grpcStatusText     *tview.TextView
	grpcReflectClient  *grpcreflect.Client
	grpcStub           grpcdynamic.Stub
	grpcConn           *grpc.ClientConn
	grpcCurrentService string
}

const collectionsFile = "collections.json"

func (a *App) saveCollections() {
	data, err := json.MarshalIndent(a.collectionsRoot, "", "  ")
	if err != nil {
		// Mungkin bisa menampilkan error di status bar
		return
	}

	_ = os.WriteFile(collectionsFile, data, 0644)
}

func (a *App) loadCollections() {
	data, err := os.ReadFile(collectionsFile)
	if err != nil {
		return // File mungkin belum ada
	}
	_ = json.Unmarshal(data, &a.collectionsRoot)
}

func NewApp() *App {
	app := &App{
		app:     tview.NewApplication().EnableMouse(true),
		history: make([]Request, 0),
		collectionsRoot: &CollectionNode{
			Name:     "Collections",
			IsFolder: true,
			Expanded: true,
		},
	}
	// Muat koleksi yang ada, jika tidak ada, root akan tetap ada
	app.loadCollections()
	return app
}

func (a *App) Init() {
	// Main layout
	mainFlex := tview.NewFlex()

	// Root pages untuk switch HTTP/gRPC
	a.rootPages = tview.NewPages()
	a.app.SetRoot(a.rootPages, true)
	a.rootPages.AddPage("http", mainFlex, true, true)

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
		SetText("").
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

	saveBtn := tview.NewButton("Save (F8)").SetSelectedFunc(func() {
		a.showSaveRequestModal()
	})
	saveBtn.SetBorder(true)

	buttonFlex.AddItem(saveBtn, 0, 1, false)
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
	a.historyList = tview.NewList().ShowSecondaryText(false)
	a.historyList.SetBorder(true).SetTitle("History (F7)")
	a.historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index < len(a.history) {
			req := a.history[index]
			a.loadRequest(req)
		}
	})

	// Collections
	a.collectionsTree = tview.NewTreeView()
	a.collectionsTree.SetBorder(true).SetTitle("Collections (F9, 'n' for new folder)")
	a.populateCollectionsTree()
	a.collectionsTree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			return
		}

		collectionNode, ok := reference.(*CollectionNode)
		if !ok {
			return
		}

		if collectionNode.IsFolder {
			// Expand atau collapse folder
			node.SetExpanded(!node.IsExpanded())
			collectionNode.Expanded = node.IsExpanded()
		} else {
			// Muat permintaan
			if collectionNode.Request != nil {
				a.loadRequest(*collectionNode.Request)
			}
		}
	})
	a.collectionsTree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// 'n' for "new folder"
		if event.Key() == tcell.KeyRune && event.Rune() == 'n' {
			a.showCreateFolderModal()
			return nil
		}
		return event
	})

	// Right panel pages (History/Collections)
	a.rightPages = tview.NewPages()
	a.rightPages.AddPage("history", a.historyList, true, true)
	a.rightPages.AddPage("collections", a.collectionsTree, true, false)

	rightPanel.AddItem(a.statusText, 3, 0, false)
	rightPanel.AddItem(a.responseText, 0, 2, false)
	rightPanel.AddItem(a.rightPages, 0, 1, false)

	// Main layout
	mainFlex.AddItem(leftPanel, 0, 1, true)
	mainFlex.AddItem(rightPanel, 0, 1, false)

	// Pages
	a.pages = tview.NewPages()
	a.pages.AddPage("main", mainFlex, true, true)

	// Buat halaman gRPC
	a.createGrpcPage()

	// Tambahkan header/switcher di atas
	switcher := a.createModeSwitcher()
	rootFlex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(switcher, 1, 0, false).AddItem(a.rootPages, 0, 1, true)

	// Help modal
	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetText(`[yellow]Keyboard Shortcuts:[-]

[green]F5[-]     - Send Request
[green]F6[-]     - Clear Form
[green]F7[-]     - Focus History
[green]F8[-]     - Save Request to Collection
[green]F9[-]     - Focus Collections
[green]F12[-]    - Switch HTTP/gRPC Mode
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
7. Press F8 to save the request to your collection
7. View response in the right panel
8. Access previous requests from History

[yellow]Resizing Panels with Mouse:[-]
1. Move your mouse cursor over the border between two panels.
2. Click and drag the border to adjust the size.
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
			a.rightPages.SwitchToPage("history")
			return nil
		case tcell.KeyF8:
			a.showSaveRequestModal()
			return nil
		case tcell.KeyF9:
			a.app.SetFocus(a.collectionsTree)
			a.rightPages.SwitchToPage("collections")
			return nil
		case tcell.KeyF1:
			a.pages.ShowPage("help")
			return nil
		case tcell.KeyF12:
			a.switchMode()
			return nil
		case tcell.KeyEsc:
			a.pages.HidePage("help")
			return nil
		}
		return event
	})

	a.app.SetRoot(rootFlex, true)
	a.app.SetFocus(a.urlInput)
}

func (a *App) createModeSwitcher() tview.Primitive {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("HTTP Client [yellow](F12 to switch)[-]")

	a.rootPages.SetChangedFunc(func() {
		page, _ := a.rootPages.GetFrontPage()
		if page == "http" {
			textView.SetText("HTTP Client [yellow](F12 to switch)[-]")
		} else {
			textView.SetText("gRPC Client [yellow](F12 to switch)[-]")
		}
	})
	return textView
}

func (a *App) switchMode() {
	currentPage, _ := a.rootPages.GetFrontPage()
	if currentPage == "http" {
		a.rootPages.SwitchToPage("grpc")
		a.app.SetFocus(a.grpcServerInput)
	} else {
		a.rootPages.SwitchToPage("http")
		a.app.SetFocus(a.urlInput)
	}
}

func (a *App) createGrpcPage() {
	// Layout utama gRPC
	grpcFlex := tview.NewFlex()

	// Panel Kiri: Server & Services
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow)
	serverInputFlex := tview.NewFlex()
	a.grpcServerInput = tview.NewInputField().SetLabel("Server: ").SetText("localhost:8081")
	connectBtn := tview.NewButton("Connect").SetSelectedFunc(a.grpcConnect)
	serverInputFlex.AddItem(a.grpcServerInput, 0, 1, true).AddItem(connectBtn, 10, 0, false)

	a.grpcServiceTree = tview.NewTreeView()
	a.grpcServiceTree.SetBorder(true).SetTitle("Services")
	a.grpcServiceTree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		// Simpan service/method yang dipilih
		if serviceName, ok := ref.(string); ok {
			a.grpcCurrentService = serviceName
			a.grpcStatusText.SetText(fmt.Sprintf("Selected: [green]%s", serviceName))
		}
	})

	leftPanel.AddItem(serverInputFlex, 1, 0, true)
	leftPanel.AddItem(a.grpcServiceTree, 0, 1, false)

	// Panel Tengah: Request
	middlePanel := tview.NewFlex().SetDirection(tview.FlexRow)
	a.grpcRequestMeta = tview.NewTextArea().SetPlaceholder("Metadata (JSON format)...")
	a.grpcRequestMeta.SetBorder(true).SetTitle("Metadata")
	a.grpcRequestBody = tview.NewTextArea().SetPlaceholder("Request Body (JSON format)...")
	a.grpcRequestBody.SetBorder(true).SetTitle("Request Body")
	sendGrpcBtn := tview.NewButton("Send (F5)").SetSelectedFunc(a.sendGrpcRequest)
	middlePanel.AddItem(a.grpcRequestMeta, 0, 1, false).AddItem(a.grpcRequestBody, 0, 2, false).AddItem(sendGrpcBtn, 1, 0, false)

	// Panel Kanan: Response
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow)
	a.grpcStatusText = tview.NewTextView().SetDynamicColors(true).SetText("[yellow]Not connected")
	a.grpcStatusText.SetBorder(true).SetTitle("Status")
	a.grpcResponseView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWordWrap(true)
	a.grpcResponseView.SetBorder(true).SetTitle("Response")
	rightPanel.AddItem(a.grpcStatusText, 3, 0, false).AddItem(a.grpcResponseView, 0, 1, false)

	grpcFlex.AddItem(leftPanel, 30, 0, true).AddItem(middlePanel, 0, 1, false).AddItem(rightPanel, 0, 1, false)
	a.rootPages.AddPage("grpc", grpcFlex, true, false)
}

func (a *App) grpcConnect() {
	serverAddr := a.grpcServerInput.GetText()
	if serverAddr == "" {
		a.grpcStatusText.SetText("[red]Server address is required")
		return
	}

	// Update status di UI dan jalankan koneksi di goroutine agar tidak hang
	a.grpcStatusText.SetText(fmt.Sprintf("[yellow]Connecting to %s...", serverAddr))

	go func() {
		// Bersihkan koneksi lama jika ada
		if a.grpcConn != nil {
			a.grpcConn.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Failed to connect: %v", err))
			})
			return
		}

		a.grpcConn = conn
		a.grpcStub = grpcdynamic.NewStub(conn)

		// Gunakan refleksi
		reflectionClient := grpc_reflection_v1alpha.NewServerReflectionClient(a.grpcConn)
		a.grpcReflectClient = grpcreflect.NewClient(ctx, reflectionClient)
		services, err := a.grpcReflectClient.ListServices()
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Failed to list services: %v", err))
			})
			return
		}

		// Kirim pembaruan UI kembali ke thread utama
		a.app.QueueUpdateDraw(func() {
			root := tview.NewTreeNode("Services").SetColor(tcell.ColorRed)
			a.grpcServiceTree.SetRoot(root).SetCurrentNode(root)
			for _, srv := range services {
				if srv == "grpc.reflection.v1alpha.ServerReflection" {
					continue
				}
				srvNode := tview.NewTreeNode(srv).SetColor(tcell.ColorGreen)
				root.AddChild(srvNode)

				sd, err := a.grpcReflectClient.ResolveService(srv)
				if err != nil {
					continue
				}
				for _, md := range sd.GetMethods() {
					methodName := fmt.Sprintf("%s/%s", srv, md.GetName())
					methodNode := tview.NewTreeNode(md.GetName()).SetReference(methodName).SetSelectable(true)
					srvNode.AddChild(methodNode)
				}
			}

			a.grpcStatusText.SetText(fmt.Sprintf("[green]Connected to %s. Found %d services.", serverAddr, len(services)-1))
		})
	}()
}

func (a *App) sendGrpcRequest() {
	if a.grpcConn == nil {
		a.grpcStatusText.SetText("[red]Not connected to any server.")
		return
	}
	if a.grpcCurrentService == "" {
		a.grpcStatusText.SetText("[red]No service/method selected.")
		return
	}

	a.grpcStatusText.SetText(fmt.Sprintf("[yellow]Sending request to %s...", a.grpcCurrentService))
	a.grpcResponseView.SetText("")

	go func() {
		// 1. Parse service and method name
		parts := strings.SplitN(a.grpcCurrentService, "/", 2)
		if len(parts) != 2 {
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Invalid service/method format: %s", a.grpcCurrentService))
			})
			return
		}
		serviceName, methodName := parts[0], parts[1]

		// 2. Resolve service and then find method descriptor
		sd, err := a.grpcReflectClient.ResolveService(serviceName)
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Error resolving service '%s': %v", serviceName, err))
			})
			return
		}
		md := sd.FindMethodByName(methodName)
		if md == nil {
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Method '%s' not found in service '%s'", methodName, serviceName))
			})
			return
		}

		// 3. Create dynamic message from JSON body
		req := md.GetInputType()
		dynMsg := dynamic.NewMessage(req)
		bodyText := a.grpcRequestBody.GetText()
		if bodyText != "" {
			if err := dynMsg.UnmarshalJSON([]byte(bodyText)); err != nil {
				a.app.QueueUpdateDraw(func() {
					a.grpcStatusText.SetText(fmt.Sprintf("[red]Error parsing request body JSON: %v", err))
				})
				return
			}
		}

		// 4. Prepare context with metadata
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		metaText := a.grpcRequestMeta.GetText()
		if metaText != "" {
			var metaMap map[string]string
			if err := json.Unmarshal([]byte(metaText), &metaMap); err != nil {
				a.app.QueueUpdateDraw(func() {
					a.grpcStatusText.SetText(fmt.Sprintf("[red]Error parsing metadata JSON: %v", err))
				})
				return
			}
			ctx = metadata.NewOutgoingContext(ctx, metadata.New(metaMap))
		}

		// 5. Invoke RPC
		start := time.Now()
		resp, err := a.grpcStub.InvokeRpc(ctx, md, dynMsg)
		duration := time.Since(start)

		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]RPC Error: %v", err))
				a.grpcResponseView.SetText(fmt.Sprintf("[red]%v", err))
				return
			}

			// 6. Format and display response
			dynResp, ok := resp.(*dynamic.Message)
			if !ok {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Internal Error: Unexpected response type %T", resp))
				a.grpcResponseView.SetText(fmt.Sprintf("Could not format response: %v", resp))
				return
			}
			respJSON, err := dynResp.MarshalJSONIndent()
			if err != nil {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Error formatting response JSON: %v", err))
				a.grpcResponseView.SetText(fmt.Sprintf("Could not format response JSON: %v", err))
				return
			}
			a.grpcStatusText.SetText(fmt.Sprintf("[green]Success![-] | Duration: [cyan]%v[-]", duration))
			a.grpcResponseView.SetText(string(respJSON)).ScrollToBeginning()
		})
	}()
}

func (a *App) showSaveRequestModal() {
	_, method := a.methodDrop.GetCurrentOption()
	url := a.urlInput.GetText()

	nameInput := tview.NewInputField().SetLabel("Request Name").SetText(fmt.Sprintf("%s %s", method, url)).SetFieldWidth(60)

	form := tview.NewForm().
		AddFormItem(nameInput).
		AddButton("Save", func() {
			name := nameInput.GetText()
			a.saveCurrentRequest(name)
			a.pages.RemovePage("saveModal")
		}).
		AddButton("Cancel", func() {
			a.pages.RemovePage("saveModal")
		})

	form.SetBorder(true).SetTitle("Save Request to Collection")
	modal := a.createModal(form, 80, 7)
	a.app.SetFocus(nameInput)
	a.pages.AddPage("saveModal", modal, true, true)
}

func (a *App) showCreateFolderModal() {
	nameInput := tview.NewInputField().SetLabel("Folder Name").SetFieldWidth(50)

	form := tview.NewForm().
		AddFormItem(nameInput).
		AddButton("Create", func() {
			folderName := nameInput.GetText()
			if folderName != "" {
				a.createCollectionFolder(folderName)
			}
			a.pages.RemovePage("createFolderModal")
		}).
		AddButton("Cancel", func() {
			a.pages.RemovePage("createFolderModal")
		})

	form.SetBorder(true).SetTitle("Create New Folder")
	modal := a.createModal(form, 70, 7)
	a.app.SetFocus(nameInput)
	a.pages.AddPage("createFolderModal", modal, true, true)
}

func (a *App) createCollectionFolder(name string) {
	selectedTreeNode := a.collectionsTree.GetCurrentNode()
	if selectedTreeNode == nil {
		return
	}

	// Find the parent CollectionNode where the new folder will be added
	var parentData *CollectionNode
	ref := selectedTreeNode.GetReference()
	if ref != nil {
		selectedData, ok := ref.(*CollectionNode)
		if ok {
			if selectedData.IsFolder {
				// If a folder is selected, the new folder is a child of it
				parentData = selectedData
			} else {
				// If a request is selected, find its parent in the data structure
				parentData = a.findParentNode(a.collectionsRoot, selectedData)
			}
		}
	}

	if parentData == nil {
		parentData = a.collectionsRoot // Default to root if no parent is found
	}

	newFolder := &CollectionNode{Name: name, IsFolder: true}
	parentData.Children = append(parentData.Children, newFolder)
	a.populateCollectionsTree()
	a.saveCollections()
}

func (a *App) saveCurrentRequest(name string) {
	_, method := a.methodDrop.GetCurrentOption()
	url := a.urlInput.GetText()
	headersText := a.headersText.GetText()
	body := a.bodyText.GetText()

	headers := make(map[string]string)
	if headersText != "" {
		_ = json.Unmarshal([]byte(headersText), &headers)
	}

	requestData := &Request{
		Name:    name,
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
		Time:    time.Now(),
	}

	newNode := &CollectionNode{
		Name:     name,
		IsFolder: false,
		Request:  requestData,
	}

	// Add to the currently selected folder or root
	selectedTreeNode := a.collectionsTree.GetCurrentNode()
	var parentData *CollectionNode
	if selectedTreeNode != nil {
		ref := selectedTreeNode.GetReference()
		if ref != nil {
			selectedData, ok := ref.(*CollectionNode)
			if ok {
				if selectedData.IsFolder {
					parentData = selectedData
				} else {
					parentData = a.findParentNode(a.collectionsRoot, selectedData)
				}
			}
		}
	}

	if parentData == nil {
		parentData = a.collectionsRoot // Default to root
	}

	parentData.Children = append(parentData.Children, newNode)

	a.populateCollectionsTree()
	a.saveCollections()
}

func (a *App) populateCollectionsTree() {
	rootNode := tview.NewTreeNode(a.collectionsRoot.Name).SetReference(a.collectionsRoot)
	a.collectionsTree.SetRoot(rootNode).SetCurrentNode(rootNode)
	a.addTreeNodes(rootNode, a.collectionsRoot.Children)
	rootNode.SetExpanded(a.collectionsRoot.Expanded)
}

func (a *App) addTreeNodes(parent *tview.TreeNode, children []*CollectionNode) {
	for _, childData := range children {
		icon := "📄" // Ikon untuk request
		if childData.IsFolder {
			icon = "📁" // Ikon untuk folder
		}
		node := tview.NewTreeNode(fmt.Sprintf("%s %s", icon, childData.Name)).
			SetReference(childData).
			SetSelectable(true)

		if childData.IsFolder {
			a.addTreeNodes(node, childData.Children)
		}
		node.SetExpanded(childData.Expanded)
		parent.AddChild(node)
	}
}

func (a *App) findParentNode(root, target *CollectionNode) *CollectionNode {
	for _, child := range root.Children {
		if child == target {
			return root
		}
		if child.IsFolder {
			if parent := a.findParentNode(child, target); parent != nil {
				return parent
			}
		}
	}
	return nil
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
		fmt.Sprintf("[%s] %s (%s)", method, url, historyReq.Time.Format("15:04:05")),
		"",
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
		a.headersText.SetText(string(headersJSON), false)
	} else {
		a.headersText.SetText("", false)
	}

	if req.Body != "" {
		a.bodyText.SetText(req.Body, false)
	} else {
		a.bodyText.SetText("", false)
	}

	a.app.SetFocus(a.urlInput)
}

func (a *App) Run() error {
	return a.app.Run()
}

func main() {
	app := NewApp()
	app.Init()

	defer app.saveCollections() // Ini hanya menyimpan koleksi HTTP
	if err := app.Run(); err != nil {
		panic(err)
	}
}
