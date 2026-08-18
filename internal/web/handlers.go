package web

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
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

func (wh *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	containers, err := wh.DockerMgr.ListContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wh.Templates.ExecuteTemplate(w, "index.html", containers)
}

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

	// Renderizamos de nuevo el cuerpo de la tabla actualizado
	containers, _ := wh.DockerMgr.ListContainers()
	for _, c := range containers {
		stateBadge := `<span class="px-2.5 py-1 rounded-full text-xs font-semibold bg-red-900/60 text-red-300 border border-red-700">exited</span>`
		if c.State == "running" {
			stateBadge = `<span class="px-2.5 py-1 rounded-full text-xs font-semibold bg-green-900/60 text-green-300 border border-green-700">running</span>`
		}

		fmt.Fprintf(w, `
			<tr class="hover:bg-gray-750 transition-colors">
				<td class="px-5 py-4">
					<div class="font-semibold text-blue-300">%s</div>
					<div class="text-xs text-gray-500 font-mono">%s • %s</div>
				</td>
				<td class="px-5 py-4">%s</td>
				<td class="px-5 py-4 text-center space-x-1">
					<button hx-get="/control/%s?action=start" hx-target="#containers-body" class="bg-green-700 hover:bg-green-600 px-2.5 py-1 rounded text-xs">▶</button>
					<button hx-get="/control/%s?action=stop" hx-target="#containers-body" class="bg-red-700 hover:bg-red-600 px-2.5 py-1 rounded text-xs">⏹</button>
					<button hx-get="/control/%s?action=restart" hx-target="#containers-body" class="bg-yellow-700 hover:bg-yellow-600 px-2.5 py-1 rounded text-xs">🔄</button>
					<button hx-get="/details/%s?id=%s" hx-target="#details-panel" class="bg-blue-600 hover:bg-blue-500 px-3 py-1 rounded text-xs font-semibold">Gestionar</button>
				</td>
			</tr>
		`, c.Name, c.ID, c.Image, stateBadge, c.ID, c.ID, c.ID, c.Name, c.ID)
	}
}

// LogsHandler ahora crea una estructura de flujo natural (Top-Down)
func (wh *WebHandler) LogsHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := wh.DockerMgr.GetLogs(id, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	firstTimestamp := extractFirstTimestamp(logs)

	fmt.Fprintf(w, `
		<div class="flex flex-col space-y-2">
			<div class="flex justify-between items-center mb-2">
				<span class="text-sm font-semibold">Logs Recientes</span>
				<button hx-get="/logs/%s" hx-target="#log-container" class="text-xs bg-gray-700 px-2 py-1 rounded">🔄 Actualizar</button>
			</div>
			
			<div id="log-container" class="flex flex-col">
				<!-- Botón de carga previa: Siempre arriba del contenido -->
				<div id="prev-button-area" class="flex justify-center mb-2">
					<button hx-get="/logs-prev/%s?until=%s" 
							hx-target="#log-content" 
							hx-swap="afterbegin" 
							class="text-[10px] text-blue-400 hover:text-blue-300 underline">
						⬆️ Cargar 100 líneas anteriores
					</button>
				</div>
				
				<!-- Área de contenido de logs -->
				<div id="log-content" class="text-xs text-green-400 p-2 bg-black rounded border border-gray-700 overflow-auto max-h-96 font-mono whitespace-pre-wrap">
					%s
				</div>
			</div>
		</div>
	`, id, id, firstTimestamp, logs)
}

// LogsPreviousHandler ahora solo devuelve el bloque de texto
func (wh *WebHandler) LogsPreviousHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	until := r.URL.Query().Get("until")

	logs, err := wh.DockerMgr.GetLogs(id, until)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	// Devolvemos el texto envuelto en un div para que se mantenga el formato
	// Usamos un div simple porque hx-swap="afterbegin" lo pondrá arriba del contenido actual
	fmt.Fprintf(w, `<div class="pb-2 border-b border-gray-800 mb-2 opacity-70">%s</div>`, logs)
}

// Función auxiliar para extraer la fecha de la primera línea del log
func extractFirstTimestamp(logs string) string {
	lines := strings.Split(logs, "\n")
	if len(lines) == 0 { return "" }
	
	// El formato de Docker con Timestamps es: 2023-10-27T10:00:00.000Z mensaje...
	firstLine := lines[0]
	if len(firstLine) > 24 {
		return firstLine[:24] // Extraemos la fecha y hora (ISO8601)
	}
	return ""
}


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
// 📁 GESTIÓN DE ARCHIVOS POR PROYECTO (FASE 2)
// =============================================================================

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
		if entry.IsDir() { continue }
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

// =============================================================================
// 🌍 EXPLORADOR GLOBAL (FASE 3)
// =============================================================================

