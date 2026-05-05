package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Dadido3/go-typst"
	"github.com/labstack/echo/v5"
	"github.com/shlewislee/typos/internal/printer"
)

type Handler struct {
	Templates map[string]Template
	tempDir   string
	FontPath  string
	Logger    *slog.Logger

	jobs        *Jobs
	printerConn *PrinterConn

	fontCache     []string
	fontCacheErr  error
	loadFontsOnce sync.Once
}

func NewHandler(logger *slog.Logger, tempDir, fontPath string, templates map[string]Template, jobs *Jobs, printerConn *PrinterConn) *Handler {
	return &Handler{
		Logger:      logger,
		tempDir:     tempDir,
		FontPath:    fontPath,
		Templates:   templates,
		jobs:        jobs,
		printerConn: printerConn,
	}
}

type TemplateRequest struct {
	Name         string                `json:"name"`
	Inputs       map[string]string     `json:"inputs"`
	ImageOptions *printer.ImageOptions `json:"image_options"`
}

func RegisterRoutes(e *echo.Echo, h *Handler) {
	e.GET("/health", h.handleHealth)
	e.POST("/print/template", h.handlePrintTemplate)
	e.POST("/print/file", h.handlePrintFile)
	e.POST("/print/image", h.handlePrintImage)
	e.GET("/print/jobs/:id", h.handleGetJob)
	e.GET("/printer/status", h.handleGetStatus)
	e.POST("/printer/reconnect", h.handleReconnect)
	e.GET("/fonts", h.handleListFonts)
}

func (h *Handler) handleHealth(c *echo.Context) error {
	if h.printerConn == nil || !h.printerConn.IsConnected() {
		return c.String(http.StatusServiceUnavailable, "printer offline")
	}
	return c.String(http.StatusOK, "OK")
}

func (h *Handler) handlePrintTemplate(c *echo.Context) error {
	h.Logger.Debug("handling template request", "uri", c.Request().RequestURI)

	var req TemplateRequest

	contentType := c.Request().Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		var err error
		id, err := h.withJobDir(func(jobID, jobDir string) error {
			req, err = h.parseMultipartTemplateRequest(c, jobDir)
			if err != nil {
				return err
			}
			return h.enqueueTemplateJob(req, jobID, jobDir)
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusAccepted, map[string]string{"id": id})
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json payload")
	}

	id, err := h.withJobDir(func(jobID, jobDir string) error {
		return h.enqueueTemplateJob(req, jobID, jobDir)
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]string{"id": id})
}

func (h *Handler) parseMultipartTemplateRequest(c *echo.Context, jobDir string) (TemplateRequest, error) {
	var req TemplateRequest
	req.Name = c.FormValue("name")

	inputsStr := c.FormValue("inputs")
	req.Inputs = make(map[string]string)
	if inputsStr != "" {
		if err := json.Unmarshal([]byte(inputsStr), &req.Inputs); err != nil {
			return req, echo.NewHTTPError(http.StatusBadRequest, "invalid inputs json string")
		}
	}

	var err error
	req.ImageOptions, err = h.parseImageOptions(c)
	if err != nil {
		return req, echo.NewHTTPError(http.StatusBadRequest, "invalid image_options json string")
	}

	err = h.handleMultipartUploads(c, jobDir, req.Inputs)
	if err != nil {
		return req, echo.NewHTTPError(http.StatusInternalServerError, "failed to process uploaded files")
	}

	return req, nil
}

func (h *Handler) buildFontPaths() []string {
	if h.FontPath != "" {
		return []string{h.FontPath}
	}
	return nil
}

func (h *Handler) enqueueTemplateJob(req TemplateRequest, jobID, jobDir string) error {
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "template name is required")
	}

	h.Logger.Debug("looking up template", "name", req.Name)
	tpl, ok := h.Templates[req.Name]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "template not found")
	}

	for _, field := range tpl.RequiredFields {
		if _, ok := req.Inputs[field]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "missing required field: "+field)
		}
	}
	fontPaths := h.buildFontPaths()

	job := &Job{
		ID:           jobID,
		Type:         JobTypeTemplate,
		TemplatePath: tpl.Filename,
		Inputs:       req.Inputs,
		JobDir:       jobDir,
		ImageOptions: req.ImageOptions,
		FontPaths:    fontPaths,
	}
	_, err := h.enqueueJob(job)
	return err
}

func (h *Handler) handlePrintFile(c *echo.Context) error {
	h.Logger.Debug("handling file upload request")

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file upload is required")
	}
	h.Logger.Debug("processing file upload", "filename", file.Filename)

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to open file")
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read file content")
	}

	inputsStr := c.FormValue("inputs")
	var inputs map[string]string
	if inputsStr != "" {
		if err := json.Unmarshal([]byte(inputsStr), &inputs); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid inputs json string")
		}
	} else {
		inputs = make(map[string]string)
	}

	imageOpts, err := h.parseImageOptions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid image_options json string")
	}

	fontPaths := h.buildFontPaths()

	id, err := h.withJobDir(func(jobID, jobDir string) error {
		err = h.handleMultipartUploads(c, jobDir, inputs, "file")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to process additional uploaded files")
		}

		job := &Job{
			ID:           jobID,
			Type:         JobTypeFile,
			FileContent:  content,
			Inputs:       inputs,
			JobDir:       jobDir,
			ImageOptions: imageOpts,
			FontPaths:    fontPaths,
		}
		_, err = h.enqueueJob(job)
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]string{"id": id})
}

