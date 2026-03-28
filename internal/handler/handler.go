package handler

import (
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/zedmakesense/url-shortner/internal/service"
)

type Handler struct {
	service   service.ServiceInterface
	templates *template.Template
}

func NewHandler(service service.ServiceInterface) *Handler {
	templates := template.Must(template.ParseFiles(filepath.Join("templates", "*html")))
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) ShowHome(w http.ResponseWriter, r *http.Request) {
}