func (wh *WebHandler) ExplorerHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" { path = "." }
	entries, err := wh.DockerMgr.FS.ListDir(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al listar: %v", err), http.StatusInternalServerError)
		return
	}
	html := fmt.Sprintf(`
		<div class="flex flex-col space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-purple-400 font-bold">📂 Explorador: /stacks/%s</h3>
				<button hx-get="/explorer?path=." hx-target="#details-panel" class="text-xs bg-gray-600 px-2 py-1 rounded">🏠 Raíz</button>
			</div>
			<form hx-post="/explorer/upload?path=%s" hx-target="#details-panel" hx-encoding="multipart/form-data" class="flex gap-2 bg-gray-700 p-2 rounded border border-gray-600">
				<input type="file" name="file" class="text-xs text-gray-300">
				<button type="submit" class="bg-green-600 hover:bg-green-500 px-2 py-1 rounded text-xs font-bold">Subir</button>
			</form>
			<div class="grid grid-cols-1 gap-2">
	`, path, path)
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(path, name)
		if entry.IsDir() {
			html += fmt.Sprintf(`
				<button hx-get="/explorer?path=%s" hx-target="#details-panel" 
					class="text-left p-2 bg-gray-800 hover:bg-gray-700 rounded text-xs border border-gray-700 flex justify-between items-center">
					<span>📁 <strong>%s</strong></span>
					<span class="text-gray-500 text-[10px]">Abrir →</span>
				</button>
			`, fullPath, name)
		} else {
			html += fmt.Sprintf(`
				<div class="flex items-center justify-between p-2 bg-gray-700 hover:bg-gray-600 rounded text-xs border border-gray-600">
					<span class="truncate">📄 %s</span>
					<div class="flex gap-1">
						<a href="/explorer/download?path=%s" class="bg-blue-600 hover:bg-blue-500 p-1 rounded text-[10px]">⬇️</a>
						<button hx-delete="/explorer/delete?path=%s" hx-target="#details-panel" hx-confirm="¿Seguro?" class="bg-red-600 hover:bg-red-500 p-1 rounded text-[10px]">🗑️</button>
						<button hx-get="/edit-global?path=%s" hx-target="#details-panel" class="bg-yellow-600 hover:bg-yellow-500 p-1 rounded text-[10px]">✏️</button>
					</div>
				</div>
			`, name, fullPath, fullPath, fullPath)
		}
	}
	html += `</div></div>`
	fmt.Fprint(w, html)
}

func (wh *WebHandler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error al recibir archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()
	destPath := filepath.Join(path, header.Filename)
	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		http.Error(w, "Error al leer archivo", http.StatusInternalServerError)
		return
	}
	err = wh.DockerMgr.FS.WriteFile(destPath, buf.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al guardar: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `<p class="text-green-400 text-xs p-2">✅ Subido con éxito</p>`)
	w.Header().Set("HX-Trigger", "refreshExplorer")
}

func (wh *WebHandler) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := wh.DockerMgr.FS.ReadFile(path)
	if err != nil {
		http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(path)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write([]byte(content))
}

func (wh *WebHandler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	err := wh.DockerMgr.FS.DeleteFile(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al borrar: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `<p class="text-red-400 text-xs p-2">🗑️ Eliminado con éxito</p>`)
}

func (wh *WebHandler) EditGlobalHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := wh.DockerMgr.FS.ReadFile(path)
	if err != nil {
		http.Error(w, "Error al leer archivo", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `
		<div class="flex flex-col space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-blue-400 font-bold">📄 Editando: %s</h3>
				<button hx-get="/explorer" hx-target="#details-panel" class="text-xs bg-gray-600 px-2 py-1 rounded">Volver</button>
			</div>
			<form hx-post="/save-global?path=%s" hx-target="#details-panel" hx-swap="outerHTML">
				<textarea name="content" class="w-full h-64 p-2 bg-black text-green-400 font-mono text-xs border border-gray-700 rounded focus:outline-none focus:border-blue-500">%s</textarea>
				<button type="submit" class="mt-2 bg-blue-600 hover:bg-blue-500 px-4 py-2 rounded text-sm font-bold w-full">💾 Guardar Cambios</button>
			</form>
		</div>
	`, filepath.Base(path), path, content)
}

func (wh *WebHandler) SaveGlobalHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content := r.FormValue("content")
	err := wh.DockerMgr.FS.WriteFile(path, content)
	if err != nil {
		http.Error(w, "Error al guardar", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `<div class="p-4 bg-green-900 text-green-100 rounded text-center">✅ Guardado con éxito. <br><button hx-get="/explorer" hx-target="#details-panel" class="underline text-xs">Volver al explorador</button></div>`)
}

