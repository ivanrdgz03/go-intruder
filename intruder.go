package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Colores
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

type ScanResult struct {
	Timestamp string              `json:"timestamp"`
	Payload   string              `json:"payload"`
	Status    int                 `json:"status"`
	Size      int                 `json:"size"`
	Duration  string              `json:"duration_ms"`
	URL       string              `json:"final_url"`        // URL final (útil tras redirecciones)
	Original  string              `json:"original_target"`  // URL a la que atacamos
	Headers   map[string][]string `json:"response_headers"`
	Body      string              `json:"response_body"`
}

type RequestTemplate struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Host    string
}

type Filters struct {
	IgnoreCodes []int
	IgnoreSize  []int
}

func main() {
	// --- FLAGS ---
	wordlistPtr := flag.String("w", "", "Diccionario")
	reqFilePtr := flag.String("r", "", "Archivo Request RAW")
	outputFilePtr := flag.String("o", "", "Archivo de salida JSON (Opcional)")
	threadsPtr := flag.Int("t", 10, "Hilos")
	delayPtr := flag.Int("d", 0, "Delay (ms)")
	httpsPtr := flag.Bool("ssl", false, "Usar HTTPS")
	followPtr := flag.Bool("L", false, "Seguir redirecciones (3xx)") // NUEVO FLAG

	// Filtros
	filterCodePtr := flag.String("fc", "", "Filtrar status codes (ej: 404,403)")
	filterSizePtr := flag.String("fs", "", "Filtrar por tamaño (ej: 1250)")

	flag.Parse()

	if *wordlistPtr == "" || *reqFilePtr == "" {
		fmt.Println(ColorRed + "[!] Falta -w o -r" + ColorReset)
		os.Exit(1)
	}

	// --- SETUP OUTPUT ---
	var fileOutput *os.File
	var err error
	var fileMutex sync.Mutex

	if *outputFilePtr != "" {
		fileOutput, err = os.Create(*outputFilePtr)
		if err != nil {
			fmt.Printf(ColorRed+"[!] Error creando archivo: %v\n"+ColorReset, err)
			return
		}
		defer fileOutput.Close()
	}

	// --- SETUP FILTROS Y REQUEST ---
	filters := Filters{
		IgnoreCodes: parseIntList(*filterCodePtr),
		IgnoreSize:  parseIntList(*filterSizePtr),
	}

	template, err := parseRawRequest(*reqFilePtr, *httpsPtr)
	if err != nil {
		fmt.Printf(ColorRed+"[!] Error leyendo request: %v\n"+ColorReset, err)
		return
	}

	wordlistFile, err := os.Open(*wordlistPtr)
	if err != nil {
		fmt.Printf(ColorRed+"[!] Error wordlist: %v\n"+ColorReset, err)
		return
	}
	defer wordlistFile.Close()

	payloads := make(chan string)
	var wg sync.WaitGroup

	// --- CONFIGURACIÓN DEL CLIENTE HTTP ---
	
	// Por defecto: NO seguir redirecciones
	redirectFunc := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Si el usuario pone -L, usamos nil (Go sigue redirecciones por defecto)
	if *followPtr {
		redirectFunc = nil 
	}

	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: redirectFunc, // Aplicamos la política elegida
	}

	fmt.Printf(ColorCyan+"[*] Target: %s\n"+ColorReset, template.URL)
	if *followPtr {
		fmt.Println(ColorYellow + "[!] Modo Follow Redirects: ACTIVADO" + ColorReset)
	}
	start := time.Now()

	// --- WORKERS ---
	for i := 0; i < *threadsPtr; i++ {
		wg.Add(1)
		go worker(client, template, payloads, &wg, *delayPtr, filters, fileOutput, &fileMutex)
	}

	scanner := bufio.NewScanner(wordlistFile)
	for scanner.Scan() {
		payloads <- scanner.Text()
	}
	close(payloads)

	wg.Wait()
	fmt.Printf("\n"+ColorGreen+"[OK] Completado en: %v\n"+ColorReset, time.Since(start))
}

