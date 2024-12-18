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
				Content: `	Given a list of syslog messages, respond only with lines of text
							that start with ANOMALIES: and followed by lines of anomalous syslog messages.
							Syslog messages:\n` + strings.Join(messages, "\n "),
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
	fmt.Println("anomalies", len(anomalies))

	return anomalies, nil
}

func main() {
	inputFilePtr := flag.String("i", "", "Path to the syslog file")

	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	url := os.Getenv("OPENAI_API_URL")
	model := os.Getenv("OPENAI_MODEL")

	if apiKey == "" {
		log.Fatal("Please provide an API key using env var OPENAI_API_KEY")
	}
	if url == "" {
		url = "https://api.openai.com/v1/chat/completions"
	}
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	if *inputFilePtr == "" {
		log.Fatal("Please provide an input file using the -i flag.")
	}

	fileContent, err := os.ReadFile(*inputFilePtr)
	if err != nil {
		log.Fatalf("Error reading input file: %v", err)
	}

	messages := strings.Split(string(fileContent), "\n")
	messages = removeEmptyStrings(messages)
	config := LLMConfig{apiKey: apiKey, url: url, model: model}
	anomalies, err := findAnomalies(config, messages)
	if err != nil {
		log.Fatalf("Error analyzing syslog messages: %v", err)
	}
	for _, anomaly := range anomalies {
		fmt.Println(anomaly)
	}
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
