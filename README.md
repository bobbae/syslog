# Syslog Server and Client

This repository contains a syslog server and client program written Go.
The syslog server can accept syslog messages over UDP or TCP, and forward them to an upstream Syslog server. 
It can store received messages in compressed rotating files and supports anomaly detection via OpenAI-compatible LLM.
It has a builtin web UI to view and filter the incoming logs.
A syslog client can send syslog messages over TCP and UDP.

<img width="1988" alt="Screenshot 2024-10-31 at 11 40 23 AM" src="https://github.com/user-attachments/assets/cecf7b43-91b9-4b88-8211-d68e278835e3">
