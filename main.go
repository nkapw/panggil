package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/reflect/protoreflect"

	"net/http"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc/metadata"
)

type Request struct {
	Name    string            `json:"name"`
	Method  string            `json:"method,omitempty"` // For HTTP
	URL     string            `json:"url,omitempty"`    // For HTTP
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
	Time    time.Time         `json:"time"`

	Type         string `json:"type"` // "http" or "grpc"
	GrpcServer   string `json:"grpc_server,omitempty"`
	GrpcMethod   string `json:"grpc_method,omitempty"`
	GrpcMetadata string `json:"grpc_metadata,omitempty"`
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
	contentLayout   *tview.Flex  // Layout untuk explorer dan halaman utama
	appLayout       *tview.Flex  // Layout paling atas aplikasi
	explorerPanel   *tview.Flex  // Panel kiri untuk Collections/History
	httpRightPanel  *tview.Flex  // Referensi ke panel kanan di view HTTP
	methodDrop      *tview.DropDown
	urlInput        *tview.InputField
	authType        *tview.DropDown
	authToken       *tview.InputField
	authUser        *tview.InputField
	authPass        *tview.InputField
	authPanel       *tview.Flex
	headersText     *tview.TextArea
	bodyText        *tview.TextArea
	headerBar       *tview.Flex
	responseText    *tview.TextView
	historyList     *tview.List
	collectionsTree *tview.TreeView
	statusText      *tview.TextView
	history         []Request
	collectionsRoot *CollectionNode

	// gRPC components
	grpcPages            *tview.Pages
	grpcServerInput      *tview.InputField
	grpcServiceTree      *tview.TreeView
	grpcRequestMeta      *tview.TextArea
	grpcRequestBody      *tview.TextArea
	grpcResponseView     *tview.TextView
	grpcStatusText       *tview.TextView
	grpcReflectClient    *grpcreflect.Client
	grpcStub             grpcdynamic.Stub
	grpcConn             *grpc.ClientConn
	grpcCurrentService   string
	grpcBodyCache        map[string]string // Cache untuk body request gRPC
	explorerPanelVisible bool
}

// getConfigPath mengembalikan path absolut untuk file konfigurasi,
// memastikan file tersebut disimpan di direktori konfigurasi pengguna yang sesuai.
func getConfigPath(filename string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not get user config dir: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "myhttp")
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return "", fmt.Errorf("could not create app config dir: %w", err)
	}

	return filepath.Join(appConfigDir, filename), nil
}

