package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sahilm/fuzzy"
	"golang.design/x/clipboard"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc/metadata"
)

// Version information injected at build time via ldflags.
// Informasi versi yang diinjeksi saat build via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// App encapsulates all the components and state of the TUI application.
// App membungkus semua komponen dan state dari aplikasi TUI.
type App struct {
	app       *tview.Application
	rootPages *tview.Pages // Main pages for switching between HTTP and gRPC views. / Halaman utama untuk beralih antara view HTTP dan gRPC.
	// Layout for the explorer panel and main content. / Layout untuk explorer panel dan content utama.
	contentLayout *tview.Flex
	appLayout     *tview.Flex // The top-level layout of the entire application. / Layout tingkat atas dari seluruh aplikasi.
	explorerPanel *tview.Flex // The left panel for Collections and History. / Panel kiri untuk Collections dan History.
	headerBar     *tview.Flex

	// Core application state / State inti aplikasi
	history         []Request
	collectionsRoot *CollectionNode

	// HTTP view components / Komponen view HTTP
	// Reference to the right panel in the HTTP view. / Referensi ke panel kanan di view HTTP.
	httpRightPanel *tview.Flex
	methodDrop     *tview.DropDown
	urlInput       *tview.InputField
	authType       *tview.DropDown
	authToken      *tview.InputField
	authUser       *tview.InputField
	authPass       *tview.InputField
	authPanel      *tview.Flex
	headersText    *tview.TextArea
	bodyText       *tview.TextArea
	responseText   *tview.TextView
	statusText     *tview.TextView // Shared status text for HTTP view / Teks status bersama untuk view HTTP

	// gRPC view components / Komponen view gRPC
	grpcServerInput    *tview.InputField
	grpcMethodInput    *tview.InputField
	grpcMethodList     *tview.List
	grpcMethodSelector *tview.Flex
	grpcRequestMeta    *tview.TextArea
	grpcRequestBody    *tview.TextArea
	grpcResponseView   *tview.TextView
	grpcStatusText     *tview.TextView

	// gRPC client and reflection state / State client gRPC dan reflection
	grpcReflectClient    *grpcreflect.Client
	grpcStub             grpcdynamic.Stub
	grpcConn             *grpc.ClientConn
	grpcCurrentService   string
	grpcAvailableMethods []string
	grpcAllMethods       []string
	grpcBodyCache        map[string]string

	// Shared UI components / Komponen UI bersama
	historyList     *tview.List
	collectionsTree *tview.TreeView

	// UI state flags / Flag untuk state UI
	allCollectionNodes     []*CollectionNode
	collectionMatchedNodes []*CollectionNode

	explorerPanelVisible bool
}

// loadCollections reads the collections data from a JSON file in the config directory.
// loadCollections membaca data collections dari file JSON di direktori config.
func (a *App) loadCollections() {
	path, _ := getConfigPath("collections.json")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("INFO: Collections file not found, will be created on exit.")
		return
	}
	if err := json.Unmarshal(data, &a.collectionsRoot); err != nil {
		log.Printf("ERROR: Failed to unmarshal collections: %v", err)
	}
}

// loadGrpcCache reads the gRPC request body cache from a JSON file.
// loadGrpcCache membaca cache body request gRPC dari file JSON.
func (a *App) loadGrpcCache() {
	path, _ := getConfigPath("grpc_cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("INFO: gRPC cache file not found, will be created on exit.")
		return
	}
	if err := json.Unmarshal(data, &a.grpcBodyCache); err != nil {
		log.Printf("ERROR: Failed to unmarshal gRPC cache: %v", err)
	}
}

// NewApp creates and initializes a new App instance.
// NewApp membuat dan menginisialisasi instance App baru.
func NewApp() *App {
	app := &App{
		app:     tview.NewApplication().EnableMouse(true).EnablePaste(true),
		history: make([]Request, 0),
		collectionsRoot: &CollectionNode{
			Name:     "Collections",
			IsFolder: true,
			Expanded: true,
		},
		grpcBodyCache:        make(map[string]string),
		explorerPanelVisible: false, // Explorer panel is hidden by default. / Explorer panel disembunyikan secara default.
	}
	app.loadCollections()
	app.loadGrpcCache()
	return app
}

