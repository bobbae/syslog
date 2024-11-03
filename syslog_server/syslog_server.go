package main

import (
	"bytes"
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
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/natefinch/lumberjack"
)

//go:embed templates/*
//go:embed static/*
var embeddedFiles embed.FS

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
	config            *Config  // Added to store configuration
	muConfig          sync.Mutex
}

type Config struct {
	MaxMessages int `json:"maxMessages"`
	DisableLog  bool `json:"disableLog"`
	AnomaliesOnly  bool   `json:"anomaliesOnly"`
	MessagePattern string `json:"messagepattern"`
	Severity       int    `json:"severity"`
	AppName        string `json:"appname"`
	HostName       string `json:"hostname"`
	ApiKey         string `json:"apiKey"`
	Url            string `json:"url"`
	Model          string `json:"model"`
	LogFile string `json:"logfile"`
}

type syslogMsg struct {
	Timestamp string `json:"timestamp"`
	Hostname  string `json:"hostname"`
	Appname   string `json:"appname"`
	Message   string `json:"message"`
}

type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type LLMConfig struct {
	apiKey string
	model  string
	url    string
}

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
		config:            &Config{MaxMessages: 1000, DisableLog: false, AnomaliesOnly: false, Severity: 7, AppName: "", MessagePattern: ""},
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
	if len(lh.messages) >= lh.config.MaxMessages && lh.config.MaxMessages > 0 {
		lh.messages = lh.messages[len(lh.messages) - lh.config.MaxMessages:]
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


func (lh *logFileHandler) updateConfig(config *Config) {
	lh.muConfig.Lock()
	defer lh.muConfig.Unlock()
	lh.config = config
	if len(lh.messages) >= lh.config.MaxMessages && lh.config.MaxMessages > 0 {
		lh.messages = lh.messages[len(lh.messages) - lh.config.MaxMessages:]
	}
}

func (lh *logFileHandler) getConfig() *Config {
	lh.muConfig.Lock()
	defer lh.muConfig.Unlock()
	return lh.config
}

func isRegexp(str string) bool {
	_, err := regexp.Compile(str)
	return err == nil
}

func renderMessageRows(handler *logFileHandler) (template.HTML, error) {
	handler.mu.Lock()
	defer handler.mu.Unlock()

	config := handler.getConfig()
	var messages []syslogMsg
	if len(handler.messages) == 0 {
		return template.HTML("<tr><td colspan='5'>No messages yet.</td></tr>"), nil
	}
	if config.AnomaliesOnly {
		if config.ApiKey == "" {
			return template.HTML("<tr><td colspan='5'>OpenAI API key not found. Please set the OPENAI_API_KEY environment variable and rerun the server.</td></tr>"), nil
		}
		apiKey := config.ApiKey
		url := config.Url
		model := config.Model
		if url == "" {
			url = "https://api.openai.com/v1/chat/completions"
		}
		if model == "" {
			model = "gpt-3.5-turbo"
		}
		anomalies, err := findAnomalies(LLMConfig{apiKey: apiKey, url: url, model: model}, handler.messages)
		if err != nil {
			return template.HTML("<tr><td colspan='5'>Error analyzing syslog messages: " + err.Error() + "</td></tr>"), nil
		}
		handler.messages = anomalies
	}
	for _, msg := range handler.messages {
		syslogMsg, err := parseSyslogMessage(msg)
		if err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Apply filtering based on configuration
		if config.AppName != "" && !strings.Contains(syslogMsg.Appname, config.AppName) {
			continue
		}
		if config.HostName != "" && !strings.Contains(syslogMsg.Hostname, config.HostName) {
			continue
		}
		if config.MessagePattern != "" {
			if isRegexp(config.MessagePattern) {
				if config.MessagePattern != "" {
					matched, err := regexp.MatchString(config.MessagePattern, syslogMsg.Message)
					if err != nil {
						log.Printf("Error matching regex: %v", err)
						continue
					}
					if !matched {
						continue
					}
				}
			} else {
				if !strings.Contains(syslogMsg.Message, config.MessagePattern) {
					continue
				}
			}
		}
		_, msgSeverity, err := parsePriority(msg)
		if err != nil {
			log.Printf("Error parsing priority: %v", err)
			continue
		}
		if msgSeverity > config.Severity {
			continue
		}
		messages = append(messages, *syslogMsg)
	}
	tmpl, err := template.ParseFiles("templates/message_rows.html")
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, struct {
		Messages []syslogMsg
	}{Messages: messages})
	if err != nil {
		return "", err
	}
	return template.HTML(tpl.String()), nil
}

