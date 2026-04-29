// main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
)

// ServerConfig holds configuration for the SPA server
type ServerConfig struct {
	SourceDir   string
	Port        int
	Host        string
	IndexFile   string
	StaticDir   string
	ShowQR      bool
	OpenBrowser bool
	EnableCORS  bool
	BasePath    string
}

// Server represents the SPA server
type Server struct {
	config     *ServerConfig
	httpServer *http.Server
}

// NewServer creates a new SPA server
func NewServer(config *ServerConfig) *Server {
	return &Server{
		config: config,
	}
}

// getPublicIPAddress returns the network IP address
func getPublicIPAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var ipAddress string
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback and non-IPv4
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			ipStr := ip.String()
			// Prefer 192.168.x.x or 10.x.x.x addresses
			if strings.HasPrefix(ipStr, "192.168.") ||
				strings.HasPrefix(ipStr, "10.") ||
				strings.HasPrefix(ipStr, "172.") {
				if ipAddress == "" || strings.HasPrefix(ipStr, "192.168.") {
					ipAddress = ipStr
				}
			} else if ipAddress == "" {
				ipAddress = ipStr
			}
		}
	}

	if ipAddress == "" {
		return "", fmt.Errorf("no public IP address found")
	}

	return ipAddress, nil
}

// openBrowser opens the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, etc.
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// generateQRCode generates and prints QR code as ASCII
func generateQRCode(url string) error {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return err
	}

	// Print QR code as ASCII
	fmt.Println("\nScan QR code to open on mobile:")
	fmt.Println(qr.ToSmallString(false))
	return nil
}

// corsMiddleware adds CORS headers if enabled
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves the SPA index.html for all routes
type spaHandler struct {
	indexPath string
	basePath  string
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if the base path matches
	if h.basePath != "/" && !strings.HasPrefix(r.URL.Path, h.basePath) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, h.indexPath)
}

// fileServerWithNotFoundHandler serves static files with SPA fallback
type fileServerWithNotFoundHandler struct {
	fs        http.Handler
	indexPath string
}

func (h *fileServerWithNotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a response recorder to check if file was found
	recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	h.fs.ServeHTTP(recorder, r)

	// If file not found, serve index.html
	if recorder.status == http.StatusNotFound {
		http.ServeFile(w, r, h.indexPath)
	}
}

// responseRecorder records the status code
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Start starts the SPA server
func (s *Server) Start() error {
	// Validate source directory
	if _, err := os.Stat(s.config.SourceDir); os.IsNotExist(err) {
		return fmt.Errorf("directory '%s' does not exist", s.config.SourceDir)
	}

	// Validate index file
	indexPath := filepath.Join(s.config.SourceDir, s.config.IndexFile)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("index file '%s' not found in %s", s.config.IndexFile, s.config.SourceDir)
	}

	mux := http.NewServeMux()

	// Apply CORS middleware if enabled
	var handler http.Handler = mux
	if s.config.EnableCORS {
		handler = corsMiddleware(handler)
	}

	// Auto-detect common static directories
	if s.config.StaticDir == "" {
		commonDirs := []string{"res", "fonts", "css", "js", "lib", "assets", "static", "public"}
		for _, dir := range commonDirs {
			dirPath := filepath.Join(s.config.SourceDir, dir)
			if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
				staticHandler := http.FileServer(http.Dir(dirPath))
				mux.Handle("/"+dir+"/", http.StripPrefix("/"+dir, staticHandler))
				fmt.Printf("Serving static directory: /%s\n", dir)
			}
		}
	} else {
		staticPath := filepath.Join(s.config.SourceDir, s.config.StaticDir)
		if info, err := os.Stat(staticPath); err == nil && info.IsDir() {
			staticHandler := http.FileServer(http.Dir(staticPath))
			mux.Handle("/"+s.config.StaticDir+"/", http.StripPrefix("/"+s.config.StaticDir, staticHandler))
			fmt.Printf("Serving static directory: %s\n", staticPath)
		}
	}

	// Serve static files from source directory with SPA fallback
	fileServer := &fileServerWithNotFoundHandler{
		fs:        http.FileServer(http.Dir(s.config.SourceDir)),
		indexPath: indexPath,
	}

	// Normalize base path
	normalizedBasePath := s.config.BasePath
	if normalizedBasePath != "/" && !strings.HasSuffix(normalizedBasePath, "/") {
		normalizedBasePath += "/"
	}

	// Handle base path routing
	if normalizedBasePath == "/" {
		mux.Handle("/", fileServer)
	} else {
		mux.Handle(normalizedBasePath, http.StripPrefix(strings.TrimSuffix(normalizedBasePath, "/"), fileServer))
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler: handler,
	}

	// Start server
	listenAddr := fmt.Sprintf("http://localhost:%d", s.config.Port)
	fmt.Printf("\nStarting server on %s\n", listenAddr)

	// Get network IP for QR code
	var networkURL string
	if s.config.ShowQR {
		ip, err := getPublicIPAddress()
		if err == nil {
			networkURL = fmt.Sprintf("http://%s:%d", ip, s.config.Port)
		}
	}

	// Start server in goroutine
	go func() {
		var err error
		if s.config.Host == "0.0.0.0" {
			err = s.httpServer.ListenAndServe()
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Display server info
	fmt.Println("\nSPA is ready!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   Local:    %s\n", listenAddr)
	if networkURL != "" {
		fmt.Printf("   Network:  %s\n", networkURL)
	}
	fmt.Printf("   Serving:  %s\n", s.config.SourceDir)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Generate and display QR code
	if s.config.ShowQR && networkURL != "" {
		if err := generateQRCode(networkURL); err != nil {
			fmt.Printf("Failed to generate QR code: %v\n", err)
		}
	}

	// Open browser if enabled
	if s.config.OpenBrowser {
		if err := openBrowser(listenAddr); err != nil {
			// Silently fail if browser can't be opened
			_ = err
		}
	}

	fmt.Println("Press Ctrl+C to stop the server\n")

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// parsePort parses port from string with validation
func parsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port number: %s", portStr)
	}
	return port, nil
}