func worker(client *http.Client, tmpl RequestTemplate, jobs <-chan string, wg *sync.WaitGroup, delay int, filters Filters, fOut *os.File, mu *sync.Mutex) {
	defer wg.Done()

	for payload := range jobs {
		if delay > 0 { time.Sleep(time.Duration(delay) * time.Millisecond) }

		targetURL := strings.ReplaceAll(tmpl.URL, "$$", payload)
		bodyData := strings.ReplaceAll(tmpl.Body, "$$", payload)

		req, err := http.NewRequest(tmpl.Method, targetURL, strings.NewReader(bodyData))
		if err != nil { continue }

		for k, v := range tmpl.Headers {
			req.Header.Set(k, strings.ReplaceAll(v, "$$", payload))
		}

		reqStart := time.Now()
		resp, err := client.Do(req)
		if err != nil { continue }
		
		duration := time.Since(reqStart)
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		size := len(bodyBytes)
		bodyString := string(bodyBytes)

		// --- FILTROS ---
		skip := false
		for _, code := range filters.IgnoreCodes {
			if resp.StatusCode == code { skip = true; break }
		}
		if skip { continue }
		for _, s := range filters.IgnoreSize {
			if size == s { skip = true; break }
		}
		if skip { continue }

		// --- IMPRESIÓN ---
		statusColor := ColorReset
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			statusColor = ColorGreen
		} else if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			statusColor = ColorCyan
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			statusColor = ColorYellow
		} else {
			statusColor = ColorRed
		}

		fmt.Printf("[+] Payload: %-15s | Status: %s%d%s | Size: %d\n", 
			payload, statusColor, resp.StatusCode, ColorReset, size)

		// --- GUARDADO JSON ---
		if fOut != nil {
			result := ScanResult{
				Timestamp: time.Now().Format(time.RFC3339),
				Payload:   payload,
				Status:    resp.StatusCode,
				Size:      size,
				Duration:  fmt.Sprintf("%d", duration.Milliseconds()),
				URL:       resp.Request.URL.String(), // URL Final (puede ser distinta tras redirect)
				Original:  targetURL,                 // URL Original
				Headers:   resp.Header,
				Body:      bodyString,
			}

			jsonLine, err := json.Marshal(result)
			if err == nil {
				mu.Lock()
				fOut.Write(jsonLine)
				fOut.WriteString("\n")
				mu.Unlock()
			}
		}
	}
}

// --- HELPERS (Sin cambios) ---
func parseIntList(s string) []int {
	var nums []int
	if s == "" { return nums }
	parts := strings.Split(s, ",")
	for _, p := range parts {
		i, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil { nums = append(nums, i) }
	}
	return nums
}

func parseRawRequest(path string, useHTTPS bool) (RequestTemplate, error) {
	var tmpl RequestTemplate
	tmpl.Headers = make(map[string]string)
	content, err := os.ReadFile(path)
	if err != nil { return tmpl, err }
	raw := string(content)
	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) < 2 { parts = strings.SplitN(raw, "\n\n", 2) }
	headerPart := parts[0]
	if len(parts) > 1 { tmpl.Body = strings.TrimSpace(parts[1]) }
	lines := strings.Split(headerPart, "\n")
	firstLine := strings.Fields(lines[0])
	if len(firstLine) < 2 { return tmpl, fmt.Errorf("bad request format") }
	tmpl.Method = firstLine[0]
	tmpl.URL = strings.TrimSpace(firstLine[1]) 
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" { continue }
		headerParts := strings.SplitN(line, ":", 2)
		if len(headerParts) == 2 {
			k := strings.TrimSpace(headerParts[0])
			v := strings.TrimSpace(headerParts[1])
			tmpl.Headers[k] = v
			if strings.EqualFold(k, "host") { tmpl.Host = v }
		}
	}
	protocol := "http"
	if useHTTPS { protocol = "https" }
	tmpl.URL = fmt.Sprintf("%s://%s%s", protocol, tmpl.Host, tmpl.URL)
	return tmpl, nil
}