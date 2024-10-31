# Syslog Server and Client

This repository contains two Go programs:

* `syslog_server.go`: A syslog server and web UI. Able to forward to upstream syslog server. Rotating compressed syslog file storage. Anomaly detection via OpenAI compatible LLM.
* `syslog_client.go`: A syslog client to send over TCP and UDP.

<img width="1992" alt="screenshot" src="https://github.com/user-attachments/assets/11611a69-2159-4561-9097-4268adb1d137">
