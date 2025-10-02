package admin

import (
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
)

// QRCode handles the generation of QR codes for the current instance.
func (h *AdminHandler) QRCode(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Extract parameters from the URL
	extension := chi.URLParam(r, "extension")
	if extension != "png" && extension != "svg" {
		h.logger.Error("QRCodeHandler: Invalid extension provided")
		http.Error(w, "Invalid extension provided", http.StatusNotFound)
		return
	}

	action := chi.URLParam(r, "action")
	if action != "in" && action != "out" {
		h.logger.Error("QRCodeHandler: Invalid type provided")
		http.Error(w, "Improper type provided", http.StatusNotFound)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		h.logger.Error("QRCodeHandler: No location provided")
		http.Error(w, "No location provided", http.StatusNotFound)
		return
	}

	// Check if the user has access to the location
	access, err := h.accessService.CanAdminAccessMarker(r.Context(), user.ID, id)
	if err != nil {
		h.logger.Error("QRCodeHandler: Error checking access", "error", err)
		http.Error(w, "Error checking access", http.StatusInternalServerError)
		return
	}
	if !access {
		h.logger.Error("QRCodeHandler: User does not have access to this location", "user", user.ID, "location", id)
		http.Error(w, "You do not have access to this location", http.StatusForbidden)
		return
	}

	// Get the path and content for the QR code
	path, content := h.assetGenerator.GetQRCodePathAndContent(action, id, "", extension)

	// Check if the file already exists, if so serve it
	if _, statErr := os.Stat(path); statErr == nil {
		if extension == "svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
		} else {
			w.Header().Set("Content-Type", "image/png")
		}
		http.ServeFile(w, r, path)
		return
	}

	// Generate the QR code
	err = h.assetGenerator.CreateQRCodeImage(
		r.Context(),
		path,
		content,
		h.assetGenerator.WithQRFormat(extension),
	)
	if err != nil {
		h.logger.Error("QRCodeHandler: Could not create QR code", "error", err)
		http.Error(w, "Could not create QR code", http.StatusInternalServerError)
		return
	}

	// Serve the generated QR code
	switch extension {
	case "svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case "png":
		w.Header().Set("Content-Type", "image/png")
	default:
		http.Error(w, "Invalid extension provided", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, path)
}

// GenerateQRCodeArchive generates a zip file containing all the QR codes for the current instance.
func (h *AdminHandler) GenerateQRCodeArchive(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	var paths []string
	for _, location := range user.CurrentInstance.Locations {
		for _, extension := range []string{"png", "svg"} {
			path, content := h.assetGenerator.GetQRCodePathAndContent("in", location.MarkerID, location.Name, extension)
			paths = append(paths, path)

			// Check if the file already exists, otherwise generate it
			if _, err := os.Stat(path); err == nil {
				continue
			}

			// Generate the QR code
			err := h.assetGenerator.CreateQRCodeImage(
				r.Context(),
				path,
				content,
				h.assetGenerator.WithQRFormat(extension),
			)
			if err != nil {
				h.logger.Error("QRCodeHandler: Could not create QR code", "error", err)
				http.Error(w, "Could not create QR code", http.StatusInternalServerError)
				return
			}
		}
	}

	path, err := h.assetGenerator.CreateArchive(r.Context(), paths)
	if err != nil {
		h.logger.Error("QR codes could not be zipped", "error", err, "instance", user.CurrentInstanceID)
		http.Error(w, "QR codes could not be zipped", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, path)
	os.Remove(path)
}

// GeneratePosters generates a PDF file containing all the QR codes for the current instance.
func (h *AdminHandler) GeneratePosters(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	pdfData := services.PDFData{
		InstanceName: user.CurrentInstance.Name,
		Pages:        services.PDFPages{},
	}

	for _, location := range user.CurrentInstance.Locations {
		path, content := h.assetGenerator.GetQRCodePathAndContent("in", location.MarkerID, location.Name, "png")

		// Check if the file already exists, otherwise generate it
		if _, statErr := os.Stat(path); statErr != nil {
			// Generate the QR code
			qrErr := h.assetGenerator.CreateQRCodeImage(
				r.Context(),
				path,
				content,
				h.assetGenerator.WithQRFormat("png"),
			)
			if qrErr != nil {
				h.logger.Error("GeneratePoster: Could not create posters", "error", qrErr)
				http.Error(w, "Could not create posters", http.StatusInternalServerError)
				return
			}
		}

		page := services.PDFPage{
			LocationName: location.Name,
			ImagePath:    path,
			URL:          content,
		}
		pdfData.Pages = append(pdfData.Pages, page)
	}
	path, err := h.assetGenerator.CreatePDF(r.Context(), pdfData)
	if err != nil {
		h.logger.Error("Posters could not be generated", "error", err, "instance", user.CurrentInstanceID)
		http.Error(w, "Posters could not be generated", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+user.CurrentInstance.Name+" posters.pdf\"")
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, path)
	os.Remove(path)
}

// GeneratePoster generates a poster for the given location.
func (h *AdminHandler) GeneratePoster(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		h.logger.Error("QRCodeHandler: No location provided")
		http.Error(w, "No location provided", http.StatusNotFound)
		return
	}

	found := false
	var location models.Location
	for _, loc := range user.CurrentInstance.Locations {
		if loc.MarkerID == id {
			found = true
			location = loc
			break
		}
	}
	if !found {
		h.logger.Error("GeneratePoster: Location not found", "location", id)
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	pdfData := services.PDFData{
		InstanceName: user.CurrentInstance.Name,
		Pages:        services.PDFPages{},
	}

	path, content := h.assetGenerator.GetQRCodePathAndContent("in", location.MarkerID, location.Name, "png")

	// Check if the file already exists, otherwise generate it
	if _, statErr := os.Stat(path); statErr != nil {
		// Generate the QR code
		qrErr := h.assetGenerator.CreateQRCodeImage(
			r.Context(),
			path,
			content,
			h.assetGenerator.WithQRFormat("png"),
		)
		if qrErr != nil {
			h.logger.Error("GeneratePoster: Could not create posters", "error", qrErr)
			http.Error(w, "Could not create posters", http.StatusInternalServerError)
			return
		}
	}

	page := services.PDFPage{
		LocationName: location.Name,
		ImagePath:    path,
		URL:          content,
	}

	pdfData.Pages = append(pdfData.Pages, page)
	path, err := h.assetGenerator.CreatePDF(r.Context(), pdfData)
	if err != nil {
		h.logger.Error("Posters could not be generated", "error", err, "instance", user.CurrentInstanceID)
		http.Error(w, "Posters could not be generated", http.StatusInternalServerError)
		return
	}

	w.Header().
		Set("Content-Disposition", "attachment; filename=\""+user.CurrentInstance.Name+" - "+location.Name+" poster.pdf\"")
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, path)
	os.Remove(path)
}
