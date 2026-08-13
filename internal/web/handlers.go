package web

import (
	"go-docker-manager/internal/docker"
	"html/template"
	"net/http"
	"github.com/go-chi/chi/v5"
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

	// Con HTMX, no recargamos la página, solo respondemos con un mensaje o actualizamos la fila
	w.Write([]byte("✅ Acción ejecutada con éxito"))
}