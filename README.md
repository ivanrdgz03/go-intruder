# Go-Intruder: Fuzzer Web de Alto Rendimiento

**Go-Intruder** es una herramienta de fuerza bruta web (fuzzer) ultra rápida, escrita en Go. Diseñada como una alternativa ligera y eficiente al "Intruder" de Burp Suite, que es muy lento en la versión gratuita, permite realizar miles de peticiones por segundo utilizando concurrencia real con GoRoutines.

Soporta archivos de petición RAW, filtros de respuesta, salida en JSON detallado y seguimiento de redirecciones.

## Características Principales

* **Ultra Rápido:** Utiliza Goroutines para concurrencia masiva con bajo consumo de memoria.
* **Formato RAW:** Acepta peticiones copiadas directamente desde Burp Suite.
* **Marcador Flexible:** Usa `$$` para marcar el punto de inyección (URL, Headers, Body, Host).
* **Filtros Inteligentes:** Filtra por Código de Estado (`-fc`) y Tamaño (`-fs`) para eliminar ruido.
* **Reporte Detallado:** Opción de guardar respuestas completas (Headers + Body) en formato JSON.
* **Redirecciones:** Soporte opcional para seguir redirecciones (`-L`).
* **SSL/TLS:** Soporte para HTTPS.

## 🛠️ Instalación y Compilación

Necesitas tener [Go instalado](https://go.dev/dl/).

1. **Clonar o descargar el código** en un archivo llamado `intruder.go`.
2. **Compilar:**
   ```bash
   go build ./fuzzing.go
## Ejemplos Prácticos
1. Ataque básico (Login)
Fuzzeo simple marcando el password en req.txt.
    ```bash
    ./fuzzer -r request.txt -w rockyou.txt -t 20
2. Filtrando ruido
Eliminar todas las respuestas 404 y aquellas que pesen exactamente 340 bytes.
    ```bash
    ./fuzzer -r request.txt -w rutas.txt -fc 404 -fs 340
3. Guardando evidencia completa
Guardar todas las respuestas (incluyendo el HTML) en un archivo JSON para análisis posterior.
    ```bash
    ./fuzzer -r request.txt -w usuarios.txt -o resultados.json
4. Fuzzing de Subdominios (Virtual Host)
Si colocas el marcador $$ en el header Host dentro de request.txt.
    ```bash
    ./fuzzer -r vhost_req.txt -w subdominios.txt -t 50
## Formato del Archivo Request (-r)
Copia la petición desde tu proxy (Burp Suite, Caido, ZAP) y guárdala en un archivo .txt. Reemplaza el valor que quieres atacar con $$.

Ejemplo req.txt:
    
    HTTP

    POST /api/login HTTP/1.1
    Host: ejemplo.com
    User-Agent: Mozilla/5.0
    Content-Type: application/json
    Content-Length: 45

    {"username": "admin", "password": "$$"}
### Argumentos y Opciones

| Flag | Descripción | Requerido | Ejemplo |
| :--- | :--- | :--- | :--- |
| `-r` | Ruta al archivo con la petición RAW (http). | **Sí** | `-r request.txt` |
| `-w` | Ruta al archivo de diccionario (wordlist). | **Sí** | `-w passwords.txt` |
| `-t` | Número de hilos concurrentes (Workers). | No (Def: 10) | `-t 50` |
| `-d` | Delay (retraso) entre peticiones en ms. | No (Def: 0) | `-d 200` |
| `-o` | Guarda los resultados detallados en un archivo JSON. | No | `-o output.json` |
| `-ssl` | Fuerza la conexión por HTTPS (SSL/TLS). | No | `-ssl` |
| `-L` | Sigue las redirecciones (301, 302, etc.). | No | `-L` |
| `-fc` | Filtra (oculta) códigos de estado específicos (sep. por comas). | No | `-fc 404,403` |
| `-fs` | Filtra (oculta) respuestas por tamaño en bytes (sep. por comas). | No | `-fs 1250` |