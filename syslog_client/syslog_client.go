package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// Command-line flags
	protocol := flag.String("p", "udp", "Protocol to use: 'udp' or 'tcp'")
	address := flag.String("a", "127.0.0.1:514", "Address of the syslog server")
	facility := flag.Int("f", 1, "Syslog facility level (0 to 23)")
	severity := flag.Int("s", 6, "Syslog severity level (0 to 7)")
	host := flag.String("h", "localhost", "Host name")
	app := flag.String("n", "syslog_client", "Application name")
	message := flag.String("m", "Test syslog message", "The message to send")
	inputFile := flag.String("i", "", "Input file containing syslog messages")
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

	// Validate priority level
	if *facility < 0 || *facility > 23 {
		log.Fatalf("Invalid facility level: %d. Must be between 0 and 23.", *facility)
	}

	if *severity < 0 || *severity > 7 {
		log.Fatalf("Invalid severity level: %d. Must be between 0 and 7.", *severity)
	}

	// Check if input file is provided
	if *inputFile != "" {
		sendMessagesFromFile(*inputFile, *protocol, *address, *facility)
	} else {
		// Create the syslog message with a timestamp and priority level
		syslogMessage := formatSyslogMessage(*facility*8+*severity, *host, *app, *message)

		// Send the message based on the chosen protocol
		switch strings.ToLower(*protocol) {
		case "udp":
			sendUDPMessage(*address, syslogMessage)
		case "tcp":
			sendTCPMessage(*address, syslogMessage)
		default:
			log.Fatalf("Unsupported protocol: %s. Use 'udp' or 'tcp'.", *protocol)
		}
	}
}

// formatSyslogMessage creates a syslog message with priority, timestamp, and message body.
func formatSyslogMessage(priority int, host string, app string, message string) string {
	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("<%d>%s %s %s", priority, timestamp, host, app+": "+message)
}

// sendUDPMessage sends a syslog message over UDP.
func sendUDPMessage(address, message string) {
	conn, err := net.Dial("udp", address)
	if err != nil {
		log.Fatalf("Error connecting to UDP server: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Fatalf("Error sending UDP message: %v", err)
	}

	log.Printf("Sent UDP message to %s: %s", address, message)
}

// sendTCPMessage sends a syslog message over TCP.
func sendTCPMessage(address, message string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Error connecting to TCP server: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message + "\n"))
	if err != nil {
		log.Fatalf("Error sending TCP message: %v", err)
	}

	log.Printf("Sent TCP message to %s: %s", address, message)
}

// sendMessagesFromFile reads syslog messages from a file and sends them.
func sendMessagesFromFile(filename, protocol, address string, facility int) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		syslogMessage := parseSyslogLine(line, facility)

		switch strings.ToLower(protocol) {
		case "udp":
			sendUDPMessage(address, syslogMessage)
		case "tcp":
			sendTCPMessage(address, syslogMessage)
		default:
			log.Fatalf("Unsupported protocol: %s. Use 'udp' or 'tcp'.", protocol)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
}

// parseSyslogLine parses a line from the input file and formats it as a syslog message.
func parseSyslogLine(line string, facility int) string {
	parts := strings.SplitN(line, " ", 6)
	if len(parts) < 6 {
		log.Printf("Error: Invalid syslog line format: %s", line)
		return ""
	}
	log.Printf("Received syslog message: %v|%v|%v|%v|%v|%v", parts[0], parts[1], parts[2], parts[3], parts[4], parts[5])
	date := parts[0] + " " + parts[1] + " " + parts[2]
	host := parts[3]
	app := parts[4]
	app = strings.TrimSuffix(app, ":")
	severityStr := "info"
	if strings.HasPrefix(parts[5], "[DEBUG]") {
		severityStr = "debug"
	} else if strings.HasPrefix(parts[5], "[WARNING]") {
		severityStr = "warning"
	} else if strings.HasPrefix(parts[5], "[ERROR]") {
		severityStr = "err"
	} else if strings.HasPrefix(parts[5], "[CRITICAL]") {
		severityStr = "crit"
	} else if strings.HasPrefix(parts[5], "[ALERT]") {
		severityStr = "alert"
	} else if strings.HasPrefix(parts[5], "[EMERG]") {
		severityStr = "emerg"
	}

	message := parts[5]
	log.Printf("Parsed syslog message: date %s host %s app %s severity %s message %s", date, host, app, severityStr, message)
	severity := parseSeverity(severityStr)
	priority := facility*8 + severity

	return fmt.Sprintf("<%d>%s %s %s: %s", priority, date, host, app, message)
}

// parseSeverity converts severity string to integer.
func parseSeverity(severityStr string) int {
	switch strings.ToLower(severityStr) {
	case "emerg":
		return 0
	case "alert":
		return 1
	case "crit":
		return 2
	case "err":
		return 3
	case "warning":
		return 4
	case "notice":
		return 5
	case "info":
		return 6
	case "debug":
		return 7
	default:
		log.Printf("Unknown severity: %s. Defaulting to 'info' (6)", severityStr)
		return 6
	}
}
