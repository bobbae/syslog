package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/natefinch/lumberjack"
)

// Stats structure to track server metrics
type stats struct {
	LogsReceived  uint64 `json:"logs_received"`
	LogsForwarded uint64 `json:"logs_forwarded"`
}

// Struct to manage log file and forwarding settings
type logFileHandler struct {
	logger            *lumberjack.Logger // Open log file
	lineCount         int                // In-memory line counter
	maxSize           int                // Max size of the log file in megabytes
	filename          string             // Path to the log file
	forwardAddr       string             // Address of the upstream syslog server (optional)
	forwardProto      string             // Protocol for forwarding (tcp/udp)
	forwardConn       net.Conn           // Persistent connection for forwarding
	forwardLevel      int                // Priority level for forwarding
	mu                sync.Mutex         // Mutex for thread-safe access
	stats             *stats             // Pointer to the stats structure
	disableLogging    bool               // Flag to stop writing to the log file (default: false)
	disableForwarding bool               // Flag to stop forwarding logs (default: false)
}

// createLogFileHandler initializes a new log file handler with optional forwarding.
func createLogFileHandler(filename string, maxSize int, forwardAddr,
	forwardProto string, forwardLevel int, stats *stats) (*logFileHandler, error) {

	handler := &logFileHandler{
		lineCount:         0,
		maxSize:           maxSize,
		filename:          filename,
		forwardAddr:       forwardAddr,
		forwardProto:      forwardProto,
		forwardLevel:      forwardLevel,
		stats:             stats,
		disableLogging:    false,
		disableForwarding: false,
	}
	if filename == "" {
		handler.disableLogging = true
	} else {
		handler.logger = &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
	}

	if forwardAddr != "" {
		if err := handler.setupForwardConnection(); err != nil {
			return nil, fmt.Errorf("failed to connect to upstream syslog server: %w", err)
		}
	} else {
		handler.disableForwarding = true
	}

	return handler, nil
}

// setupForwardConnection establishes a persistent connection to the upstream syslog server.
func (lh *logFileHandler) setupForwardConnection() error {
	conn, err := net.Dial(lh.forwardProto, lh.forwardAddr)
	if err != nil {
		return err
	}

	lh.forwardConn = conn
	log.Printf("Connected to upstream syslog server at %s via %s", lh.forwardAddr, lh.forwardProto)
	return nil
}

func parsePriority(buf string) (int, int, error) {
	if !strings.HasPrefix(buf, "<") {
		return 0, 0, fmt.Errorf("no syslog priority start character")
	}
	ix := strings.Index(buf, ">")
	if ix < 0 {
		return 0, 0, fmt.Errorf("no syslog priority end character")
	}
	priority, err := strconv.Atoi(buf[1:ix])
	if err != nil {
		return 0, 0, err
	}
	facility := priority / 8
	severity := priority % 8
	return facility, severity, nil
}

// logMessage writes a message to the log file and forwards it if configured.
func (lh *logFileHandler) logMessage(remoteAddr, message string) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	if !lh.disableLogging {
		logEntry := fmt.Sprintf("[%s] %s\n", remoteAddr, message)
		if _, err := lh.logger.Write([]byte(logEntry)); err != nil {
			log.Printf("Error writing to log file: %v", err)
			return
		}
		atomic.AddUint64(&lh.stats.LogsReceived, 1)
		lh.lineCount++
	}

	if lh.forwardAddr != "" && !lh.disableForwarding {
		_, severity, err := parsePriority(message)
		if err != nil {
			log.Printf("Error parsing syslog message: %v", err)
			return
		}
		if lh.forwardLevel > severity {
			return
		}
		lh.forwardMessage(message)
	}
}

// forwardMessage sends the log message to the upstream syslog server.
func (lh *logFileHandler) forwardMessage(message string) {
	if lh.disableForwarding {
		//log.Printf("Forwarding is disabled. Skipping forward for: %s", message)
		return
	}
	if lh.forwardConn == nil {
		log.Printf("Forward connection is not available, reconnecting...")
		if err := lh.setupForwardConnection(); err != nil {
			log.Printf("Failed to reconnect to upstream syslog server: %v", err)
			return
		}
	}
	_, err := lh.forwardConn.Write([]byte(message + "\n"))
	if err != nil {
		log.Printf("Error forwarding message, reconnecting: %v", err)
		lh.forwardConn.Close()
		if err := lh.setupForwardConnection(); err != nil {
			log.Printf("Failed to reconnect: %v", err)
			return
		}
		if _, err := lh.forwardConn.Write([]byte(message + "\n")); err != nil {
			log.Printf("Failed to forward message after reconnecting: %v", err)
		}
	}
	atomic.AddUint64(&lh.stats.LogsForwarded, 1)
}

// handleStatsRequest returns server stats in JSON format.
func handleStatsRequest(stats *stats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// Helper function to start a UDP server
func startUDPServer(addr string, bufferSize int, handler *logFileHandler) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Error starting UDP listener: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, bufferSize)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading UDP message: %v", err)
			continue
		}
		message := strings.TrimSpace(string(buffer[:n]))
		handler.logMessage(remoteAddr.String(), message)
	}
}

// Helper function to start a TCP server
func startTCPServer(addr string, handler *logFileHandler) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Error starting TCP listener: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting TCP connection: %v", err)
			continue
		}
		go handleTCPConnection(conn, handler)
	}
}

// Handle individual TCP connection
func handleTCPConnection(conn net.Conn, handler *logFileHandler) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("Error reading TCP message: %v", err)
			return
		}
		handler.logMessage(conn.RemoteAddr().String(), strings.TrimSpace(message))
	}
}

func main() {
	address := flag.String("addr", ":514", "Syslog server address")
	bufferSize := flag.Int("buf", 1024, "Buffer size for UDP packets")
	logFile := flag.String("file", "", "Log file path")
	maxSize := flag.Int("maxsize", 10, "Max log file size in MB")
	forwardAddr := flag.String("remote", "", "Upstream syslog server address")
	forwardProto := flag.String("proto", "udp", "Forwarding protocol: 'tcp' or 'udp'")
	forwardLevel := flag.Int("level", 6, "Forwarding priority level")
	apiAddr := flag.String("api", ":8080", "REST API address")
	debuglog := flag.String("debug", "", "debug log file")
	flag.Parse()

	if *debuglog != "" {
		f, err := os.OpenFile(*debuglog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Error opening debug log file: %v", err)
		}
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		//log.SetOutput(os.Stdout)
		log.SetFlags(0)
		log.SetOutput(io.Discard)
	}

	stats := &stats{}
	logHandler, err := createLogFileHandler(*logFile, *maxSize, *forwardAddr, *forwardProto,
		*forwardLevel, stats)
	if err != nil {
		log.Fatalf("Failed to create log handler: %v", err)
	}

	http.HandleFunc("/stats", handleStatsRequest(stats))
	http.HandleFunc("/echo", echoHandler)

	go func() {
		log.Printf("REST API listening on %s", *apiAddr)
		if err := http.ListenAndServe(*apiAddr, nil); err != nil {
			log.Fatalf("Failed to start REST API: %v", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); startUDPServer(*address, *bufferSize, logHandler) }()
	go func() { defer wg.Done(); startTCPServer(*address, logHandler) }()
	wg.Wait()
}
