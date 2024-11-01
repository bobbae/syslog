package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/natefinch/lumberjack"
)

//go:embed templates/*
//go:embed static/*
var embeddedFiles embed.FS

// ... (rest of the code remains the same until configHandler) ...

func configHandler(handler *logFileHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(embeddedFiles, "templates/config_form.html")
		if err != nil {
			http.Error(w, "Failed to parse template", http.StatusInternalServerError)
			return
		}

		data := struct {
			Config *Config
		}{
			Config: handler.getConfig(),
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			err := tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, "Failed to execute template", http.StatusInternalServerError)
				return
			}
			return
		}
		// ... (rest of the configHandler remains the same) ...
	}
}

// ... (rest of the code remains the same) ...
