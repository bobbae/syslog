package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSyslogServer(t *testing.T) {
	// Start the syslog server with a specific number of lines per file
	cmd := exec.Command("go", "run", "syslog_server.go", "-file", "syslog.log",
		"-addr", ":514", "-buf", "1024", "-maxsize", "1", "-debug", "debug.log")
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	ssURL := "http://localhost:8080"

	// Wait for the server to start
	//time.Sleep(1 * time.Second)

	log.Printf("waiting for syslog service to start")
	for i := 0; i < 10; i++ {
		isUp, _ := isServiceUp(ssURL)
		if isUp {
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Printf("syslog service started, pid %d", cmd.Process.Pid)

	removeExistingSyslogFiles()

	// Send a test message to the server
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:514")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	log.Printf("sending syslog messages")
	for i := 0; i < 30000; i++ {
		msg := []byte(fmt.Sprintf("<16>Jan  1 00:00:00 localhost Test message %d", i))
		_, err = conn.Write(msg)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Verify that the log file was rotated correctly
	time.Sleep(1 * time.Second)
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	rotatedFile := ""
	for _, file := range files {
		fn := file.Name()
		log.Printf("checking file %s", fn)
		if strings.HasPrefix(fn, "syslog-") && strings.HasSuffix(fn, ".log.gz") {
			log.Printf("found rotated file %s", fn)
			rotatedFile = fn
			break
		}
	}
	if rotatedFile == "" {
		t.Errorf("Expected rotated log file not found")
	}
}
func removeExistingSyslogFiles() error {
	files, err := os.ReadDir(".")
	if err != nil {
		return err
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "syslog.log") ||
			(strings.HasPrefix(file.Name(), "syslog-") &&
				strings.HasSuffix(file.Name(), "log.gz")) {
			err := os.Remove(file.Name())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func isServiceUp(url string) (bool, error) {
	req, err := http.NewRequest("POST", url+"/echo", strings.NewReader(`{"key": "value"}`))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("service is down (status code %d)", resp.StatusCode)
	}

	return true, nil
}