// CreateContainerPage muestra el formulario de creación
func (wh *WebHandler) CreateContainerPage(w http.ResponseWriter, r *http.Request) {
	// Capturamos la imagen si viene desde la Store (ej: /create?image=nginx)
	selectedImage := r.URL.Query().Get("image")

	fmt.Fprintf(w, `
		<div class="flex flex-col space-y-4">
			<h3 class="text-green-400 font-bold">🚀 Desplegar Nuevo Contenedor</h3>
			<form hx-post="/create" hx-target="#details-panel" class="flex flex-col gap-3">
				<div>
					<label class="text-xs text-gray-400">Nombre del Proyecto/Contenedor</label>
					<input type="text" name="name" placeholder="ej: mi-nginx" class="w-full bg-black p-2 rounded border border-gray-700 text-sm text-white" required>
				</div>
				<div>
					<label class="text-xs text-gray-400">Imagen de Docker Hub</label>
					<input type="text" name="image" value="%s" placeholder="ej: nginx:latest" class="w-full bg-black p-2 rounded border border-gray-700 text-sm text-white" required>
				</div>
				<div>
					<label class="text-xs text-gray-400">Puertos (Host:Contenedor, separados por coma)</label>
					<input type="text" name="ports" placeholder="ej: 8080:80, 3000:3000" class="w-full bg-black p-2 rounded border border-gray-700 text-sm text-white">
				</div>
				<button type="submit" class="bg-green-600 hover:bg-green-500 p-2 rounded font-bold text-sm transition-colors">🚀 Crear y Lanzar</button>
			</form>
			<button hx-get="/explorer" hx-target="#details-panel" class="text-xs text-gray-400 underline">Volver al explorador</button>
		</div>
	`, selectedImage) // <--- Aquí inyectamos la imagen seleccionada
}


// CreateContainerHandler procesa la creación
func (wh *WebHandler) CreateContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	image := r.FormValue("image")
	portsStr := r.FormValue("ports")
	
	var ports []string
	if portsStr != "" {
		ports = strings.Split(portsStr, ",")
	}

	err := wh.DockerMgr.CreateManagedContainer(name, image, ports)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `
		<div class="p-4 bg-green-900 text-green-100 rounded text-center">
			✅ Contenedor <strong>%s</strong> creado y lanzado con éxito.
			<br><button hx-get="/" hx-target="body" class="mt-2 underline text-xs">Ver en Dashboard</button>
		</div>
	`, name)
}

// StoreHandler renderiza la página principal de la tienda/catálogo
func (wh *WebHandler) StoreHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `
		<div class="flex flex-col space-y-6">
			<div class="text-center space-y-2">
				<h2 class="text-3xl font-bold text-blue-400">🏪 Docker Store</h2>
				<p class="text-gray-400 text-sm">Busca y despliega imágenes oficiales y comunitarias de Docker Hub</p>
			</div>

			<div class="relative">
				<input type="text" 
					name="query" 
					placeholder="Buscar imagen (ej: mysql, nginx, redis...)" 
					hx-get="/search" 
					hx-trigger="keyup changed delay:500ms" 
					hx-target="#search-results" 
					class="w-full p-4 bg-gray-800 text-white rounded-xl border border-gray-700 focus:border-blue-500 focus:outline-none transition-all text-lg shadow-2xl">
				<div class="absolute right-4 top-4 text-gray-500">🔍</div>
			</div>

			<div id="search-results" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
				<div class="col-span-full text-center text-gray-500 italic py-10">
					Escribe algo arriba para empezar a buscar imágenes...
				</div>
			</div>
		</div>
	`)
}

// SearchImagesHandler procesa la búsqueda y devuelve las "Cards" de las imágenes
func (wh *WebHandler) SearchImagesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		fmt.Fprintf(w, `<div class="col-span-full text-center text-gray-500 italic py-10">Escribe algo para buscar...</div>`)
		return
	}

	images, err := wh.DockerMgr.SearchImages(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error en la búsqueda: %v", err), http.StatusInternalServerError)
		return
	}

	if len(images) == 0 {
		fmt.Fprintf(w, `<div class="col-span-full text-center text-gray-500 italic py-10">No se encontraron imágenes para "%s"</div>`, query)
		return
	}

	html := ""
	for _, img := range images {
		officialBadge := ""
		if img.IsOfficial {
			officialBadge = `<span class="bg-blue-600 text-[10px] px-2 py-0.5 rounded-full text-white font-bold uppercase">Oficial 🛡️</span>`
		}

		html += fmt.Sprintf(`
			<div class="bg-gray-800 p-4 rounded-xl border border-gray-700 hover:border-blue-500 transition-all group shadow-lg flex flex-col justify-between">
				<div>
					<div class="flex justify-between items-start mb-2">
						<h4 class="text-blue-300 font-bold truncate pr-2" title="%s">%s</h4>
						%s
					</div>
					<p class="text-gray-400 text-xs line-clamp-3 mb-4 h-12">%s</p>
					<div class="flex items-center text-yellow-500 text-xs font-medium mb-4">
						<span>⭐ %d estrellas</span>
					</div>
				</div>
				<button hx-get="/create?image=%s" hx-target="#details-panel" 
					class="w-full bg-gray-700 group-hover:bg-blue-600 text-white py-2 rounded-lg text-xs font-bold transition-colors">
					Instalar 🚀
				</button>
			</div>
		`, img.Name, img.Name, officialBadge, img.Description, img.Stars, img.Name)
	}

	fmt.Fprint(w, html)
}