func findAnomalies(config LLMConfig, messages []string) ([]string, error) {

	requestBody := CompletionRequest{
		Model: config.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: `Given a list of syslog messages, respond only with lines of text that start with ANOMALIES: and followed by lines of anomalous syslog messages. Syslog messages:\n` + strings.Join(messages, "\n "),
			},
		},
	}

	apiKey := config.apiKey
	url := config.url
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	var completionResponse CompletionResponse
	if err := json.Unmarshal(body, &completionResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	anomalyReport := "ANOMALIES:"
	anomalies := []string{}
	for _, choice := range completionResponse.Choices {
		idx := strings.Index(choice.Message.Content, anomalyReport)
		if idx == 0 {
			anomalies = strings.Split(choice.Message.Content[len(anomalyReport):], "\n")
			anomalies = removeEmptyStrings(anomalies)
			break
		}
	}

	return anomalies, nil
}
func cleanString(s string) string {
	s = strings.ReplaceAll(s, "<script>", "<XXX>")
	s = strings.ReplaceAll(s, "</script>", "</XXX>")
	return strings.TrimSpace(s)
}

func removeEmptyStrings(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}
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

type MessageRequest struct {
	Messages []string `json:"messages"`
}

func messagesHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			rows, err := renderMessageRows(handler)
			if err != nil {
				http.Error(w, "Error rendering message rows", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, rows)
		} else if r.Method == http.MethodPost {
			var reqBody MessageRequest
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			if err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			for _, msg := range reqBody.Messages {
				handler.logMessage(r.RemoteAddr, msg)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Syslog messages received"})
		} else {
			http.Error(w, "Only GET and POST methods are allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

func configHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Only GET or POST method is allowed", http.StatusMethodNotAllowed)
			return
		}
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}
		severity, _ := strconv.Atoi(r.FormValue("severity"))
		anomaliesOnly := r.FormValue("anomaliesOnly") == "on" // Parse anomaliesOnly checkbox
		maxMessages, _ := strconv.Atoi(r.FormValue("maxMessages"))
		defer r.Body.Close()
		config := handler.getConfig()
		config.AnomaliesOnly = anomaliesOnly
		config.MaxMessages = maxMessages
		config.AppName = r.FormValue("appname")
		config.HostName = r.FormValue("hostname")
		config.MessagePattern = r.FormValue("messagepattern")
		config.Severity = severity
		handler.updateConfig(config)
		w.WriteHeader(http.StatusOK)
	}
}

func renderPage(w http.ResponseWriter, page string, tmpl *template.Template,
	handler *logFileHandler) {
	w.Header().Set("Content-Type", "text/html")
	config := handler.getConfig()

	err := tmpl.ExecuteTemplate(w, page+".html", config)
	if err != nil {
		fmt.Printf("render template error %s %v\n", page, err)
		http.Error(w, "render template error", http.StatusInternalServerError)
	}
}

func main() {
	address := flag.String("a", ":514", "Syslog server address")
	logFile := flag.String("f", "", "Log file path")
	maxSize := flag.Int("m", 10, "Max log file size in MB")
	forwardAddr := flag.String("r", "", "Upstream syslog server address")
	forwardProto := flag.String("p", "udp", "Forwarding protocol: 'tcp' or 'udp'")
	forwardLevel := flag.Int("l", 6, "Forwarding priority level")
	apiAddr := flag.String("w", ":3001", "REST API and Web UI address")
	debuglog := flag.String("d", "", "debug log file")
	flag.Parse()

	if *debuglog != "" {
		f, err := os.OpenFile(*debuglog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Error opening debug log file: %v", err)
		}
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
		log.SetFlags(0)
	}
	
	logHandler, err := createLogFileHandler(*logFile, *maxSize, *forwardAddr, *forwardProto,
		*forwardLevel)
	if err != nil {
		log.Fatalf("Failed to create log handler: %v", err)
	}
	logHandler.config.ApiKey = os.Getenv("OPENAI_API_KEY")
	logHandler.config.Url = os.Getenv("OPENAI_API_URL")
	logHandler.config.Model = os.Getenv("OPENAI_MODEL")
	logHandler.config.LogFile = *logFile
	http.HandleFunc("/static/search.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "static/search.js")
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(embeddedFiles))))
	tmpl, err := template.ParseFS(embeddedFiles, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "logs", tmpl, logHandler)
	})
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "logs", tmpl, logHandler)
	})
	http.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "settings", tmpl, logHandler)
	})
	http.HandleFunc("/messages", messagesHandler(logHandler))
	http.HandleFunc("/config", configHandler(logHandler))

	go func() {
		fmt.Printf("Web UI and REST API listening on %s\n", *apiAddr)
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

	fmt.Printf("Syslog server listening on UDP %s\n", *address)

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
