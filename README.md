# Syslog Server and Client

This repository contains a syslog server and a syslog client program written Go.

The syslog server can accept syslog messages over UDP or TCP, and forward them to an upstream syslog server. 
It can store received messages in compressed rotating files and supports anomaly detection via LLM
which can be specified in environment variables: OPENAI_API_KEY, OPENAI_API_URL, and OPENAI_MODEL before running
the syslog_server.go. Any OpenAI compatible API can work, including locally hosted Ollama. 
There is a builtin web UI to view and filter the incoming logs. REST API can accept log messages
in addition to UDP or TCP syslog input. 

The syslog client can send syslog messages over TCP and UDP, similar to `logger` command on Linux. 
Additionally, the flag `-i` can be used to pass in a file that has
many lines of syslog messages.

<img width="1988" alt="Screenshot 2024-10-31 at 11 40 23 AM" src="https://github.com/user-attachments/assets/cecf7b43-91b9-4b88-8211-d68e278835e3">
