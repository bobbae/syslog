package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// Priority levels based on RFC 5424 (e.g., 0 = Emergency, 1 = Alert, etc.)
var levels = map[int]string{
	0: "EMERGENCY",
	1: "ALERT",
	2: "CRITICAL",
	3: "ERROR",
	4: "WARNING",
	5: "NOTICE",
	6: "INFO",
	7: "DEBUG",
}

func main() {
	// Command-line flags
	protocol := flag.String("proto", "udp", "Protocol to use: 'udp' or 'tcp'")
	address := flag.String("addr", "127.0.0.1:514", "Address of the syslog server")
	priority := flag.Int("priority", 1, "Syslog priority level (0 to 7)")
	message := flag.String("msg", "Test syslog message", "The message to send")
	flag.Parse()

	// Validate priority level
	if *priority < 0 || *priority > 7 {
		log.Fatalf("Invalid priority level: %d. Must be between 0 and 7.", *priority)
	}

	// Create the syslog message with a timestamp and priority level
	syslogMessage := formatSyslogMessage(*priority, *message)

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

// formatSyslogMessage creates a syslog message with priority, timestamp, and message body.
func formatSyslogMessage(priority int, message string) string {
	timestamp := time.Now().Format(time.RFC3339)
	level := levels[priority] // Get the level name from the levels map

	return fmt.Sprintf("<%d>%s %s", priority, timestamp, level+": "+message)
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