// Init initializes all UI components, layouts, and keybindings.
// Init menginisialisasi semua komponen UI, layout, dan keybindings.
func (a *App) Init() {
	httpPage := a.createHttpPage()

	// The rootPages container allows switching between different main views.
	// Container rootPages memungkinkan pergantian antara view utama yang berbeda.
	a.rootPages = tview.NewPages()
	a.app.SetRoot(a.rootPages, true)
	a.rootPages.AddPage("http", httpPage, true, true)

	// The collectionsTree displays saved requests in a hierarchical view.
	// collectionsTree menampilkan request yang disimpan dalam view hierarkis.
	a.collectionsTree = tview.NewTreeView()
	a.collectionsTree.SetBorder(true).SetTitle("Collections")
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
			node.SetExpanded(!node.IsExpanded())
			collectionNode.Expanded = node.IsExpanded()
		} else {
			req := collectionNode.Request
			if req != nil {
				if req.Type == "grpc" {
					log.Println("Loading gRPC request from collection:", req.Name)
					a.loadGrpcRequest(*req)
				} else {
					log.Println("Loading HTTP request from collection:", req.Name)

					a.loadRequest(*req)
				}
			}
		}
	})
	a.collectionsTree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'n' {
			a.showCreateFolderModal()
			return nil
		}
		if event.Key() == tcell.KeyDelete {
			a.showDeleteConfirmationModal()
			return nil
		}
		return event
	})

	// Initialize the gRPC server input here so it can be accessed by the page and header. / Inisialisasi input server gRPC di sini agar dapat diakses oleh page dan header.
	a.grpcServerInput = tview.NewInputField().SetLabel("Server: ").SetText("localhost:8081").SetFieldBackgroundColor(tcell.ColorBlack)

	a.createGrpcPage()

	a.headerBar = a.createHeaderBar()

	// The historyList displays recently sent requests. / historyList menampilkan request yang baru saja dikirim.
	a.historyList = tview.NewList().ShowSecondaryText(false)
	a.historyList.SetBorder(true).SetTitle("History")
	a.historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {})

	// The explorerPanel holds the collections and history views. / explorerPanel menampung view Collections dan History.
	a.explorerPanel = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(a.collectionsTree, 0, 1, false).AddItem(a.historyList, 0, 1, false)
	// The top-level layout combines the explorer and the main content area. / Layout tingkat atas menggabungkan explorer dan area content utama.
	initialExplorerSize := 0
	initialExplorerProportion := 0
	if a.explorerPanelVisible {
		initialExplorerSize = 40
		initialExplorerProportion = 0 // Gunakan fixed size, bukan proporsi
	}
	// contentLayout menampung explorer dan halaman utama (HTTP/gRPC)
	a.contentLayout = tview.NewFlex().
		AddItem(a.explorerPanel, initialExplorerSize, initialExplorerProportion, false).
		AddItem(a.rootPages, 0, 1, true)

	a.appLayout = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(a.headerBar, 1, 0, false).AddItem(a.contentLayout, 0, 1, true)

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
[green]Ctrl+F[-] - Search Collections (Telescope)
[green]Ctrl+C[-] - Copy text from focused field
[green]Ctrl+Q[-] - Quit Application
[green]Ctrl+E[-] - Toggle Collections/History Panel
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
	helpText.SetBorder(true).SetTitle("Help")

	a.rootPages.AddPage("help", a.createModal(helpText, 60, 20), true, false)

	// Set global key bindings for the application.
	// Mengatur key bindings global untuk aplikasi.
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF5:
			currentPage, _ := a.rootPages.GetFrontPage()
			if currentPage == "http" {
				a.sendRequest()
			} else {
				a.sendGrpcRequest()
			}
			return nil
		case tcell.KeyF6:
			a.clearForm()
			return nil
		case tcell.KeyF7:
			a.app.SetFocus(a.historyList)
			return nil
		case tcell.KeyF8:
			a.showSaveRequestModal()
			return nil
		case tcell.KeyF9:
			a.app.SetFocus(a.collectionsTree)
			return nil
		case tcell.KeyF1:
			a.rootPages.ShowPage("help")
			return nil
		case tcell.KeyF12:
			a.switchMode()
			return nil
		case tcell.KeyEsc:
			// Handle modal closing first. If a modal is open, close it.
			// Otherwise, fall back to hiding the help page.
			if a.rootPages.HasPage("collectionSearchModal") {
				a.rootPages.RemovePage("collectionSearchModal")
				a.app.SetFocus(a.collectionsTree)
				return nil
			}
			if a.rootPages.HasPage("help") {
				a.rootPages.HidePage("help")
				return nil
			}
		case tcell.KeyCtrlE:
			a.toggleExplorerPanel()
			return nil
		case tcell.KeyCtrlF:
			a.showCollectionSearchModal()
			return nil
		case tcell.KeyCtrlQ:
			a.app.Stop()
			return nil
		case tcell.KeyCtrlC:
			// Copy text from focused widget to clipboard.
			// Menyalin teks dari widget yang sedang fokus ke clipboard.
			a.copyToClipboard()
			return nil
		}
		return event
	})

	a.app.SetRoot(a.appLayout, true)
	a.app.SetFocus(a.urlInput)
}

// createHeaderBar creates the top bar with dynamic buttons based on the current view.
// createHeaderBar membuat bar atas dengan tombol dinamis berdasarkan view saat ini.
func (a *App) createHeaderBar() *tview.Flex {
	header := tview.NewFlex()

	switchModeBtn := tview.NewButton("Switch (F12)").SetSelectedFunc(a.switchMode)

	httpSendBtn := tview.NewButton("Send (F5)").SetSelectedFunc(a.sendRequest)
	clearBtn := tview.NewButton("Clear (F6)").SetSelectedFunc(a.clearForm)
	saveBtn := tview.NewButton("Save (F8)").SetSelectedFunc(a.showSaveRequestModal)
	grpcSendBtn := tview.NewButton("Send (F5)").SetSelectedFunc(a.sendGrpcRequest)
	explorerBtn := tview.NewButton("Explorer (Ctrl+E)").SetSelectedFunc(a.toggleExplorerPanel)

	// The header is rebuilt whenever the page changes.
	// Header di-render ulang setiap kali page berubah.
	a.rootPages.SetChangedFunc(func() {
		page, _ := a.rootPages.GetFrontPage()
		header.Clear()

		if page == "http" {
			header.AddItem(httpSendBtn, 0, 1, false).
				AddItem(clearBtn, 0, 1, false).
				AddItem(saveBtn, 0, 1, false).
				AddItem(explorerBtn, 0, 1, false).
				AddItem(switchModeBtn, 0, 1, false)
		} else {
			header.AddItem(grpcSendBtn, 0, 1, false).
				AddItem(saveBtn, 0, 1, false).
				AddItem(explorerBtn, 0, 1, false).
				AddItem(switchModeBtn, 0, 1, false)
		}
	})

	// Set the initial state for the HTTP mode.
	// Mengatur state awal untuk mode HTTP.
	header.AddItem(httpSendBtn, 0, 1, false).
		AddItem(clearBtn, 0, 1, false).
		AddItem(saveBtn, 0, 1, false).
		AddItem(explorerBtn, 0, 1, false).
		AddItem(switchModeBtn, 0, 1, false)

	return header
}

