# Syslog Server and Client

This repository contains two Go programs:

* `syslog_server.go`: A syslog server and web UI. Able to forward to upstream syslog server. Rotating compressed syslog file storage. Anomaly detection via OpenAI compatible LLM.
* `syslog_client.go`: A syslog client to send over TCP and UDP.

<img width="1988" alt="Screenshot 2024-10-31 at 11 40 23 AM" src="https://github.com/user-attachments/assets/cecf7b43-91b9-4b88-8211-d68e278835e3">
