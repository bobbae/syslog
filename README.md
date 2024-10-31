# Syslog Server and Client

This repository contains two Go programs:

* `syslog_server.go`: A syslog server and web UI. Able to forward to upstream syslog server. Rotating compressed syslog file storage.
* `syslog_client.go`: A syslog client to send over TCP and UDP.