func initLogger() {
	path, err := getConfigPath("myhttp.log")
	if err != nil {
		log.Fatalf("FATAL: Failed to get log file path: %v", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("FATAL: error opening log file: %v", err)
	}
	log.SetOutput(f)
	log.Println("INFO: Logger initialized. Application starting.")
}

func (a *App) saveCollections() {
	path, err := getConfigPath("collections.json")
	if err != nil {
		log.Printf("ERROR: Could not get config path for collections: %v", err)
		return
	}
	data, err := json.MarshalIndent(a.collectionsRoot, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal collections: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("ERROR: Failed to write collections file: %v", err)
	}
}

func (a *App) saveGrpcCache() {
	path, err := getConfigPath("grpc_cache.json")
	if err != nil {
		log.Printf("ERROR: Could not get config path for gRPC cache: %v", err)
		return
	}
	data, err := json.MarshalIndent(a.grpcBodyCache, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal gRPC cache: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("ERROR: Failed to write gRPC cache file: %v", err)
	}
}

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

func NewApp() *App {
	app := &App{
		app:     tview.NewApplication().EnableMouse(true),
		history: make([]Request, 0),
		collectionsRoot: &CollectionNode{
			Name:     "Collections",
			IsFolder: true,
			Expanded: true,
		},
		grpcBodyCache:        make(map[string]string),
		explorerPanelVisible: false, // Sembunyikan explorer panel secara default
	}
	// Muat koleksi yang ada, jika tidak ada, root akan tetap ada
	app.loadCollections()
	// Muat cache gRPC yang ada
	app.loadGrpcCache()
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

	leftPanel.AddItem(topFlex, 3, 0, false)
	leftPanel.AddItem(a.authPanel, 3, 0, false)
	leftPanel.AddItem(a.headersText, 0, 1, false)
	leftPanel.AddItem(a.bodyText, 0, 1, false)

	// Right panel - Response and History
	a.httpRightPanel = tview.NewFlex().SetDirection(tview.FlexRow)

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

	// Collections
	a.collectionsTree = tview.NewTreeView()
	a.collectionsTree.SetBorder(true).SetTitle("Collections")
	a.populateCollectionsTree() // Tampilkan semua koleksi saat startup
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
			req := collectionNode.Request
			if req != nil {
				// Default ke http jika tipe tidak ada (untuk kompatibilitas mundur)
				if req.Type == "grpc" {
					a.loadGrpcRequest(*req)
				} else {
					a.loadRequest(*req)
				}
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

	a.httpRightPanel.AddItem(a.statusText, 3, 0, false).AddItem(a.responseText, 0, 1, false)
	mainFlex.AddItem(leftPanel, 0, 1, true).AddItem(a.httpRightPanel, 0, 1, false)

	// Inisialisasi input server gRPC di sini agar bisa diakses oleh halaman dan header
	a.grpcServerInput = tview.NewInputField().SetLabel("Server: ").SetText("localhost:8081").SetFieldBackgroundColor(tcell.ColorBlack)

	// Buat halaman gRPC
	a.createGrpcPage()

	// Tambahkan header/switcher di atas
	a.headerBar = a.createHeaderBar()

	// History - Inisialisasi di sini agar bisa diakses oleh explorer
	a.historyList = tview.NewList().ShowSecondaryText(false)
	a.historyList.SetBorder(true).SetTitle("History")
	a.historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		// Logika pemuatan akan ditangani oleh updateHistoryView
		// Untuk saat ini, kita hanya perlu tahu item mana yang dipilih.
		// Kita bisa menambahkan logika untuk memuat request yang benar di sini nanti.
	})

	// Buat panel explorer di sebelah kiri
	a.explorerPanel = tview.NewFlex().SetDirection(tview.FlexRow)
	a.explorerPanel.AddItem(a.collectionsTree, 0, 1, false).AddItem(a.historyList, 0, 1, false)

	// Layout paling atas yang menggabungkan explorer dan konten utama
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

	// Key bindings
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF5:
			// Kirim request berdasarkan mode aktif
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
			a.rootPages.HidePage("help")
			return nil
		case tcell.KeyCtrlE:
			a.toggleExplorerPanel()
			return nil
		}
		return event
	})

	a.app.SetRoot(a.appLayout, true)
	a.app.SetFocus(a.urlInput)
}