// switchMode toggles the main view between HTTP and gRPC modes.
// switchMode mengganti view utama antara mode HTTP dan gRPC.
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

// updateGrpcMethodList filters the gRPC method list based on the user's query.
// updateGrpcMethodList memfilter daftar method gRPC berdasarkan query pengguna.
func (a *App) updateGrpcMethodList(query string) {
	a.grpcMethodList.Clear()

	if query == "" {
		a.grpcAvailableMethods = a.grpcAllMethods // Show all when query is empty
		a.hideMethodList()
		return
	}

	// Use a proper fuzzy finder to get ranked matches.
	// Menggunakan fuzzy finder untuk mendapatkan hasil yang terurut.
	matches := fuzzy.Find(query, a.grpcAllMethods)

	var matchedMethods []string
	for _, match := range matches {
		matchedMethods = append(matchedMethods, match.Str)
	}

	a.grpcAvailableMethods = matchedMethods

	if len(matchedMethods) > 0 {
		for _, method := range matchedMethods {
			a.grpcMethodList.AddItem(method, "", 0, nil)
		}
		listHeight := a.grpcMethodList.GetItemCount()
		if listHeight > 8 {
			listHeight = 8
		}
		a.grpcMethodSelector.ResizeItem(a.grpcMethodList, listHeight, 1)
	} else {
		a.grpcMethodList.AddItem("[gray]No results found", "", 0, nil)
		a.grpcMethodSelector.ResizeItem(a.grpcMethodList, 1, 1)
	}
}

// generateGrpcBodyTemplate uses reflection to create a JSON template for a gRPC method's request body.
// It runs in a goroutine to avoid blocking the UI. /
// generateGrpcBodyTemplate menggunakan reflection untuk membuat template JSON untuk body request dari sebuah method gRPC. Ini berjalan di goroutine agar tidak memblokir UI.
func (a *App) generateGrpcBodyTemplate(fullMethodName, existingBody string) {
	if a.grpcReflectClient == nil || fullMethodName == "" {
		return
	}

	go func() {
		parts := strings.SplitN(fullMethodName, "/", 2)
		if len(parts) != 2 {
			return // Format tidak valid
		}
		serviceName, methodName := parts[0], parts[1]

		sd, err := a.grpcReflectClient.ResolveService(serviceName)
		if err != nil {
			return
		}

		md := sd.FindMethodByName(methodName)
		if md == nil {
			return
		}

		reqType := md.GetInputType()

		newReqType := reqType.Unwrap().(protoreflect.MessageDescriptor)

		var existingData map[string]interface{}
		if err := json.Unmarshal([]byte(existingBody), &existingData); err != nil {
			existingData = make(map[string]interface{})
		}

		mergedMap := buildTemplateMap(newReqType, existingData)
		jsonTemplate, err := json.MarshalIndent(mergedMap, "", "  ")
		if err != nil {
			log.Printf("ERROR: could not marshal template: %v", err)
			return
		}

		a.app.QueueUpdateDraw(func() {
			if string(jsonTemplate) == "null" {
				jsonTemplate = []byte("{}")
			}
			a.grpcRequestBody.SetText(string(jsonTemplate), false)
		})
	}()
}

// buildTemplateMap recursively builds a map[string]interface{} from a Protobuf message descriptor,
// preserving existing values from the `existingData` map. /
// buildTemplateMap secara rekursif membangun sebuah map[string]interface{} dari message descriptor Protobuf, dengan mempertahankan value yang ada dari `existingData` map.
func buildTemplateMap(md protoreflect.MessageDescriptor, existingData map[string]interface{}) map[string]interface{} {
	template := make(map[string]interface{})
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.JSONName())

		if existingValue, ok := existingData[fieldName]; ok {
			if field.Kind() == protoreflect.MessageKind && !field.IsList() && !field.IsMap() {
				if subMap, isMap := existingValue.(map[string]interface{}); isMap {
					template[fieldName] = buildTemplateMap(field.Message(), subMap)
				} else {
					template[fieldName] = existingValue // Tipe tidak cocok, gunakan apa adanya.
				}
			} else {
				template[fieldName] = existingValue
			}
		} else {
			if field.IsList() {
				template[fieldName] = []interface{}{}
			} else if field.IsMap() {
				template[fieldName] = make(map[string]interface{})
			} else if field.Kind() == protoreflect.MessageKind {
				template[fieldName] = buildTemplateMap(field.Message(), make(map[string]interface{}))
			} else {
				template[fieldName] = getZeroValue(field)
			}
		}
	}
	return template
}

// getZeroValue returns the appropriate zero value for a Protobuf field type.
// getZeroValue mengembalikan zero value yang sesuai untuk tipe field Protobuf.
func getZeroValue(fd protoreflect.FieldDescriptor) interface{} {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return ""
	case protoreflect.BoolKind:
		return false
	default: // Int32, Int64, Float, Double, Enum, etc.
		return 0
	}
}

