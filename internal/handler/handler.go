package handler

import (
	"path/filepath"
	"text/template"

	"github.com/zedmakesense/url-shortner/internal/service"
)

type Handler struct {
	service   service.Service
	templates *template.Template
}

func NewHandler(service service.Service) *Handler {
	templates := template.Must(template.ParseFiles(filepath.Join("templates", "*html")))
	return &Handler{
		service:   service,
		templates: templates,
	}
}