func (a *App) createHeaderBar() *tview.Flex {
	header := tview.NewFlex()

	switchModeBtn := tview.NewButton("Switch (F12)").SetSelectedFunc(a.switchMode)

	// Definisikan semua tombol
	httpSendBtn := tview.NewButton("Send (F5)").SetSelectedFunc(a.sendRequest)
	clearBtn := tview.NewButton("Clear (F6)").SetSelectedFunc(a.clearForm)
	saveBtn := tview.NewButton("Save (F8)").SetSelectedFunc(a.showSaveRequestModal)
	grpcSendBtn := tview.NewButton("Send (F5)").SetSelectedFunc(a.sendGrpcRequest)
	explorerBtn := tview.NewButton("Explorer (Ctrl+E)").SetSelectedFunc(a.toggleExplorerPanel)

	// Atur ulang header saat mode berubah
	a.rootPages.SetChangedFunc(func() {
		page, _ := a.rootPages.GetFrontPage()
		header.Clear() // Hapus semua item lama

		if page == "http" {
			header.AddItem(httpSendBtn, 0, 1, false).
				AddItem(clearBtn, 0, 1, false).
				AddItem(saveBtn, 0, 1, false).
				AddItem(explorerBtn, 0, 1, false).
				AddItem(switchModeBtn, 0, 1, false)
		} else {
			// Tombol Connect dan input server sekarang ada di dalam halaman gRPC
			header.AddItem(grpcSendBtn, 0, 1, false).
				AddItem(saveBtn, 0, 1, false).
				AddItem(explorerBtn, 0, 1, false).
				AddItem(switchModeBtn, 0, 1, false)
		}
	})

	// Atur state awal untuk mode HTTP
	header.AddItem(httpSendBtn, 0, 1, false).
		AddItem(clearBtn, 0, 1, false).
		AddItem(saveBtn, 0, 1, false).
		AddItem(explorerBtn, 0, 1, false).
		AddItem(switchModeBtn, 0, 1, false)

	return header
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
	a.grpcServiceTree = tview.NewTreeView()
	a.grpcServiceTree.SetBorder(true).SetTitle("Services")
	a.grpcServiceTree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		// Simpan service/method yang dipilih
		if serviceName, ok := ref.(string); ok && len(node.GetChildren()) == 0 {
			// 1. Simpan body dari method sebelumnya (jika ada) ke cache
			if a.grpcCurrentService != "" {
				a.grpcBodyCache[a.grpcCurrentService] = a.grpcRequestBody.GetText()
			}

			// 2. Atur method baru sebagai yang aktif
			a.grpcCurrentService = serviceName
			a.grpcStatusText.SetText(fmt.Sprintf("Selected: [green]%s", serviceName))
			a.grpcResponseView.SetText("") // Hapus respons sebelumnya
			// 3. Buat template untuk method baru, gunakan body dari cache jika ada
			a.generateGrpcBodyTemplate(serviceName, a.grpcBodyCache[serviceName])
		} else {
			// Jika yang dipilih adalah folder (service), buka/tutup saja
			node.SetExpanded(!node.IsExpanded())
		}
	})

	// Konten utama di sebelah kanan service tree
	mainContent := tview.NewFlex().SetDirection(tview.FlexRow)

	// Baris atas: Input Server dan Status
	topRow := tview.NewFlex()
	serverInputFlex := tview.NewFlex().
		AddItem(a.grpcServerInput, 0, 1, true).
		AddItem(tview.NewButton("Connect").SetSelectedFunc(func() { a.grpcConnect(nil) }), 12, 0, false)
	serverInputFlex.SetBorder(true).SetTitle("Server")

	a.grpcStatusText = tview.NewTextView().SetDynamicColors(true).SetText("[yellow]Not connected")
	a.grpcStatusText.SetBorder(true).SetTitle("Status")
	topRow.AddItem(serverInputFlex, 0, 1, true).AddItem(a.grpcStatusText, 0, 1, false)

	// Baris bawah: Request dan Response
	bottomRow := tview.NewFlex()
	middlePanel := tview.NewFlex().SetDirection(tview.FlexRow)
	a.grpcRequestMeta = tview.NewTextArea().SetPlaceholder("Metadata (JSON format)...")
	a.grpcRequestMeta.SetBorder(true).SetTitle("Metadata")
	a.grpcRequestBody = tview.NewTextArea().SetPlaceholder("Select a service method to see the request body template...")
	a.grpcRequestBody.SetBorder(true).SetTitle("Request Body")
	middlePanel.AddItem(a.grpcRequestMeta, 0, 1, false).AddItem(a.grpcRequestBody, 0, 2, false)

	a.grpcResponseView = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWordWrap(true)
	a.grpcResponseView.SetBorder(true).SetTitle("Response")
	bottomRow.AddItem(middlePanel, 0, 1, false).AddItem(a.grpcResponseView, 0, 1, false)

	mainContent.AddItem(topRow, 3, 0, true).AddItem(bottomRow, 0, 1, false)
	grpcFlex.AddItem(a.grpcServiceTree, 30, 0, true).AddItem(mainContent, 0, 1, false)
	a.rootPages.AddPage("grpc", grpcFlex, true, false)
}

func (a *App) generateGrpcBodyTemplate(fullMethodName, existingBody string) {
	if a.grpcReflectClient == nil {
		return
	}

	// Jalankan di goroutine agar tidak memblokir UI
	go func() {
		parts := strings.SplitN(fullMethodName, "/", 2)
		if len(parts) != 2 {
			return // Format tidak valid
		}
		serviceName, methodName := parts[0], parts[1]

		// ResolveService adalah panggilan jaringan, harus di luar thread utama
		sd, err := a.grpcReflectClient.ResolveService(serviceName)
		if err != nil {
			return // Service tidak ditemukan
		}

		md := sd.FindMethodByName(methodName)
		if md == nil {
			return // Method tidak ditemukan
		}

		// Buat template JSON dari deskriptor pesan
		reqType := md.GetInputType()

		// Gunakan .Unwrap() untuk mendapatkan deskriptor API baru yang kompatibel.
		newReqType := reqType.Unwrap().(protoreflect.MessageDescriptor)

		// Gunakan body dari cache (atau string kosong jika belum ada)
		var existingData map[string]interface{}
		if err := json.Unmarshal([]byte(existingBody), &existingData); err != nil {
			existingData = make(map[string]interface{}) // Jika parse gagal, mulai dengan map kosong
		}

		mergedMap := buildTemplateMap(newReqType, existingData)
		jsonTemplate, err := json.MarshalIndent(mergedMap, "", "  ")
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.grpcRequestBody.SetText(fmt.Sprintf("/* could not marshal template: %v */", err), false)
			})
			return
		}

		// Kirim pembaruan UI kembali ke thread utama
		a.app.QueueUpdateDraw(func() {
			// Jika template kosong (misalnya untuk Empty message), tampilkan {}
			if string(jsonTemplate) == "null" {
				jsonTemplate = []byte("{}")
			}
			a.grpcRequestBody.SetText(string(jsonTemplate), false)
		})
	}()
}