// grpcConnect establishes a connection to a gRPC server and uses reflection
// to discover available services and methods. It runs asynchronously. /
// grpcConnect membuat koneksi ke server gRPC dan menggunakan reflection untuk menemukan service dan method yang tersedia. Ini berjalan secara asinkron.
func (a *App) grpcConnect(onSuccess func()) {
	serverAddr := a.grpcServerInput.GetText()
	if serverAddr == "" {
		a.grpcStatusText.SetText("[red]Server address is required")
		return
	}

	a.grpcStatusText.SetText(fmt.Sprintf("[yellow]Connecting to %s...", serverAddr))

	go func() {
		if a.grpcConn != nil {
			a.grpcConn.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Using DialContext to establish connection immediately for reflection.
		// grpc.NewClient is lazy and doesn't connect until first RPC, which breaks reflection.
		// Menggunakan DialContext untuk membuat koneksi langsung untuk reflection.
		// grpc.NewClient bersifat lazy dan tidak connect sampai RPC pertama, yang membuat reflection gagal.
		conn, err := grpc.DialContext(ctx, serverAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				log.Printf("ERROR: gRPC dial failed for %s: %v", serverAddr, err)
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Failed to connect: %v", err))
			})
			return
		}

		a.grpcConn = conn
		a.grpcStub = grpcdynamic.NewStub(conn)

		// Use NewClientAuto for reflection (auto-detects v1 or v1alpha).
		// Menggunakan NewClientAuto untuk reflection (auto-detect v1 atau v1alpha).
		a.grpcReflectClient = grpcreflect.NewClientAuto(ctx, a.grpcConn)
		services, err := a.grpcReflectClient.ListServices()
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				log.Printf("ERROR: gRPC reflection ListServices failed: %v", err)
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Failed to list services: %v", err))
			})
			return
		}

		a.app.QueueUpdateDraw(func() {
			var serviceMethods []string
			for _, srv := range services {
				if srv == "grpc.reflection.v1alpha.ServerReflection" {
					continue
				}

				sd, err := a.grpcReflectClient.ResolveService(srv)
				if err != nil {
					continue
				}
				for _, md := range sd.GetMethods() {
					serviceMethods = append(serviceMethods, fmt.Sprintf("%s/%s", srv, md.GetName()))
				}
			}

			a.grpcAllMethods = serviceMethods
			a.grpcAvailableMethods = serviceMethods
			a.grpcMethodInput.SetText("")
			a.grpcMethodInput.SetPlaceholder("Type to search for a method...")
			a.grpcStatusText.SetText(fmt.Sprintf("[green]Connected to %s. Found %d services.", serverAddr, len(services)-1))

			if onSuccess != nil {
				onSuccess()
			}
		})
	}()
}

// sendGrpcRequest sends a gRPC request using the dynamic stub.
// sendGrpcRequest mengirimkan request gRPC menggunakan dynamic stub.
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
		parts := strings.SplitN(a.grpcCurrentService, "/", 2)
		if len(parts) != 2 {
			log.Printf("ERROR: Invalid gRPC service/method format: %s", a.grpcCurrentService)
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Invalid service/method format: %s", a.grpcCurrentService))
			})
			return
		}
		serviceName, methodName := parts[0], parts[1]

		sd, err := a.grpcReflectClient.ResolveService(serviceName)
		if err != nil {
			log.Printf("ERROR: Failed to resolve gRPC service '%s': %v", serviceName, err)
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Error resolving service '%s': %v", serviceName, err))
			})
			return
		}
		md := sd.FindMethodByName(methodName)
		if md == nil {
			log.Printf("ERROR: gRPC method '%s' not found in service '%s'", methodName, serviceName)
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Method '%s' not found in service '%s'", methodName, serviceName))
			})
			return
		}

		req := md.GetInputType()
		dynMsg := dynamic.NewMessage(req)
		bodyText := a.grpcRequestBody.GetText()
		if bodyText != "" {
			if err := dynMsg.UnmarshalJSON([]byte(bodyText)); err != nil {
				log.Printf("ERROR: Failed to unmarshal gRPC request body JSON: %v", err)
				a.app.QueueUpdateDraw(func() {
					a.grpcStatusText.SetText(fmt.Sprintf("[red]Error parsing request body JSON: %v", err))
				})
				return
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		metaText := a.grpcRequestMeta.GetText()
		if metaText != "" {
			var metaMap map[string]string
			if err := json.Unmarshal([]byte(metaText), &metaMap); err != nil {
				log.Printf("ERROR: Failed to unmarshal gRPC metadata JSON: %v", err)
				a.app.QueueUpdateDraw(func() {
					a.grpcStatusText.SetText(fmt.Sprintf("[red]Error parsing metadata JSON: %v", err))
				})
				return
			}
			ctx = metadata.NewOutgoingContext(ctx, metadata.New(metaMap))
		}

		log.Printf("INFO: Invoking gRPC method: %s", a.grpcCurrentService)
		start := time.Now()
		resp, err := a.grpcStub.InvokeRpc(ctx, md, dynMsg)
		duration := time.Since(start)

		a.app.QueueUpdateDraw(func() {
			if err != nil {
				log.Printf("ERROR: gRPC InvokeRpc failed for %s: %v", a.grpcCurrentService, err)
				a.grpcStatusText.SetText(fmt.Sprintf("[red]RPC Error: %v", err))
				a.grpcResponseView.SetText(fmt.Sprintf("[red]%v", err))
				return
			}

			dynResp, ok := resp.(*dynamic.Message)
			if !ok {
				log.Printf("ERROR: Unexpected gRPC response type: %T", resp)
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Internal Error: Unexpected response type %T", resp))
				a.grpcResponseView.SetText(fmt.Sprintf("Could not format response: %v", resp))
				return
			}
			respJSON, err := dynResp.MarshalJSONIndent()
			if err != nil {
				log.Printf("ERROR: Failed to marshal gRPC response JSON: %v", err)
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Error formatting response JSON: %v", err))
				a.grpcResponseView.SetText(fmt.Sprintf("Could not format response JSON: %v", err))
				return
			}
			log.Printf("INFO: gRPC call to %s successful. Duration: %v", a.grpcCurrentService, duration)
			a.grpcStatusText.SetText(fmt.Sprintf("[green]Success![-] | Duration: [cyan]%v[-]", duration))
			a.grpcResponseView.SetText(string(respJSON)).ScrollToBeginning()
		})
	}()

	historyReq := Request{
		Name:         a.grpcCurrentService,
		Type:         "grpc",
		GrpcServer:   a.grpcServerInput.GetText(),
		GrpcMethod:   a.grpcCurrentService,
		GrpcMetadata: a.grpcRequestMeta.GetText(),
		Body:         a.grpcRequestBody.GetText(),
		Time:         time.Now(),
	}
	a.history = append([]Request{historyReq}, a.history...)
	a.updateHistoryView()
}

