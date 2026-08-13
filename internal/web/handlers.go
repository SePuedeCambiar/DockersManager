package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings" // Necesario para procesar los comandos de la consola

	"github.com/go-chi/chi/v5"
	"go-docker-manager/internal/docker"
)

type WebHandler struct {
	DockerMgr *docker.DockerManager
	Templates *template.Template
}

// NewWebHandler inicializa el manejador web y carga las plantillas HTML
func NewWebHandler(dm *docker.DockerManager) *WebHandler {
	// Cargamos todos los archivos .html de la carpeta templates
	tmpl := template.Must(template.ParseGlob("internal/web/templates/*.html"))
	return &WebHandler{
		DockerMgr: dm,
		Templates: tmpl,
	}
}

// Dashboard renderiza la página principal con la lista de contenedores
func (wh *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	containers, err := wh.DockerMgr.ListContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Renderizamos la plantilla "index.html" pasando la lista de contenedores
	wh.Templates.ExecuteTemplate(w, "index.html", containers)
}

// ControlContainer maneja el inicio, parada y reinicio
func (wh *WebHandler) ControlContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	action := r.URL.Query().Get("action")

	var err error
	switch action {
	case "start":
		err = wh.DockerMgr.StartContainer(id)
	case "stop":
		err = wh.DockerMgr.StopContainer(id)
	case "restart":
		err = wh.DockerMgr.RestartContainer(id)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Con HTMX, respondemos con un mensaje simple que el cliente puede ignorar o mostrar
	w.Write([]byte("✅ Acción ejecutada con éxito"))
}

// LogsHandler devuelve los logs en un formato HTML simple para que HTMX los inserte en la página
func (wh *WebHandler) LogsHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := wh.DockerMgr.GetLogs(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Devolvemos los logs envueltos en una etiqueta <pre> para mantener los saltos de línea y el formato de consola
	fmt.Fprintf(w, `<pre class="text-xs text-green-400 p-2 bg-black rounded border border-gray-700 overflow-auto max-h-64">%s</pre>`, logs)
}

// ExecHandler recibe un comando desde el input de la web y devuelve la respuesta del contenedor
func (wh *WebHandler) ExecHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	commandStr := r.URL.Query().Get("cmd")
	
	if commandStr == "" {
		fmt.Fprintf(w, `<p class="text-xs text-yellow-500 p-2">Por favor, escribe un comando.</p>`)
		return
	}

	// strings.Fields separa el comando por espacios (ej: "ls -la" -> ["ls", "-la"])
	cmd := strings.Fields(commandStr)

	output, err := wh.DockerMgr.ExecCommand(id, cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Si el comando no devolvió nada, avisamos al usuario
	if output == "" {
		output = "(El comando se ejecutó pero no devolvió ninguna salida)"
	}

	fmt.Fprintf(w, `<pre class="text-xs text-blue-300 p-2 bg-black rounded border border-gray-700 overflow-auto max-h-64">%s</pre>`, output)
}