// buildTemplateMap secara rekursif membuat map[string]interface{} dari deskriptor pesan Protobuf.
// Ini memastikan semua field disertakan dalam template JSON, tidak seperti marshalling pesan kosong.
func buildTemplateMap(md protoreflect.MessageDescriptor, existingData map[string]interface{}) map[string]interface{} {
	// Hindari rekursi tak terbatas pada tipe well-known
	if md.FullName() == "google.protobuf.Any" {
		return map[string]interface{}{
			"@type": "type.googleapis.com/your.Type",
			"value": "...",
		}
	}

	template := make(map[string]interface{})
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := string(field.JSONName())

		if existingValue, ok := existingData[fieldName]; ok {
			// Nilai sudah ada, pertahankan
			if field.Kind() == protoreflect.MessageKind && !field.IsList() && !field.IsMap() {
				// Untuk sub-pesan, lakukan rekursi dengan data yang ada
				if subMap, isMap := existingValue.(map[string]interface{}); isMap {
					template[fieldName] = buildTemplateMap(field.Message(), subMap)
				} else {
					// Tipe tidak cocok, gunakan nilai yang ada apa adanya
					template[fieldName] = existingValue
				}
			} else {
				template[fieldName] = existingValue
			}
		} else {
			// Nilai belum ada, buat template default
			if field.IsList() {
				template[fieldName] = []interface{}{}
			} else if field.IsMap() {
				template[fieldName] = make(map[string]interface{})
			} else if field.Kind() == protoreflect.MessageKind {
				template[fieldName] = buildTemplateMap(field.Message(), nil)
			} else {
				template[fieldName] = getZeroValue(field)
			}
		}
	}
	// Jika map kosong setelah iterasi (misalnya untuk google.protobuf.Empty), kembalikan nil
	// agar json.MarshalIndent menghasilkan "null" yang bisa kita tangani.
	if len(template) == 0 {
		return nil
	}
	return template
}

// getZeroValue mengembalikan nilai nol yang sesuai untuk tipe field Protobuf.
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

func (a *App) grpcConnect(onSuccess func()) {
	serverAddr := a.grpcServerInput.GetText()
	if serverAddr == "" {
		a.grpcStatusText.SetText("[red]Server address is required")
		return
	}

	// Update status di UI dan jalankan koneksi di goroutine agar tidak membeku
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
				log.Printf("ERROR: gRPC dial failed for %s: %v", serverAddr, err)
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
				log.Printf("ERROR: gRPC reflection ListServices failed: %v", err)
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
			// Jalankan callback jika koneksi dan discovery berhasil
			if onSuccess != nil {
				onSuccess()
			}
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
			log.Printf("ERROR: Invalid gRPC service/method format: %s", a.grpcCurrentService)
			a.app.QueueUpdateDraw(func() {
				a.grpcStatusText.SetText(fmt.Sprintf("[red]Invalid service/method format: %s", a.grpcCurrentService))
			})
			return
		}
		serviceName, methodName := parts[0], parts[1]

		// 2. Resolve service and then find method descriptor
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

		// 3. Create dynamic message from JSON body
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

		// 4. Prepare context with metadata
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

		// 5. Invoke RPC
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

			// 6. Format and display response
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

	// Add to history
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
	// Update history view to reflect the new item in the current mode
	a.updateHistoryView()
}
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