// loadRequestFromHistory loads a selected request from the history list into the UI.
// loadRequestFromHistory memuat request yang dipilih dari daftar History ke dalam UI.
func (a *App) loadRequestFromHistory(index int) {
	if index < len(a.history) {
		req := a.history[index]
		if req.Type == "grpc" {
			a.loadGrpcRequest(req)
		} else {
			a.loadRequest(req)
		}
	}
}

// showSaveRequestModal displays a modal form to save the current request to a collection.
// showSaveRequestModal menampilkan form modal untuk menyimpan request saat ini ke Collection.
func (a *App) showSaveRequestModal() {
	var defaultName string
	requestPage, _ := a.rootPages.GetFrontPage()
	if requestPage == "grpc" {
		if a.grpcCurrentService != "" {
			defaultName = a.grpcCurrentService
		} else {
			defaultName = "New gRPC Request"
		}
	} else {
		_, method := a.methodDrop.GetCurrentOption()
		url := a.urlInput.GetText()
		defaultName = fmt.Sprintf("%s %s", method, url)
	}
	nameInput := tview.NewInputField().SetLabel("Request Name").SetText(defaultName).SetFieldWidth(60)

	form := tview.NewForm().
		AddFormItem(nameInput).
		AddButton("Save", func() {
			name := nameInput.GetText()
			a.saveCurrentRequest(name, requestPage)
			a.rootPages.RemovePage("saveModal")
		}).
		AddButton("Cancel", func() {
			a.rootPages.RemovePage("saveModal")
		})

	form.SetBorder(true).SetTitle("Save Request to Collection")
	modal := a.createModal(form, 80, 7)
	a.app.SetFocus(nameInput)
	a.rootPages.AddPage("saveModal", modal, true, true)
}

// showCreateFolderModal displays a modal form to create a new folder in the collections.
// showCreateFolderModal menampilkan form modal untuk membuat folder baru di Collections.
func (a *App) showCreateFolderModal() {
	nameInput := tview.NewInputField().SetLabel("Folder Name").SetFieldWidth(50)

	form := tview.NewForm().
		AddFormItem(nameInput).
		AddButton("Create", func() {
			folderName := nameInput.GetText()
			if folderName != "" {
				a.createCollectionFolder(folderName)
			}
			a.rootPages.RemovePage("createFolderModal")
		}).
		AddButton("Cancel", func() {
			a.rootPages.RemovePage("createFolderModal")
		})

	form.SetBorder(true).SetTitle("Create New Folder")
	modal := a.createModal(form, 70, 7)
	a.app.SetFocus(nameInput)
	a.rootPages.AddPage("createFolderModal", modal, true, true)
}

// showDeleteConfirmationModal displays a confirmation dialog before deleting a collection item.
// showDeleteConfirmationModal menampilkan dialog konfirmasi sebelum menghapus item dari Collection.
func (a *App) showDeleteConfirmationModal() {
	selectedNode := a.collectionsTree.GetCurrentNode()
	if selectedNode == nil || selectedNode == a.collectionsTree.GetRoot() {
		return
	}

	ref := selectedNode.GetReference()
	nodeToDelete, ok := ref.(*CollectionNode)
	if !ok {
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Are you sure you want to delete '%s'?", nodeToDelete.Name)).
		AddButtons([]string{"Delete", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Delete" {
				a.deleteCollectionItem(nodeToDelete)
			}
			a.rootPages.RemovePage("deleteConfirmModal")
		})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event // Allow default modal navigation
	})

	a.rootPages.AddPage("deleteConfirmModal", modal, true, true)
}

// deleteCollectionItem removes the specified node from the collections data structure and refreshes the tree.
// deleteCollectionItem menghapus node yang ditentukan dari struktur data Collections dan me-refresh tree view.
func (a *App) deleteCollectionItem(nodeToDelete *CollectionNode) {
	parentData := a.findParentNode(a.collectionsRoot, nodeToDelete)
	if parentData == nil {
		return
	}

	parentData.Children = removeNode(parentData.Children, nodeToDelete)
	a.populateCollectionsTree()
	a.saveCollections()
}

// createCollectionFolder adds a new folder to the collections.
// createCollectionFolder menambahkan folder baru ke Collections.
func (a *App) createCollectionFolder(name string) {
	selectedTreeNode := a.collectionsTree.GetCurrentNode()
	if selectedTreeNode == nil {
		return
	}

	var parentData *CollectionNode
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

	if parentData == nil {
		parentData = a.collectionsRoot // Default to root if no parent is found
	}

	newFolder := &CollectionNode{Name: name, IsFolder: true}
	parentData.Children = append(parentData.Children, newFolder)
	a.populateCollectionsTree()
	a.saveCollections()
	a.flattenCollections()
}

