# Syslog Server and Client

The syslog_server.go can 

- accept syslog messages over UDP or TCP
- forward logs to an upstream server. 
- store logs in compressed rotating files. 
- detect anomalies
- support any Open AI API compatible LLM 
- view & filter logs via web UI
- support REST API

The syslog_client.go can 

- send syslog messages over TCP and UDP
- send logs from a file

<img width="1988" alt="Screenshot 2024-10-31 at 11 40 23 AM" src="https://github.com/user-attachments/assets/cecf7b43-91b9-4b88-8211-d68e278835e3">
