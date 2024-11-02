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

