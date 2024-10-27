# Syslog Server and Client

This repository contains two Go programs:

1. `syslog_server.go`: A syslog server that listens for logs over both UDP and TCP protocols, supports log forwarding, message storage, and a web-based UI for viewing and managing logs.
2. `syslog_client.go`: A syslog client that allows users to send log messages to a syslog server using both UDP and TCP protocols, supports standard syslog message formats, and provides customizable options.

# Syslog Server 

## Overview
The `syslog_server.go` implements a **syslog server** that listens for logs over both **UDP** and **TCP** protocols. It supports **log forwarding**, message storage, and a **web-based UI** for viewing and managing logs. The server is designed to handle real-time log collection efficiently and provides configurable options through command-line flags.

---

## Features

### 1. Multi-Protocol Support
- Listens for incoming syslog messages over:
  - **UDP**: For fast, connectionless log transmission.
  - **TCP**: For reliable, connection-oriented log delivery.

### 2. Log Forwarding
- Can forward logs to an **upstream syslog server** using:
  - **UDP or TCP** protocol.
  - Configurable **forwarding priority level**.

### 3. Web UI with Pico.css and HTMX
<img width="1734" alt="screenshot" src="https://github.com/user-attachments/assets/9a791e51-ee19-4955-919a-1f1210212916">
- A **web-based user interface** to view and manage logs.
- Everything included in one file and binary. Easy to deploy.
- Runs at port 8080 by default (can be changed via -api flag).
  
### 4. Message Filtering and Search
- Web interface allows:
  - **Search functionality** for filtering messages.
  - **Automatic refreshing** every 5 seconds to display new messages.

### 5. REST API Endpoints
- **`GET /messages`**: Retrieve all stored messages.
- **`POST /syslog`**: Submit a syslog message via HTTP.
- **`POST /clear`**: Clear all stored messages.
- **`GET /stats`**: Get server statistics in JSON format.

### 6. Configurable Log File Storage
- Stores logs in compressed **rotating files**.
  - Supports **max file size**, **backups**, and **compression**.
- Option to **disable logging to file** if needed.

### 7. Persistent Upstream Connection
- Keeps a **persistent connection** to the upstream syslog server.
- **Automatic reconnection** on failure.

### 8. Real-Time Statistics
- Tracks and displays:
  - **Logs received** and **logs forwarded**.
  - Available via the **`/stats`** REST endpoint.

### 9. Concurrency and Thread Safety
- Uses **mutex locks** to ensure thread-safe access to shared data.
- **Goroutines** for handling multiple clients and forwarding messages asynchronously.

---

## Examples


- send message to syslog_server using HTTP POST
```bash
curl -X POST -d "<16>Oct 17 14:32:00 myhost myapp: my message" http://localhost:8080/syslog 
```

# Syslog Client - Features

## Overview
This `syslog_client.go` allows users to send log messages to a syslog server using both **TCP** and **UDP** protocols. It supports standard syslog message formats and provides configurable options through command-line flags.

---

## Features

### 1. Send Log Messages to Syslog Server
- Supports both **UDP** and **TCP** protocols.
- Can send messages to **local or remote syslog servers**.

### 2. Configurable Priority and Facility
- Users can specify **syslog priority** (e.g., `<13>` representing severity and facility).
- Allows users to adjust severity levels (e.g., `INFO`, `ERROR`, `DEBUG`).

### 3. Log Message Formatting
- Compliant with **RFC 3164** or **RFC 5424** standards.
- Includes metadata such as:
  - **Timestamp**
  - **Hostname**
  - **App Name**
  - **Message Body**

## Examples

- Customizable options for flexibility:
  
  ```bash
  go run syslog_client.go -server "localhost:514" -proto udp -priority 13 -app "myapp" "Test log message"
```
- Sending messages from a file:
```bash
go run syslog_client.go -inputfile input.txt
```

---

