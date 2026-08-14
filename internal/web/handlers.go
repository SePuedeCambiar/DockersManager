package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go-docker-manager/internal/docker"
)

type WebHandler struct {
	DockerMgr *docker.DockerManager
	Templates *template.Template
}

// NewWebHandler inicializa el manejador web y carga las plantillas HTML
func NewWebHandler(dm *docker.DockerManager) *WebHandler {
	tmpl := template.Must(template.ParseGlob("internal/web/templates/*.html"))
	return &WebHandler{
		DockerMgr: dm,
		Templates: tmpl,
	}
}

// =============================================================================
// 📦 GESTIÓN DE CONTENEDORES
// =============================================================================

// Dashboard renderiza la página principal con la lista de contenedores
func (wh *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	containers, err := wh.DockerMgr.ListContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	w.Write([]byte("✅ Acción ejecutada con éxito"))
}

// LogsHandler devuelve los logs en un formato HTML simple
func (wh *WebHandler) LogsHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := wh.DockerMgr.GetLogs(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `<pre class="text-xs text-green-400 p-2 bg-black rounded border border-gray-700 overflow-auto max-h-64">%s</pre>`, logs)
}

// ExecHandler ejecuta un comando en el contenedor y devuelve la respuesta
func (wh *WebHandler) ExecHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	commandStr := r.URL.Query().Get("cmd")
	
	if commandStr == "" {
		fmt.Fprintf(w, `<p class="text-xs text-yellow-500 p-2">Por favor, escribe un comando.</p>`)
		return
	}

	cmd := strings.Fields(commandStr)
	output, err := wh.DockerMgr.ExecCommand(id, cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if output == "" {
		output = "(El comando se ejecutó pero no devolvió ninguna salida)"
	}
	fmt.Fprintf(w, `<pre class="text-xs text-blue-300 p-2 bg-black rounded border border-gray-700 overflow-auto max-h-64">%s</pre>`, output)
}

// =============================================================================
// 📁 GESTIÓN DE ARCHIVOS (FASE 2)
// =============================================================================

// ListFilesHandler lista los archivos dentro de la carpeta del proyecto
func (wh *WebHandler) ListFilesHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entries, err := wh.DockerMgr.FS.ListDir(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al listar archivos: %v", err), http.StatusInternalServerError)
		return
	}

	html := fmt.Sprintf(`
		<div class="flex flex-col space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-purple-400 font-bold">📁 Archivos de %s</h3>
				<button hx-get="/details/%s" hx-target="#details-panel" class="text-xs bg-gray-600 px-2 py-1 rounded">Volver a Logs</button>
			</div>
			<div class="grid grid-cols-1 gap-2">
	`, id, id)

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Ignoramos carpetas por ahora
		}
		name := entry.Name()
		html += fmt.Sprintf(`
			<button hx-get="/edit/%s?file=%s" hx-target="#details-panel" 
				class="text-left p-2 bg-gray-700 hover:bg-gray-600 rounded text-xs border border-gray-600 flex justify-between transition-colors">
				<span>📄 %s</span>
				<span class="text-gray-400">Editar →</span>
			</button>
		`, id, name, name)
	}

	html += `</div></div>`
	fmt.Fprint(w, html)
}

// GetFileHandler lee un archivo y devuelve un formulario de edición
func (wh *WebHandler) GetFileHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fileName := r.URL.Query().Get("file")

	if fileName == "" {
		http.Error(w, "Nombre de archivo no proporcionado", http.StatusBadRequest)
		return
	}

	filePath := fmt.Sprintf("%s/%s", id, fileName)
	content, err := wh.DockerMgr.FS.ReadFile(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al leer archivo: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `
		<div class="flex flex-col space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-blue-400 font-bold">📄 Editando: %s</h3>
				<button hx-get="/files/%s" hx-target="#details-panel" class="text-xs bg-gray-600 px-2 py-1 rounded">Volver a Lista</button>
			</div>
			<form hx-post="/edit/%s?file=%s" hx-target="#details-panel" hx-swap="outerHTML">
				<textarea name="content" class="w-full h-64 p-2 bg-black text-green-400 font-mono text-xs border border-gray-700 rounded focus:outline-none focus:border-blue-500" spellcheck="false">%s</textarea>
				<button type="submit" class="mt-2 bg-blue-600 hover:bg-blue-500 px-4 py-2 rounded text-sm font-bold w-full transition-colors">💾 Guardar Cambios</button>
			</form>
		</div>
	`, fileName, id, id, fileName, content)
}

// SaveFileHandler guarda el contenido del archivo en el disco
func (wh *WebHandler) SaveFileHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fileName := r.URL.Query().Get("file")
	content := r.FormValue("content")

	filePath := fmt.Sprintf("%s/%s", id, fileName)
	err := wh.DockerMgr.FS.WriteFile(filePath, content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al guardar: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `
		<div class="p-4 bg-green-900 text-green-100 rounded border border-green-700 text-center">
			✅ Archivo <strong>%s</strong> guardado con éxito.
			<br><button hx-get="/files/%s" hx-target="#details-panel" class="mt-2 underline text-xs">Volver a la lista de archivos</button>
		</div>
	`, fileName, id)
}