func main() {
	// Parse command line arguments
	var (
		port        int
		host        string
		indexFile   string
		staticDir   string
		noQR        bool
		noOpen      bool
		enableCORS  bool
		basePath    string
		showVersion bool
	)

	flag.IntVar(&port, "port", 5600, "Port to listen on")
	flag.IntVar(&port, "p", 5600, "Port to listen on (shorthand)")
	flag.StringVar(&host, "host", "0.0.0.0", "Host to bind to")
	flag.StringVar(&host, "h", "0.0.0.0", "Host to bind to (shorthand)")
	flag.StringVar(&indexFile, "index", "index.html", "SPA entry point file")
	flag.StringVar(&indexFile, "i", "index.html", "SPA entry point file (shorthand)")
	flag.StringVar(&staticDir, "static-dir", "", "Static files directory (relative to source)")
	flag.StringVar(&staticDir, "s", "", "Static files directory (shorthand)")
	flag.BoolVar(&noQR, "no-qr", false, "Disable QR code generation")
	flag.BoolVar(&noOpen, "no-open", false, "Disable auto-opening browser")
	flag.BoolVar(&enableCORS, "cors", false, "Enable CORS for all origins")
	flag.StringVar(&basePath, "base", "/", "Base path for the SPA")
	flag.StringVar(&basePath, "b", "/", "Base path for the SPA (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [directory] [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  directory    Directory to serve (default: current directory)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./dist -p 8080\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./build --no-qr --cors\n", os.Args[0])
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("serve-spa version 1.0.0\n")
		return
	}

	// Get directory argument
	var sourceDir string
	args := flag.Args()
	if len(args) > 0 {
		sourceDir = args[0]
	} else {
		sourceDir = "."
	}

	// Convert to absolute path
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to resolve path '%s': %v\n", sourceDir, err)
		os.Exit(1)
	}

	// Validate port
	if port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: Port must be between 1 and 65535\n")
		os.Exit(1)
	}

	// Create and start server
	config := &ServerConfig{
		SourceDir:   absSourceDir,
		Port:        port,
		Host:        host,
		IndexFile:   indexFile,
		StaticDir:   staticDir,
		ShowQR:      !noQR,
		OpenBrowser: !noOpen,
		EnableCORS:  enableCORS,
		BasePath:    basePath,
	}

	server := NewServer(config)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		fmt.Println("\n\nShutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			fmt.Printf("Error during shutdown: %v\n", err)
		}
		fmt.Println("Server stopped.")
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Keep main goroutine running
	select {}
}