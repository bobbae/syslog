# Syslog Server and Client

This repository contains two Go programs:

1. `syslog_server.go`: A syslog server and web UI
2. `syslog_client.go`: A syslog client 
3. 
## Building and running

```bash
go build -o syslog_server/syslog_server syslog_server/syslog_server.go
./syslog_server/syslog_server -h
go build -o syslog_client/syslog_client syslog_client/syslog_client.go
./syslog_client/syslog_client -h
```

## Syslog Server 


### 1. Multi-Protocol Support
- Listens for incoming syslog messages over UDP and TCP.

### 2. Log Forwarding
- Can forward logs to an upstream syslog server using UDP or TCP.
- Configurable forwarding priority level.
- You can chain multiple syslog servers together to create a log forwarding pipeline.

### 3. Web UI 

<img width="1734" alt="screenshot" src="https://github.com/user-attachments/assets/67d998c9-998b-47bc-9683-844f0018adb6">

- A builtin web UI to view and manage logs.
- Everything is included in one binary that can be copied and deployed easily.
  
### 4. Message Filtering and Search
- Web interface allows:
  - Search functionality for filtering messages.
  - Automatic refreshing every 5 seconds to display new messages.

### 5. REST API Endpoints
- `GET /messages`: Retrieve all stored messages.
- `POST /syslog`: Submit a syslog message via HTTP.
- `POST /clear`: Clear all stored messages.

### 6. Configurable Log File Storage
- Stores logs in compressed rotating log files.
- Option to disable logging to file.



## Syslog Client 



### 1. Send Log Messages to Syslog Server
- Supports both UDP and TCP
- Can send messages to any IP:PORT where a syslog server is running

### 2. Configurable Priority and Facility
- Users can specify syslog priority (e.g., `<16>` representing severity and facility).

### 3. Log Message Formatting
- Compliant with RFC 3164
- Includes metadata such as: Timestamp, Host, App name, Message body


## Examples

- syslog_client to send syslog to a syslog server:
```bash
  go run syslog_client.go -server "localhost:514" -proto udp -priority 13 -app "myapp" "Test log message"
```

- Read each line from  a file and send it as a syslog message:
```bash
go run syslog_client.go -inputfile input.txt
```

- send message to the syslog_server using HTTP POST via `curl`
```bash
curl -X POST -d "<16>Oct 17 14:32:00 myhost myapp: my message" http://localhost:8080/syslog 
```

- sending message to a syslog server using `logger` command on Linux
```bash
logger --rfc3164 -d -n localhost -P 514 "myapp: my message"
```

- sending message to a syslog server using `nc` command on Linux
```bash
echo '<16>Oct 17 14:32:00 myhost myapp: my message' | nc -u localhost 514
```

