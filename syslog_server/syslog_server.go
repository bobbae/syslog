package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/natefinch/lumberjack"
)

//go:embed templates/*
//go:embed static/*
var embeddedFiles embed.FS


// Struct to manage log file and forwarding settings
type logFileHandler struct {
	logger            *lumberjack.Logger
	maxSize           int
	filename          string
	forwardAddr       string
	forwardProto      string
	forwardConn       net.Conn
	forwardLevel      int
	mu                sync.Mutex
	disableLogging    bool
	disableForwarding bool
	messages          []string // Added to store messages for web interface
}

// createLogFileHandler initializes a new log file handler with optional forwarding.
func createLogFileHandler(filename string, maxSize int, forwardAddr,
	forwardProto string, forwardLevel int) (*logFileHandler, error) {
	handler := &logFileHandler{
		maxSize:           maxSize,
		filename:          filename,
		forwardAddr:       forwardAddr,
		forwardProto:      forwardProto,
		forwardLevel:      forwardLevel,
		disableLogging:    false,
		disableForwarding: false,
		messages:          []string{},
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

// logMessage writes a message to the log file, forwards it if configured, and stores it for the web interface.
func (lh *logFileHandler) logMessage(remoteAddr, message string) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	if !lh.disableLogging {
		logEntry := fmt.Sprintf("[%s] %s\n", remoteAddr, message)
		if _, err := lh.logger.Write([]byte(logEntry)); err != nil {
			log.Printf("Error writing to log file: %v", err)
			return
		}
	}

	// Store message for web interface
	lh.messages = append(lh.messages, message)

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
}

// clearMessages clears all stored messages.
func (lh *logFileHandler) clearMessages() {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	lh.messages = []string{}
}

// handleStatsRequest returns server stats in HTML format.

// uiHandler serves the HTML interface with HTMX and Pico.css.
func uiHandler(handler *logFileHandler, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			MessageRows []*syslogMsg
		}{
			MessageRows: getSyslogs(handler),
		}

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// renderMessageRows dynamically generates HTML for the message rows.
func renderMessageRows(handler *logFileHandler) string {
	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.messages) == 0 {
		return "<tr><td colspan='5'>No messages yet.</td></tr>"
	}

	var result strings.Builder
	for i, msg := range handler.messages {
		syslogMsg, err := parseSyslogMessage(msg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}
		result.WriteString("<tr>")
		result.WriteString(fmt.Sprintf("<td>%d</td>", i+1))
		result.WriteString(fmt.Sprintf("<td>%v</td>", syslogMsg.Timestamp))
		result.WriteString(fmt.Sprintf("<td>%v</td>", syslogMsg.Hostname))
		result.WriteString(fmt.Sprintf("<td>%v</td>", syslogMsg.Appname))
		result.WriteString(fmt.Sprintf("<td>%v</td>", syslogMsg.Message))
		result.WriteString("</tr>")
	}
	return result.String()
}

func getSyslogs(handler *logFileHandler) []*syslogMsg {
	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.messages) == 0 {
		return []*syslogMsg{}
	}
	var result []*syslogMsg
	for i, msg := range handler.messages {
		syslogMsg, err := parseSyslogMessage(msg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}
		syslogMsg.Index = i + 1
		result = append(result, syslogMsg)
	}
	return result
}

func cleanString(s string) string {
	s = strings.ReplaceAll(s, "<script>", "<XXX>")
	s = strings.ReplaceAll(s, "</script>", "</XXX>")
	return strings.TrimSpace(s)
}

// parseSyslogMessage parses a syslog message into its components.
func parseSyslogMessage(msg string) (*syslogMsg, error) {
	if !strings.HasPrefix(msg, "<") {
		return nil, fmt.Errorf("not a syslog message")
	}
	_, _, err := parsePriority(msg)
	if err != nil {
		return nil, err
	}
	idx := strings.Index(msg, ">")
	if idx < 0 {
		return nil, fmt.Errorf("no syslog priority end character")
	}
	msg = msg[idx+1:]
	parts := strings.SplitN(msg, " ", 6)
	if len(parts) < 6 {
		return nil, fmt.Errorf("not enough parts in syslog message")
	}
	date := parts[0] + " " + parts[1] + " " + parts[2]
	host := parts[3]
	app := parts[4]
	app = strings.TrimSuffix(app, ":")
	message := parts[5]

	date = cleanString(date)
	host = cleanString(host)
	app = cleanString(app)
	message = cleanString(message)

	log.Printf("Parsed syslog message: date %s host %s app %s message %s", date, host, app, message)
	return &syslogMsg{
		Timestamp: date,
		Hostname:  host,
		Appname:   app,
		Message:   message,
	}, nil
}

type syslogMsg struct {
	Index int
	Timestamp string
	Hostname  string
	Appname   string
	Message   string
}

// messagesHandler serves the list of messages via HTMX.
func messagesHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, renderMessageRows(handler))
	}
}

// clearHandler clears all messages and returns an empty table body.
func clearHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		handler.clearMessages()
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<tr><td colspan='5'>No messages yet.</td></tr>")
	}
}

func syslogHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the body of the request.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		fmt.Println("Received syslog message:", string(body))
		handler.logMessage(r.RemoteAddr, string(body))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Syslog message received"})
	}
}

func main() {
	address := flag.String("addr", ":514", "Syslog server address")
	logFile := flag.String("file", "", "Log file path")
	maxSize := flag.Int("maxsize", 10, "Max log file size in MB")
	forwardAddr := flag.String("remote", "", "Upstream syslog server address")
	forwardProto := flag.String("proto", "udp", "Forwarding protocol: 'tcp' or 'udp'")
	forwardLevel := flag.Int("level", 6, "Forwarding priority level")
	apiAddr := flag.String("api", ":8080", "REST API and Web UI address")
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
		log.SetFlags(0)
		log.SetOutput(io.Discard)
	}

	logHandler, err := createLogFileHandler(*logFile, *maxSize, *forwardAddr, *forwardProto,
		*forwardLevel)
	if err != nil {
		log.Fatalf("Failed to create log handler: %v", err)
	}

	tmpl := template.Must(template.ParseFS(embeddedFiles, "templates/index.html"))

	http.HandleFunc("/static/search.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "static/search.js")
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(embeddedFiles))))
	http.HandleFunc("/", uiHandler(logHandler, tmpl))
	http.HandleFunc("/messages", messagesHandler(logHandler))
	http.HandleFunc("/clear", clearHandler(logHandler))
	http.HandleFunc("/syslog", syslogHandler(logHandler))

	go func() {
		log.Printf("Web UI and REST API listening on %s", *apiAddr)
		if err := http.ListenAndServe(*apiAddr, nil); err != nil {
			log.Fatalf("Failed to start Web UI and REST API: %v", err)
		}
	}()

	udpAddr, err := net.ResolveUDPAddr("udp", *address)
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Error starting UDP listener: %v", err)
	}
	defer udpConn.Close()

	log.Printf("Syslog server listening on UDP %s", *address)

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading UDP message: %v", err)
			continue
		}
		message := strings.TrimSpace(string(buffer[:n]))
		logHandler.logMessage(remoteAddr.String(), message)
	}
}

