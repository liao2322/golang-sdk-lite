package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/halalcloud/golang-sdk-lite/halalcloud/apiclient"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/config"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/model"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/services/oauth"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/services/offline"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/services/user"
	"github.com/halalcloud/golang-sdk-lite/halalcloud/services/userfile"
)

type App struct {
	cfg      Config
	sessions *sessionStore
}

type apiResponse map[string]any

type appSession struct {
	session *sessionData
	client  *apiclient.Client
}

type redirectLink struct {
	URL string
}

func NewApp(cfg Config) *App {
	return &App{
		cfg:      cfg,
		sessions: newSessionStore(),
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/device-code/start", a.handleDeviceCodeStart)
	mux.HandleFunc("/api/auth/device-code/status", a.handleDeviceCodeStatus)
	mux.HandleFunc("/api/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/auth/session", a.handleSession)
	mux.HandleFunc("/api/user/me", a.withSession(a.handleUserMe))
	mux.HandleFunc("/api/user/quota", a.withSession(a.handleUserQuota))
	mux.HandleFunc("/api/files", a.withSession(a.handleFilesList))
	mux.HandleFunc("/api/files/detail", a.withSession(a.handleFileDetail))
	mux.HandleFunc("/api/files/recent", a.withSession(a.handleRecentFiles))
	mux.HandleFunc("/api/files/trash", a.withSession(a.handleTrashList))
	mux.HandleFunc("/api/files/create", a.withSession(a.handleFileCreate))
	mux.HandleFunc("/api/files/rename", a.withSession(a.handleFileRename))
	mux.HandleFunc("/api/files/move", a.withSession(a.handleFileMove))
	mux.HandleFunc("/api/files/copy", a.withSession(a.handleFileCopy))
	mux.HandleFunc("/api/files/delete", a.withSession(a.handleFileDelete))
	mux.HandleFunc("/api/files/trash-action", a.withSession(a.handleFileTrash))
	mux.HandleFunc("/api/files/recover", a.withSession(a.handleFileRecover))
	mux.HandleFunc("/api/files/upload-task", a.withSession(a.handleUploadTask))
	mux.HandleFunc("/api/files/direct-link/", a.withSession(a.handleDirectLink))
	mux.HandleFunc("/api/files/play/", a.withSession(a.handlePlayRedirect))
	mux.HandleFunc("/api/files/download/", a.withSession(a.handleDownloadRedirect))
	mux.HandleFunc("/api/offline/parse", a.withSession(a.handleOfflineParse))
	mux.HandleFunc("/api/offline/tasks", a.withSession(a.handleOfflineTasks))
	mux.HandleFunc("/api/offline/tasks/", a.withSession(a.handleOfflineTaskByID))

	staticRoot, _ := fs.Sub(staticFS, "static")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(staticRoot))))
	mux.HandleFunc("/manifest.webmanifest", a.handleStaticFile("static/manifest.webmanifest", "application/manifest+json; charset=utf-8"))
	mux.HandleFunc("/sw.js", a.handleStaticFile("static/sw.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/favicon.svg", a.handleStaticFile("static/icon.svg", "image/svg+xml"))
	mux.HandleFunc("/apple-touch-icon.png", a.handleStaticFile("static/icon.svg", "image/svg+xml"))
	mux.HandleFunc("/", a.handleIndex)
	return mux
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	index, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

func (a *App) handleStaticFile(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := staticFS.ReadFile(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	session, ok := a.sessions.fromRequest(r)
	if !ok {
		writeJSON(w, http.StatusOK, apiResponse{"authenticated": false})
		return
	}
	client := a.newClient(session.Store)
	userSvc := user.NewUserService(client)
	resp, err := userSvc.Get(r.Context(), &user.User{})
	if err != nil {
		writeJSON(w, http.StatusOK, apiResponse{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{
		"authenticated": true,
		"user":          sanitizeUser(resp),
		"default_mode":  a.cfg.DefaultLinkMode,
	})
}

func (a *App) handleDeviceCodeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	client := a.newClient(config.NewMapConfigStore())
	svc := oauth.NewOAuthService(client)
	resp, err := svc.DeviceCodeAuthorize(r.Context(), &oauth.AuthorizeRequest{
		ClientId: a.cfg.ClientID,
		Device:   "golang-sdk-lite-webui/1.0",
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleDeviceCodeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	deviceCode := r.URL.Query().Get("device_code")
	if deviceCode == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "device_code is required"})
		return
	}
	client := a.newClient(config.NewMapConfigStore())
	svc := oauth.NewOAuthService(client)
	resp, err := svc.GetDeviceCodeState(r.Context(), &oauth.DeviceCodeAuthorizeState{
		DeviceCode: deviceCode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if resp.Status == "AUTHORIZATION_SUCCESS" && resp.AccessToken != "" {
		session, err := a.sessions.create()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{"error": err.Error()})
			return
		}
		client := a.newClient(session.Store)
		client.SetToken(resp.AccessToken, resp.RefreshToken, resp.ExpiresIn)
		a.sessions.writeCookie(w, session)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		a.sessions.delete(cookie.Value)
	}
	a.sessions.clearCookie(w)
	writeJSON(w, http.StatusOK, apiResponse{"ok": true})
}

func (a *App) handleUserMe(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := user.NewUserService(appSession.client).Get(r.Context(), &user.User{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sanitizeUser(resp))
}

func (a *App) handleUserQuota(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := user.NewUserService(appSession.client).GetStatisticsAndQuota(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleFilesList(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).List(r.Context(), &userfile.FileListRequest{
		Parent: &userfile.File{Path: queryString(r, "path", a.cfg.DefaultRoot)},
		ListInfo: &model.ScanListRequest{
			Token: r.URL.Query().Get("cursor"),
			Limit: queryInt64(r, "limit", 50),
		},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, normalizeFileList(resp))
}

func (a *App) handleFileDetail(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	file := &userfile.File{
		Identity: r.URL.Query().Get("identity"),
		Path:     r.URL.Query().Get("path"),
	}
	if file.Identity == "" && file.Path == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "identity or path is required"})
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).Get(r.Context(), file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleRecentFiles(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).ListRecentUpdatedFiles(r.Context(), &userfile.ListRecentUpdatedFilesRequest{
		Parent:  &userfile.File{Path: queryString(r, "path", a.cfg.DefaultRoot)},
		StartTs: queryInt64(r, "start_ts", 0),
		ListInfo: &model.ScanListRequest{
			Limit: queryInt64(r, "limit", 50),
		},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, normalizeFileList(resp))
}

func (a *App) handleTrashList(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).ListTrash(r.Context(), &userfile.FileListRequest{
		Parent: &userfile.File{Path: queryString(r, "path", a.cfg.DefaultRoot)},
		ListInfo: &model.ScanListRequest{
			Token: r.URL.Query().Get("cursor"),
			Limit: queryInt64(r, "limit", 50),
		},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, normalizeFileList(resp))
}

func (a *App) handleFileCreate(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Parent   string `json:"parent"`
		Name     string `json:"name"`
		Dir      bool   `json:"dir"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).Create(r.Context(), &userfile.File{
		Parent:   req.Parent,
		Name:     req.Name,
		Dir:      req.Dir,
		MimeType: req.MimeType,
		Size:     req.Size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleFileRename(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Identity string `json:"identity"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).Rename(r.Context(), &userfile.File{
		Identity: req.Identity,
		Name:     req.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleFileMove(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	a.handleBatchFileOperation(w, r, appSession, "move")
}

func (a *App) handleFileCopy(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	a.handleBatchFileOperation(w, r, appSession, "copy")
}

func (a *App) handleFileDelete(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	a.handleBatchFileOperation(w, r, appSession, "delete")
}

func (a *App) handleFileTrash(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	a.handleBatchFileOperation(w, r, appSession, "trash")
}

func (a *App) handleFileRecover(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	a.handleBatchFileOperation(w, r, appSession, "recover")
}

func (a *App) handleUploadTask(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Parent   string `json:"parent"`
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
		return
	}
	resp, err := userfile.NewUserFileService(appSession.client).CreateUploadTask(r.Context(), &userfile.File{
		Parent:   req.Parent,
		Name:     req.Name,
		MimeType: req.MimeType,
		Size:     req.Size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleDirectLink(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := a.directLink(r.Context(), appSession, strings.TrimPrefix(r.URL.Path, "/api/files/direct-link/"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handlePlayRedirect(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	identity := strings.TrimPrefix(r.URL.Path, "/api/files/play/")
	if a.linkMode(r) == "proxy" {
		if err := serveOpenListProxy(w, r, userfile.NewUserFileService(appSession.client), identity); err != nil {
			writeError(w, err)
		}
		return
	}
	link, err := a.openListRedirectLink(r.Context(), appSession, identity)
	if err != nil {
		writeError(w, err)
		return
	}
	a.redirectLikeOpenList(w, r, link)
}

func (a *App) handleDownloadRedirect(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	identity := strings.TrimPrefix(r.URL.Path, "/api/files/download/")
	if a.linkMode(r) == "proxy" {
		if err := serveOpenListProxy(w, r, userfile.NewUserFileService(appSession.client), identity); err != nil {
			writeError(w, err)
		}
		return
	}
	link, err := a.openListRedirectLink(r.Context(), appSession, identity)
	if err != nil {
		writeError(w, err)
		return
	}
	a.redirectLikeOpenList(w, r, link)
}

func (a *App) handleOfflineParse(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req offline.TaskParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
		return
	}
	resp, err := offline.NewOfflineTaskService(appSession.client).Parse(r.Context(), &req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleOfflineTasks(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	svc := offline.NewOfflineTaskService(appSession.client)
	switch r.Method {
	case http.MethodGet:
		resp, err := svc.List(r.Context(), &offline.OfflineTaskListRequest{
			ListInfo: &model.ScanListRequest{
				Token: r.URL.Query().Get("cursor"),
				Limit: queryInt64(r, "limit", 50),
			},
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			"tasks":       resp.Tasks,
			"next_cursor": nextCursor(resp.ListInfo),
			"has_more":    nextCursor(resp.ListInfo) != "",
		})
	case http.MethodPost:
		var req offline.UserTask
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
			return
		}
		resp, err := svc.Add(r.Context(), &req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		methodNotAllowed(w)
	}
}

func (a *App) handleOfflineTaskByID(w http.ResponseWriter, r *http.Request, appSession *appSession) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	identity := path.Base(strings.TrimSuffix(r.URL.Path, "/"))
	if identity == "" || identity == "tasks" {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "identity is required"})
		return
	}
	resp, err := offline.NewOfflineTaskService(appSession.client).Delete(r.Context(), &offline.OfflineTaskDeleteRequest{
		Identity: []string{identity},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleBatchFileOperation(w http.ResponseWriter, r *http.Request, appSession *appSession, operation string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		IDs      []string `json:"ids"`
		DestPath string   `json:"dest_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "invalid json"})
		return
	}
	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "ids is required"})
		return
	}

	source := make([]*userfile.File, 0, len(req.IDs))
	for _, id := range req.IDs {
		source = append(source, &userfile.File{Identity: id})
	}
	request := &userfile.BatchOperationRequest{Source: source}
	if req.DestPath != "" {
		request.Dest = &userfile.File{Path: req.DestPath}
	}

	svc := userfile.NewUserFileService(appSession.client)
	var (
		resp *userfile.BatchOperationResponse
		err  error
	)
	switch operation {
	case "move":
		resp, err = svc.Move(r.Context(), request)
	case "copy":
		resp, err = svc.Copy(r.Context(), request)
	case "delete":
		resp, err = svc.Delete(r.Context(), request)
	case "trash":
		resp, err = svc.Trash(r.Context(), request)
	case "recover":
		resp, err = svc.Recover(r.Context(), request)
	default:
		writeJSON(w, http.StatusBadRequest, apiResponse{"error": "unsupported operation"})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) directLink(ctx context.Context, appSession *appSession, identity string) (*userfile.FileDownloadAddressResponse, error) {
	if identity == "" {
		return nil, errors.New("identity is required")
	}
	return userfile.NewUserFileService(appSession.client).GetDirectDownloadAddress(ctx, &userfile.DirectDownloadRequest{
		Identity: identity,
	})
}

func (a *App) openListRedirectLink(ctx context.Context, appSession *appSession, identity string) (*redirectLink, error) {
	resp, err := a.directLink(ctx, appSession, identity)
	if err != nil {
		return nil, err
	}
	return &redirectLink{URL: resp.DownloadAddress}, nil
}

func (a *App) linkMode(r *http.Request) string {
	return normalizeLinkMode(queryString(r, "mode", a.cfg.DefaultLinkMode))
}

func (a *App) redirectLikeOpenList(w http.ResponseWriter, r *http.Request, link *redirectLink) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (a *App) withSession(handler func(http.ResponseWriter, *http.Request, *appSession)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.sessions.fromRequest(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, apiResponse{"error": "not authenticated"})
			return
		}
		handler(w, r, &appSession{
			session: session,
			client:  a.newClient(session.Store),
		})
	}
}

func (a *App) newClient(store config.ConfigStore) *apiclient.Client {
	return apiclient.NewClient(nil, a.cfg.APIHost, a.cfg.ClientID, a.cfg.ClientSecret, store, apiclient.WithTimeout(a.cfg.RequestTimeout))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		if status == 0 {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, apiResponse{
			"error":   apiErr.Message,
			"code":    apiErr.Code,
			"status":  apiErr.StatusCode,
			"details": apiErr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusBadGateway, apiResponse{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, apiResponse{"error": "method not allowed"})
}

func queryInt64(r *http.Request, key string, fallback int64) int64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func queryString(r *http.Request, key, fallback string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	return value
}

func nextCursor(listInfo *model.ScanListRequest) string {
	if listInfo == nil {
		return ""
	}
	return listInfo.Token
}

func normalizeFileList(resp *userfile.FileListResponse) apiResponse {
	return apiResponse{
		"items":       resp.Files,
		"next_cursor": nextCursor(resp.ListInfo),
		"has_more":    nextCursor(resp.ListInfo) != "",
	}
}

func sanitizeUser(u *user.User) apiResponse {
	if u == nil {
		return apiResponse{}
	}
	return apiResponse{
		"identity":  u.Identity,
		"name":      u.Name,
		"icon":      u.Icon,
		"type":      u.Type,
		"status":    u.Status,
		"create_ts": u.CreateTs,
		"update_ts": u.UpdateTs,
	}
}