// showCollectionSearchModal displays a Telescope-like modal for searching collections.
// showCollectionSearchModal menampilkan modal seperti Telescope untuk mencari koleksi.
func (a *App) showCollectionSearchModal() {
	// The list to display search results.
	resultsList := tview.NewList().ShowSecondaryText(false)

	// The input field for the search query.
	searchInput := tview.NewInputField().
		SetLabel("Search Collections: ").
		SetFieldBackgroundColor(tcell.ColorBlack)

	// The main layout for the modal.
	modalLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 1, 0, true).
		AddItem(resultsList, 0, 1, false)
	modalLayout.SetBorder(true).SetTitle("Telescope Search")

	// Function to update search results.
	updateResults := func(query string) {
		resultsList.Clear()
		a.collectionMatchedNodes = nil

		if query == "" {
			resultsList.AddItem("Type to search...", "", 0, nil)
			return
		}

		var nodeNames []string
		for _, node := range a.allCollectionNodes {
			nodeNames = append(nodeNames, node.Name)
		}

		matches := fuzzy.Find(query, nodeNames)

		if len(matches) > 0 {
			for _, match := range matches {
				originalNode := a.allCollectionNodes[match.Index]
				a.collectionMatchedNodes = append(a.collectionMatchedNodes, originalNode)
				var icon string
				if originalNode.IsFolder {
					icon = "📁"
				} else if originalNode.Request != nil && originalNode.Request.Type == "grpc" {
					icon = "🔌" // gRPC
				} else {
					icon = "🌐" // HTTP/REST
				}
				resultsList.AddItem(fmt.Sprintf("%s %s", icon, originalNode.Name), "", 0, nil)
			}
		} else {
			resultsList.AddItem("No results found", "", 0, nil)
		}
	}

	// Set initial state.
	updateResults("")

	// Link search input changes to the update function.
	searchInput.SetChangedFunc(updateResults)

	// Handle keyboard navigation between input and list.
	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown && resultsList.GetItemCount() > 0 {
			a.app.SetFocus(resultsList)
			return nil
		}
		return event
	})
	// Handle selection from the results list.
	resultsList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index < len(a.collectionMatchedNodes) {
			node := a.collectionMatchedNodes[index]
			if node.Request != nil {
				if node.Request.Type == "grpc" {
					a.loadGrpcRequest(*node.Request)
				} else {
					a.loadRequest(*node.Request)
				}
			}
			a.rootPages.RemovePage("collectionSearchModal")
		}
	})

	resultsList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp && resultsList.GetCurrentItem() == 0 {
			a.app.SetFocus(searchInput)
			return nil
		}
		return event
	})
	// Create and show the modal page.
	modal := a.createModal(modalLayout, 80, 15)
	a.rootPages.AddPage("collectionSearchModal", modal, true, true)
	a.app.SetFocus(searchInput)
}

// saveCurrentRequest gathers data from the UI and saves it as a new collection item.
// saveCurrentRequest mengumpulkan data dari UI dan menyimpannya sebagai item Collection baru.
func (a *App) saveCurrentRequest(name string, requestType string) {
	var requestData *Request
	if requestType == "grpc" {
		requestData = &Request{
			Name:         name,
			Type:         "grpc",
			GrpcServer:   a.grpcServerInput.GetText(),
			GrpcMethod:   a.grpcCurrentService,
			GrpcMetadata: a.grpcRequestMeta.GetText(),
			Body:         a.grpcRequestBody.GetText(),
			Time:         time.Now(),
		}
	} else {
		_, method := a.methodDrop.GetCurrentOption()
		authTypeIndex, _ := a.authType.GetCurrentOption()
		url := a.urlInput.GetText()
		headersText := a.headersText.GetText()
		body := a.bodyText.GetText()
		authToken := a.authToken.GetText()
		authUser := a.authUser.GetText()
		authPass := a.authPass.GetText()

		headers := make(map[string]string)
		if headersText != "" {
			if err := json.Unmarshal([]byte(headersText), &headers); err != nil {
				log.Printf("WARN: Headers JSON is invalid, will be saved as raw text: %v", err)
			}
		}

		requestData = &Request{
			Name:       name,
			Type:       "http",
			Method:     method,
			URL:        url,
			Headers:    headers,
			HeadersRaw: headersText, // Always save raw text / Selalu simpan teks mentah
			AuthType:   authTypeIndex,
			AuthToken:  authToken,
			AuthUser:   authUser,
			AuthPass:   authPass,
			Body:       body,
			Time:       time.Now(),
		}
	}

	newNode := &CollectionNode{
		Name:     name,
		IsFolder: false,
		Request:  requestData,
	}

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
		parentData = a.collectionsRoot
	}

	parentData.Children = append(parentData.Children, newNode)

	a.populateCollectionsTree()
	a.saveCollections()
	a.flattenCollections()
}

// populateCollectionsTree rebuilds the entire collections tree view from the data model.
// populateCollectionsTree membangun kembali seluruh tree view Collections dari data model.
func (a *App) populateCollectionsTree() {
	rootNode := tview.NewTreeNode(a.collectionsRoot.Name).SetReference(a.collectionsRoot)
	a.collectionsTree.SetRoot(rootNode).SetCurrentNode(rootNode)
	a.addTreeNodes(rootNode, a.collectionsRoot.Children)
	rootNode.SetExpanded(a.collectionsRoot.Expanded)
	a.flattenCollections()
}

// flattenCollections creates a flat list of all nodes in the collection for searching.
// flattenCollections membuat daftar datar dari semua node di koleksi untuk pencarian.
func (a *App) flattenCollections() {
	a.allCollectionNodes = nil
	var recursiveFlatten func(nodes []*CollectionNode)
	recursiveFlatten = func(nodes []*CollectionNode) {
		for _, node := range nodes {
			a.allCollectionNodes = append(a.allCollectionNodes, node)
			if node.IsFolder && len(node.Children) > 0 {
				recursiveFlatten(node.Children)
			}
		}
	}
	recursiveFlatten(a.collectionsRoot.Children)
}

