package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// Command-line flags
	protocol := flag.String("proto", "udp", "Protocol to use: 'udp' or 'tcp'")
	address := flag.String("addr", "127.0.0.1:514", "Address of the syslog server")
	facility := flag.Int("facility", 1, "Syslog facility level (0 to 23)")
	severity := flag.Int("severity", 6, "Syslog severity level (0 to 7)")
	app := flag.String("app", "syslog_client", "Application name")
	message := flag.String("msg", "Test syslog message", "The message to send")
	inputFile := flag.String("inputfile", "", "Input file containing syslog messages")
	flag.Parse()

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
		syslogMessage := formatSyslogMessage(*facility*8+*severity, *app, *message)

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
func formatSyslogMessage(priority int, app string, message string) string {
	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("<%d>%s %s", priority, timestamp, app+": "+message)
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
	parts := strings.SplitN(line, " ", 5)
	if len(parts) < 5 {
		log.Printf("Invalid syslog line format: %s", line)
		return ""
	}

	date := parts[0]
	host := parts[1]
	app := parts[2]
	severityStr := parts[3]
	message := parts[4]

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
