package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type SyslogMessages struct {
	Messages []string `json:"messages"`
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

func findAnomalies(config LLMConfig, messages []string) ([]string, error) {

	requestBody := CompletionRequest{
		Model: config.model,
		Messages: []Message{
			{
				Role: "user",
				Content: `You are a syslog anomaly detection system. 
							Please carefully analyze the following syslog messages and
							carefully reason about each message to determine whether
							a message is anomalous or not. Response should only
							be the list of syslog messages that are anomalous. Response 
							format should be in text format with each message on a new line.
							Syslog messages:\n\n` +
						strings.Join(messages, "\n "),
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
		fmt.Println("Error unmarshalling JSON:", err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	anomalyReport := "Here is the list of anomalous syslog messages:"
	anomalies := []string{}
	for _, choice := range completionResponse.Choices {
		idx := strings.Index(choice.Message.Content, anomalyReport)
		if idx == 0 {
			anomalies = strings.Split(choice.Message.Content[len(anomalyReport):], "\n")
			anomalies = removeEmptyStrings(anomalies)
		}
	}

	return anomalies, nil
}

func main() {
	apiKeyPtr := flag.String("apikey", "", "API key")
	inputFilePtr := flag.String("inputfile", "", "Path to the syslog file")
	urlPtr := flag.String("url", "https://api.openai.com/v1/chat/completions", "API endpoint URL")
	modelPtr := flag.String("model", "gpt-3.5-turbo", "model name")
	flag.Parse()

	if *inputFilePtr == "" {
		log.Fatal("Please provide an input file using the -inputfile flag.")
	}

	fileContent, err := os.ReadFile(*inputFilePtr)
	if err != nil {
		log.Fatalf("Error reading input file: %v", err)
	}

	messages := strings.Split(string(fileContent), "\n")
	messages = removeEmptyStrings(messages)
	config := LLMConfig{apiKey: *apiKeyPtr, url: *urlPtr, model: *modelPtr}
	anomalies, err := findAnomalies(config, messages)
	if err != nil {
		log.Fatalf("Error analyzing syslog messages: %v", err)
	}
	fmt.Println("Anomalies:")
	for _, anomaly := range anomalies {
		fmt.Println(anomaly)
	}
	fmt.Println("Total number of anomalies", len(anomalies))
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