func (h *Handler) handlePrintImage(c *echo.Context) error {
	h.Logger.Debug("handling image upload")

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file upload is required")
	}
	h.Logger.Debug("processing image upload", "filename", file.Filename)

	imageOpts, err := h.parseImageOptions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid image_options json string")
	}

	id, err := h.withJobDir(func(jobID, jobDir string) error {
		tmpPath, err := h.saveTempFile(file, jobDir)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save uploaded image")
		}

		job := &Job{
			ID:           jobID,
			Type:         JobTypeImage,
			ImagePath:    tmpPath,
			JobDir:       jobDir,
			ImageOptions: imageOpts,
		}
		_, err = h.enqueueJob(job)
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]string{"id": id})
}

func (h *Handler) handleGetJob(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "job id is required")
	}

	job, ok := h.jobs.Get(id)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	return c.JSON(http.StatusOK, job)
}

func (h *Handler) handleGetStatus(c *echo.Context) error {
	h.Logger.Debug("getting printer status")
	status, err := h.printerConn.Status()
	if err != nil {
		h.Logger.Warn("printer status query failed", "error", err)
		return c.JSON(http.StatusOK, map[string]string{
			"online": "na",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"online": status,
	})
}

func (h *Handler) handleReconnect(c *echo.Context) error {
	h.Logger.Info("reconnecting printer")
	status, err := h.printerConn.Reconnect()
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "disconnected",
			"error":  err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status": status,
	})
}

func (h *Handler) handleListFonts(c *echo.Context) error {
	h.Logger.Debug("listing fonts")

	h.loadFontsOnce.Do(func() {
		typstCaller := typst.CLI{}
		var opts *typst.OptionsFonts
		if h.FontPath != "" {
			opts = &typst.OptionsFonts{
				FontPaths: []string{h.FontPath},
			}
		}
		fonts, err := typstCaller.Fonts(opts)
		if err != nil {
			h.fontCacheErr = err
			return
		}
		h.fontCache = fonts
	})
	if h.fontCacheErr != nil {
		h.Logger.Error("failed to list fonts", "error", h.fontCacheErr)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list fonts")
	}

	return c.JSON(http.StatusOK, map[string][]string{
		"available_fonts": h.fontCache,
	})
}

func (h *Handler) withJobDir(fn func(jobID, jobDir string) error) (string, error) {
	jobID := generateID()
	jobDir := "job-" + jobID
	absJobDir := filepath.Join(h.tempDir, jobDir)

	if err := os.MkdirAll(absJobDir, 0o755); err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to create job directory")
	}

	needsCleanup := true
	defer func() {
		if needsCleanup {
			if err := os.RemoveAll(absJobDir); err != nil && !os.IsNotExist(err) {
				h.Logger.Error("failed to cleanup job directory", "path", absJobDir, "error", err)
			}
		}
	}()

	if err := fn(jobID, jobDir); err != nil {
		return "", err
	}
	needsCleanup = false
	return jobID, nil
}

func (h *Handler) enqueueJob(job *Job) (string, error) {
	h.Logger.Debug("enqueuing job", "type", job.Type)
	id, err := h.jobs.Enqueue(job)
	if err != nil {
		if errors.Is(err, ErrQueueFull) {
			return "", echo.NewHTTPError(http.StatusServiceUnavailable, "job queue is full")
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue job")
	}
	return id, nil
}

func (h *Handler) saveTempFile(fileHeader *multipart.FileHeader, destDir string) (string, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	absDestDir := filepath.Join(h.tempDir, destDir)
	if err := os.MkdirAll(absDestDir, 0o755); err != nil {
		return "", err
	}

	filename := filepath.Base(fileHeader.Filename)
	absDestPath := filepath.Join(absDestDir, filename)

	tmpFile, err := os.Create(absDestPath)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err = io.Copy(tmpFile, src); err != nil {
		return "", err
	}

	return absDestPath, nil
}

func (h *Handler) handleMultipartUploads(c *echo.Context, jobDir string, inputs map[string]string, excludeKeys ...string) error {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return nil
	}

	excludeMap := make(map[string]bool)
	for _, key := range excludeKeys {
		excludeMap[key] = true
	}

	for key, fileHeaders := range form.File {
		if excludeMap[key] || len(fileHeaders) == 0 {
			continue
		}
		fileHeader := fileHeaders[0]
		h.Logger.Debug("processing uploaded file", "key", key, "filename", fileHeader.Filename)

		tmpPath, err := h.saveTempFile(fileHeader, jobDir)
		if err != nil {
			return err
		}
		h.Logger.Debug("saved temporary file", "path", tmpPath)

		inputs[key] = filepath.Base(fileHeader.Filename)
	}

	return nil
}

func (h *Handler) parseImageOptions(c *echo.Context) (*printer.ImageOptions, error) {
	rotateStr := c.FormValue("rotate_image")
	ditherStr := c.FormValue("dither_method")
	gammaStr := c.FormValue("gamma")

	if rotateStr == "" && ditherStr == "" && gammaStr == "" {
		return nil, nil
	}

	opts := &printer.ImageOptions{}
	if rotateStr != "" {
		opts.ShouldRotate = rotateStr == "true" || rotateStr == "1"
	}
	if ditherStr != "" {
		d, err := strconv.Atoi(ditherStr)
		if err != nil {
			return nil, err
		}
		if d < 0 || d > 2 {
			return nil, fmt.Errorf("invalid dither_method: %d", d)
		}
		opts.DitherMethod = printer.DitherMethod(d)
	}
	if gammaStr != "" {
		g, err := strconv.ParseFloat(gammaStr, 64)
		if err != nil {
			return nil, err
		}
		opts.Gamma = g
	}
	return opts, nil
}