func (a *App) showSaveRequestModal() {
	var defaultName string
	currentPage, _ := a.rootPages.GetFrontPage()
	if currentPage == "grpc" {
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
			a.saveCurrentRequest(name)
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
	var requestData *Request
	currentPage, _ := a.rootPages.GetFrontPage()

	if currentPage == "grpc" {
		requestData = &Request{
			Name:         name,
			Type:         "grpc",
			GrpcServer:   a.grpcServerInput.GetText(),
			GrpcMethod:   a.grpcCurrentService,
			GrpcMetadata: a.grpcRequestMeta.GetText(),
			Body:         a.grpcRequestBody.GetText(),
			Time:         time.Now(),
		}
	} else { // HTTP
		_, method := a.methodDrop.GetCurrentOption()
		url := a.urlInput.GetText()
		headersText := a.headersText.GetText()
		body := a.bodyText.GetText()

		headers := make(map[string]string)
		if headersText != "" {
			_ = json.Unmarshal([]byte(headersText), &headers)
		}

		requestData = &Request{
			Name:    name,
			Type:    "http",
			Method:  method,
			URL:     url,
			Headers: headers,
			Body:    body,
			Time:    time.Now(),
		}
	}

	// Jika tidak ada data yang bisa disimpan
	if requestData == nil {
		return
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
	a.saveCollections() // Save changes
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
		log.Printf("ERROR: Failed to create HTTP request for %s %s: %v", method, url, err)
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
	log.Printf("INFO: Sending HTTP request: %s %s", method, url)
	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		a.statusText.SetText(fmt.Sprintf("[red]Error: %v", err))
		log.Printf("ERROR: HTTP request failed for %s %s: %v", method, url, err)
		a.responseText.SetText(fmt.Sprintf("[red]Error: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read HTTP response body: %v", err)
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

	log.Printf("INFO: HTTP request to %s %s completed with status %s. Duration: %v", method, url, resp.Status, duration)
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
	historyReq.Type = "http" // Ensure type is set for history
	a.history = append([]Request{historyReq}, a.history...)

	// Update history view to reflect the new item in the current mode
	a.updateHistoryView()
}

func (a *App) updateHistoryView() {
	a.historyList.Clear()
	for _, req := range a.history {
		var title string
		if req.Type == "grpc" {
			title = fmt.Sprintf("[gRPC] %s (%s)", req.Name, req.Time.Format("15:04:05"))
		} else {
			title = fmt.Sprintf("[%s] %s (%s)", req.Method, req.URL, req.Time.Format("15:04:05"))
		}
		a.historyList.AddItem(title, "", 0, nil)
	}
	// Set ulang selected func untuk menangkap index yang benar dari list yang sudah difilter
	a.historyList.SetSelectedFunc(func(i int, mainText string, secondaryText string, shortcut rune) {
		a.loadRequestFromHistory(i)
	})
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

func (a *App) loadGrpcRequest(req Request) {
	// Pindah ke halaman gRPC
	a.rootPages.SwitchToPage("grpc")

	// 1. Isi field dari data yang tersimpan
	a.grpcServerInput.SetText(req.GrpcServer)
	a.grpcRequestMeta.SetText(req.GrpcMetadata, false)
	a.grpcRequestBody.SetText(req.Body, false)

	// 2. Perbarui status dan service yang aktif
	a.grpcCurrentService = req.GrpcMethod
	if a.grpcCurrentService != "" {
		a.grpcStatusText.SetText(fmt.Sprintf("Selected: [green]%s", a.grpcCurrentService))
	}

	// 3. Simpan body yang dimuat ke cache agar tidak hilang saat beralih
	if req.GrpcMethod != "" {
		a.grpcBodyCache[req.GrpcMethod] = req.Body
	}

	// 4. Definisikan callback yang akan dijalankan setelah koneksi berhasil
	onConnectSuccess := func() {
		if req.GrpcMethod == "" {
			return
		}

		// Cari node di tree yang sesuai dengan method yang disimpan
		var targetNode *tview.TreeNode
		a.grpcServiceTree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
			if ref := node.GetReference(); ref != nil {
				if serviceName, ok := ref.(string); ok && serviceName == req.GrpcMethod {
					targetNode = node
					return false // Hentikan pencarian
				}
			}
			return true // Lanjutkan pencarian
		})

		// Jika ditemukan, pilih node tersebut
		if targetNode != nil {
			a.grpcServiceTree.SetCurrentNode(targetNode)
		}
	}

	// 5. Panggil grpcConnect dengan callback untuk otomatis terhubung dan memilih method
	a.grpcConnect(onConnectSuccess)
	a.app.SetFocus(a.grpcServerInput)
}

func (a *App) toggleExplorerPanel() {
	a.explorerPanelVisible = !a.explorerPanelVisible
	if a.explorerPanelVisible {
		// Tampilkan panel dengan memberikan ukuran tetap 40
		a.contentLayout.ResizeItem(a.explorerPanel, 40, 0)
	} else {
		// Sembunyikan panel dengan mengatur fixedSize dan proportion menjadi 0.
		a.contentLayout.ResizeItem(a.explorerPanel, 0, 0)
	}
}

func (a *App) Run() error {
	return a.app.Run()
}

func main() {
	initLogger()
	app := NewApp()
	app.Init()

	// Simpan state ke file saat aplikasi keluar
	defer func() {
		app.saveCollections()
		app.saveGrpcCache()
		log.Println("INFO: Application shutting down.")
	}()
	if err := app.Run(); err != nil {
		panic(err)
	}
}