// addTreeNodes is a recursive helper to add nodes to the collections tree view.
// addTreeNodes adalah helper rekursif untuk menambahkan node ke tree view Collections.
func (a *App) addTreeNodes(parent *tview.TreeNode, children []*CollectionNode) {
	for _, childData := range children {
		var icon string
		if childData.IsFolder {
			icon = "📁"
		} else if childData.Request != nil && childData.Request.Type == "grpc" {
			icon = "🔌" // gRPC indicator / Penanda gRPC
		} else {
			icon = "🌐" // HTTP/REST indicator / Penanda HTTP/REST
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

// findParentNode recursively searches for the parent of a target node within the collections data structure.
// findParentNode secara rekursif mencari parent dari sebuah target node di dalam struktur data Collections.
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

// removeNode is a helper function to remove a node from a slice of nodes.
// removeNode adalah helper untuk menghapus sebuah node dari slice.
func removeNode(slice []*CollectionNode, node *CollectionNode) []*CollectionNode {
	for i, n := range slice {
		if n == node {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// getAuthTypeIndex converts auth type string to its corresponding index.
// getAuthTypeIndex mengkonversi string auth type ke index yang sesuai.
func getAuthTypeIndex(authType string) int {
	switch authType {
	case "Bearer Token":
		return 1
	case "Basic Auth":
		return 2
	case "API Key":
		return 3
	default:
		return 0
	}
}

// updateAuthPanel dynamically changes the authentication input fields based on the selected auth type.
// updateAuthPanel secara dinamis mengubah field input otentikasi berdasarkan auth type yang dipilih.
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
		// Reset label in case it was changed by API Key.
		// Reset label jika sebelumnya diubah oleh API Key.
		a.authToken.SetLabel("Token: ")
		a.authPanel.AddItem(a.authToken, 0, 1, false)

	case 2: // Basic Auth
		basicFlex := tview.NewFlex()
		basicFlex.AddItem(a.authUser, 0, 1, false)
		basicFlex.AddItem(a.authPass, 0, 1, false)
		a.authPanel.AddItem(basicFlex, 0, 1, false)

	case 3: // API Key
		// Reuse authToken field for API Key to persist the value.
		// Menggunakan kembali field authToken untuk API Key agar nilainya tersimpan.
		a.authToken.SetLabel("API Key: ")
		a.authPanel.AddItem(a.authToken, 0, 1, false)
	}
}

// createModal is a helper function to wrap a primitive in a centered modal layout.
// createModal adalah helper untuk membungkus sebuah primitive dalam layout modal yang terpusat.
func (a *App) createModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

// sendRequest gathers data from the HTTP UI, calls the HTTP client, and updates the UI with the response.
// sendRequest mengumpulkan data dari UI HTTP, memanggil HTTP client, dan memperbarui UI dengan response.
func (a *App) sendRequest() {
	_, method := a.methodDrop.GetCurrentOption()
	_, authType := a.authType.GetCurrentOption()

	requestData := HttpRequestData{
		Method:    method,
		URL:       a.urlInput.GetText(),
		Body:      a.bodyText.GetText(),
		AuthType:  authType,
		AuthToken: a.authToken.GetText(),
		AuthUser:  a.authUser.GetText(),
		AuthPass:  a.authPass.GetText(),
		Headers:   make(map[string]string),
	}

	if requestData.URL == "" {
		a.statusText.SetText("[red]Error: URL is required")
		return
	}

	headersJSON := a.headersText.GetText()
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &requestData.Headers); err != nil {
			a.statusText.SetText(fmt.Sprintf("[red]Error parsing headers: %v", err))
			return
		}
	}

	a.statusText.SetText("[yellow]Sending request...")

	go func() {
		respData := doHttpRequest(requestData)

		a.app.QueueUpdateDraw(func() {
			if respData.Error != nil {
				a.statusText.SetText(fmt.Sprintf("[red]Error: %v", respData.Error))
				a.responseText.SetText(fmt.Sprintf("[red]Error: %v", respData.Error))
				return
			}

			statusColor := "[green]"
			if respData.StatusCode >= 400 {
				statusColor = "[red]"
			} else if respData.StatusCode >= 300 {
				statusColor = "[yellow]"
			}

			a.statusText.SetText(fmt.Sprintf("%s%s[-] | Duration: [cyan]%v[-]",
				statusColor, respData.Status, respData.Duration))

			var formattedBody bytes.Buffer
			bodyToDisplay := respData.Body
			if err := json.Indent(&formattedBody, respData.Body, "", "  "); err == nil {
				bodyToDisplay = formattedBody.Bytes()
			}

			var responseBuilder strings.Builder
			responseBuilder.WriteString(fmt.Sprintf("[yellow]Status:[-] %s%s[-]\n", statusColor, respData.Status))
			responseBuilder.WriteString(fmt.Sprintf("[yellow]Duration:[-] [cyan]%v[-]\n", respData.Duration))
			responseBuilder.WriteString(fmt.Sprintf("[yellow]Content-Length:[-] %d bytes\n\n", len(respData.Body)))
			responseBuilder.WriteString("[yellow]Headers:[-]\n")

			for k, v := range respData.Headers {
				responseBuilder.WriteString(fmt.Sprintf("  [cyan]%s:[-] %s\n", k, strings.Join(v, ", ")))
			}

			responseBuilder.WriteString(fmt.Sprintf("\n[yellow]Body:[-]\n%s", string(bodyToDisplay)))

			a.responseText.SetText(responseBuilder.String()).ScrollToBeginning()
		})
	}()

	historyReq := Request{
		Method:    requestData.Method,
		URL:       requestData.URL,
		Headers:   requestData.Headers,
		Body:      requestData.Body,
		Time:      time.Now(),
		Type:      "http",
		AuthType:  getAuthTypeIndex(requestData.AuthType),
		AuthToken: requestData.AuthToken,
		AuthUser:  requestData.AuthUser,
		AuthPass:  requestData.AuthPass,
	}
	a.history = append([]Request{historyReq}, a.history...)

	a.updateHistoryView()
}

// updateHistoryView clears and repopulates the history list view.
// updateHistoryView membersihkan dan mengisi ulang list view History.
func (a *App) updateHistoryView() {
	a.historyList.Clear()
	for i, req := range a.history {
		var title string
		if req.Type == "grpc" {
			title = fmt.Sprintf("[gRPC] %s (%s)", req.Name, req.Time.Format("15:04:05"))
		} else {
			title = fmt.Sprintf("[%s] %s (%s)", req.Method, req.URL, req.Time.Format("15:04:05"))
		}
		// Capture the index in a local variable to avoid closure issue.
		// Menangkap index di variabel lokal untuk menghindari masalah closure.
		index := i
		a.historyList.AddItem(title, "", 0, func() { a.loadRequestFromHistory(index) })
	}
}

// clearForm resets all input fields in the HTTP view to their default state.
// clearForm me-reset semua input field di view HTTP ke state default.
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

// loadRequest populates the HTTP view with data from a Request object.
// loadRequest mengisi view HTTP dengan data dari sebuah object Request.
func (a *App) loadRequest(req Request) {
	// Switch to HTTP page if not already there.
	// Pindah ke halaman HTTP jika belum di sana.
	a.rootPages.SwitchToPage("http")

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for i, m := range methods {
		if m == req.Method {
			a.methodDrop.SetCurrentOption(i)
			break
		}
	}

	a.authType.SetCurrentOption(req.AuthType)
	a.updateAuthPanel(req.AuthType)
	a.authToken.SetText(req.AuthToken)
	a.authUser.SetText(req.AuthUser)
	a.authPass.SetText(req.AuthPass)

	a.urlInput.SetText(req.URL)

	// Prefer HeadersRaw (exact user input) over marshaling Headers map.
	// Prioritaskan HeadersRaw (input user yang asli) daripada marshal Headers map.
	if req.HeadersRaw != "" {
		a.headersText.SetText(req.HeadersRaw, false)
	} else if len(req.Headers) > 0 {
		// Fallback for backward compatibility with old saved requests.
		// Fallback untuk kompatibilitas dengan request yang disimpan sebelumnya.
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

// loadGrpcRequest populates the gRPC view with data from a Request object and initiates a connection.
// loadGrpcRequest mengisi view gRPC dengan data dari sebuah object Request dan memulai koneksi.
func (a *App) loadGrpcRequest(req Request) {
	a.rootPages.SwitchToPage("grpc")

	a.grpcServerInput.SetText(req.GrpcServer)
	a.grpcRequestMeta.SetText(req.GrpcMetadata, false)
	a.grpcMethodInput.SetText(req.GrpcMethod)
	a.grpcRequestBody.SetText(req.Body, false)
	a.grpcCurrentService = req.GrpcMethod
	a.grpcStatusText.SetText(fmt.Sprintf("Loaded: [green]%s[-]", req.Name))

	if req.GrpcMethod != "" {
		a.grpcBodyCache[req.GrpcMethod] = req.Body
	}

	onConnectSuccess := func() {
		a.grpcCurrentService = req.GrpcMethod
		a.grpcMethodInput.SetText(req.GrpcMethod)
	}

	a.grpcConnect(onConnectSuccess)
	a.app.SetFocus(a.grpcServerInput)
}

// beautifyJSON formats the JSON content of a given text area.
// beautifyJSON memformat konten JSON dari sebuah text area.
func (a *App) beautifyJSON(textArea *tview.TextArea) {
	currentText := textArea.GetText()
	if currentText == "" {
		return
	}

	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, []byte(currentText), "", "  ")
	if err != nil {
		log.Printf("WARN: Failed to beautify JSON: %v", err)
		return
	}
	textArea.SetText(prettyJSON.String(), false)
	// No need to copy here, Ctrl+C is now the standard way.
}

// copyToClipboard copies text from the currently focused widget to the system clipboard.
// copyToClipboard menyalin teks dari widget yang sedang fokus ke clipboard sistem.
func (a *App) copyToClipboard() {
	var textToCopy string

	focused := a.app.GetFocus()
	switch widget := focused.(type) {
	case *tview.TextArea:
		textToCopy = widget.GetText()
	case *tview.TextView:
		textToCopy = widget.GetText(true)
	case *tview.InputField:
		textToCopy = widget.GetText()
	default:
		log.Printf("DEBUG: Cannot copy from widget type %T", focused)
		return
	}

	if textToCopy == "" {
		return
	}

	// Initialize clipboard (only needed once, but safe to call multiple times).
	// Inisialisasi clipboard (hanya perlu sekali, tapi aman dipanggil berkali-kali).
	if err := clipboard.Init(); err != nil {
		log.Printf("ERROR: Failed to initialize clipboard: %v", err)
		return
	}

	clipboard.Write(clipboard.FmtText, []byte(textToCopy))
	log.Printf("INFO: Copied %d bytes to clipboard", len(textToCopy))
}

// toggleExplorerPanel shows or hides the left-side explorer panel.
// toggleExplorerPanel menampilkan atau menyembunyikan explorer panel di sisi kiri.
func (a *App) toggleExplorerPanel() {
	a.explorerPanelVisible = !a.explorerPanelVisible
	if a.explorerPanelVisible {
		a.contentLayout.ResizeItem(a.explorerPanel, 40, 0)
	} else {
		a.contentLayout.ResizeItem(a.explorerPanel, 0, 0)
	}
}

// Run starts the application's event loop.
// Run memulai event loop dari aplikasi.
func (a *App) Run() error {
	return a.app.Run()
}

// main is the entry point of the application.
// main adalah entry point dari aplikasi.
func main() {
	initLogger()
	app := NewApp()
	app.Init()

	defer func() {
		app.saveCollections()
		app.saveGrpcCache()
		log.Println("INFO: Application shutting down.")
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
