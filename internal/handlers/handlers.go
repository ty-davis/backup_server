package handlers

import (
	"backup_server/internal/auth"
	"backup_server/internal/database"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type Handler struct {
	DB       *database.DB
	Sessions *auth.SessionStore
	Templates *template.Template
}

func NewHandler(db *database.DB, sessions *auth.SessionStore) *Handler {
	funcMap := template.FuncMap{
		"hasSuffix": func(s, suffix string) bool {
			return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
		},
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))
	return &Handler{
		DB:       db,
		Sessions: sessions,
		Templates: tmpl,
	}
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.Templates.ExecuteTemplate(w, "login.html", nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.DB.ValidateUser(username, password)
	if err != nil {
		h.Templates.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Invalid credentials"})
		return
	}

	sessionID, err := h.Sessions.Create(user.ID, user.Username, user.GroupIDs)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, sessionID)
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, err := auth.GetSessionFromRequest(r)
	if err == nil {
		h.Sessions.Delete(sessionID)
	}
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) FilesPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	files, err := h.DB.GetFilesByGroupIDs(session.GroupIDs)
	if err != nil {
		http.Error(w, "Failed to load files", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Username": session.Username,
		"Files":    files,
		"IsAdmin":  h.isAdmin(session),
	}

	h.Templates.ExecuteTemplate(w, "files.html", data)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	fileIDStr := r.URL.Query().Get("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	file, err := h.DB.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	hasAccess, err := h.DB.UserHasAccessToGroup(session.UserID, file.GroupID)
	if err != nil {
		http.Error(w, "Failed to check access", http.StatusInternalServerError)
		return
	}

	if !hasAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	f, err := os.Open(file.FilePath)
	if err != nil {
		log.Printf("Failed to open file %s: %v", file.FilePath, err)
		http.Error(w, "File not accessible", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to read file info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filepath.Base(file.Name))))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	io.Copy(w, f)
}

// ServeWorldFile serves .wld files for TerraMap with proper authentication
func (h *Handler) ServeWorldFile(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	fileIDStr := r.URL.Query().Get("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	file, err := h.DB.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Check user has access to this file's group
	hasAccess, err := h.DB.UserHasAccessToGroup(session.UserID, file.GroupID)
	if err != nil || !hasAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Open and serve the file
	f, err := os.Open(file.FilePath)
	if err != nil {
		log.Printf("Failed to open world file %s: %v", file.FilePath, err)
		http.Error(w, "File not accessible", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to read file info", http.StatusInternalServerError)
		return
	}

	// Set headers for binary file download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Cache-Control", "no-cache")

	io.Copy(w, f)
}

// TerraMapViewer serves the TerraMap viewer page for .wld files
func (h *Handler) TerraMapViewer(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	fileIDStr := r.URL.Query().Get("id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	file, err := h.DB.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Check user has access to this file's group
	hasAccess, err := h.DB.UserHasAccessToGroup(session.UserID, file.GroupID)
	if err != nil || !hasAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Render the TerraMap viewer
	data := map[string]interface{}{
		"FileID":   fileID,
		"FileName": file.Name,
	}

	err = h.Templates.ExecuteTemplate(w, "terramap.html", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render viewer", http.StatusInternalServerError)
	}
}

func (h *Handler) AdminPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	hasAdminAccess := false
	for _, groupID := range session.GroupIDs {
		if access, _ := h.DB.UserHasAccessToGroup(session.UserID, groupID); access {
			groups, _ := h.DB.GetAllGroups()
			for _, g := range groups {
				if g.ID == groupID && g.Name == "admins" {
					hasAdminAccess = true
					break
				}
			}
		}
	}

	if !hasAdminAccess {
		http.Error(w, "Access denied - Admin privileges required", http.StatusForbidden)
		return
	}

	files, err := h.DB.GetAllFiles()
	if err != nil {
		http.Error(w, "Failed to load files", http.StatusInternalServerError)
		return
	}

	groups, err := h.DB.GetAllGroups()
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}

	groupNames := make(map[int]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	data := map[string]interface{}{
		"Username":   session.Username,
		"Files":      files,
		"Groups":     groups,
		"GroupNames": groupNames,
	}

	if msg := r.URL.Query().Get("success"); msg != "" {
		data["Message"] = msg
		data["Success"] = true
	} else if msg := r.URL.Query().Get("error"); msg != "" {
		data["Message"] = msg
		data["Success"] = false
	}

	h.Templates.ExecuteTemplate(w, "admin.html", data)
}

func (h *Handler) AdminAddFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	hasAdminAccess := h.isAdmin(session)
	if !hasAdminAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")
	filePath := r.FormValue("file_path")
	groupID, _ := strconv.Atoi(r.FormValue("group_id"))
	description := r.FormValue("description")

	err := h.DB.AddFile(name, filePath, groupID, description)
	if err != nil {
		log.Printf("Failed to add file: %v", err)
		http.Redirect(w, r, "/admin/files?error=Failed+to+add+file", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/files?success=File+added+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminEditFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	hasAdminAccess := h.isAdmin(session)
	if !hasAdminAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fileID, _ := strconv.Atoi(r.FormValue("id"))
	name := r.FormValue("name")
	filePath := r.FormValue("file_path")
	groupID, _ := strconv.Atoi(r.FormValue("group_id"))
	description := r.FormValue("description")

	err := h.DB.UpdateFile(fileID, name, filePath, groupID, description)
	if err != nil {
		log.Printf("Failed to update file: %v", err)
		http.Redirect(w, r, "/admin/files?error=Failed+to+update+file", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/files?success=File+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	hasAdminAccess := h.isAdmin(session)
	if !hasAdminAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fileID, _ := strconv.Atoi(r.FormValue("id"))

	err := h.DB.DeleteFile(fileID)
	if err != nil {
		log.Printf("Failed to delete file: %v", err)
		http.Redirect(w, r, "/admin/files?error=Failed+to+delete+file", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/files?success=File+deleted+successfully", http.StatusSeeOther)
}

func (h *Handler) isAdmin(session *auth.Session) bool {
	groups, err := h.DB.GetAllGroups()
	if err != nil {
		return false
	}

	for _, groupID := range session.GroupIDs {
		for _, g := range groups {
			if g.ID == groupID && g.Name == "admins" {
				return true
			}
		}
	}
	return false
}

func (h *Handler) AdminUsersPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	if !h.isAdmin(session) {
		http.Error(w, "Access denied - Admin privileges required", http.StatusForbidden)
		return
	}

	users, err := h.DB.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to load users", http.StatusInternalServerError)
		return
	}

	groups, err := h.DB.GetAllGroups()
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}

	groupNames := make(map[int]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	data := map[string]interface{}{
		"Username":   session.Username,
		"Users":      users,
		"Groups":     groups,
		"GroupNames": groupNames,
	}

	if msg := r.URL.Query().Get("success"); msg != "" {
		data["Message"] = msg
		data["Success"] = true
	} else if msg := r.URL.Query().Get("error"); msg != "" {
		data["Message"] = msg
		data["Success"] = false
	}

	h.Templates.ExecuteTemplate(w, "admin_users.html", data)
}

func (h *Handler) AdminAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	
	r.ParseForm()
	groupIDStrs := r.Form["group_ids"]
	var groupIDs []int
	for _, gidStr := range groupIDStrs {
		gid, _ := strconv.Atoi(gidStr)
		groupIDs = append(groupIDs, gid)
	}

	if len(groupIDs) == 0 {
		http.Redirect(w, r, "/admin/users?error=User+must+belong+to+at+least+one+group", http.StatusSeeOther)
		return
	}

	err := h.DB.CreateUser(username, password, groupIDs)
	if err != nil {
		log.Printf("Failed to add user: %v", err)
		http.Redirect(w, r, "/admin/users?error=Failed+to+add+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+added+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminEditUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	userID, _ := strconv.Atoi(r.FormValue("id"))
	username := r.FormValue("username")
	
	r.ParseForm()
	groupIDStrs := r.Form["group_ids"]
	var groupIDs []int
	for _, gidStr := range groupIDStrs {
		gid, _ := strconv.Atoi(gidStr)
		groupIDs = append(groupIDs, gid)
	}

	if len(groupIDs) == 0 {
		http.Redirect(w, r, "/admin/users?error=User+must+belong+to+at+least+one+group", http.StatusSeeOther)
		return
	}

	err := h.DB.UpdateUser(userID, username, groupIDs)
	if err != nil {
		log.Printf("Failed to update user: %v", err)
		http.Redirect(w, r, "/admin/users?error=Failed+to+update+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	userID, _ := strconv.Atoi(r.FormValue("id"))
	password := r.FormValue("password")

	err := h.DB.UpdateUserPassword(userID, password)
	if err != nil {
		log.Printf("Failed to update password: %v", err)
		http.Redirect(w, r, "/admin/users?error=Failed+to+update+password", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?success=Password+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	userID, _ := strconv.Atoi(r.FormValue("id"))

	if userID == session.UserID {
		http.Redirect(w, r, "/admin/users?error=Cannot+delete+your+own+account", http.StatusSeeOther)
		return
	}

	err := h.DB.DeleteUser(userID)
	if err != nil {
		log.Printf("Failed to delete user: %v", err)
		http.Redirect(w, r, "/admin/users?error=Failed+to+delete+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+deleted+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminGroupsPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*auth.Session)

	if !h.isAdmin(session) {
		http.Error(w, "Access denied - Admin privileges required", http.StatusForbidden)
		return
	}

	groups, err := h.DB.GetAllGroups()
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}

	memberCounts := make(map[int]int)
	fileCounts := make(map[int]int)

	for _, g := range groups {
		memberCount, _ := h.DB.GetGroupMemberCount(g.ID)
		fileCount, _ := h.DB.GetGroupFileCount(g.ID)
		memberCounts[g.ID] = memberCount
		fileCounts[g.ID] = fileCount
	}

	data := map[string]interface{}{
		"Username":     session.Username,
		"Groups":       groups,
		"MemberCounts": memberCounts,
		"FileCounts":   fileCounts,
	}

	if msg := r.URL.Query().Get("success"); msg != "" {
		data["Message"] = msg
		data["Success"] = true
	} else if msg := r.URL.Query().Get("error"); msg != "" {
		data["Message"] = msg
		data["Success"] = false
	}

	h.Templates.ExecuteTemplate(w, "admin_groups.html", data)
}

func (h *Handler) AdminAddGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")

	_, err := h.DB.CreateGroup(name)
	if err != nil {
		log.Printf("Failed to add group: %v", err)
		http.Redirect(w, r, "/admin/groups?error=Failed+to+add+group", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/groups?success=Group+added+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminEditGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	groupID, _ := strconv.Atoi(r.FormValue("id"))
	name := r.FormValue("name")

	err := h.DB.UpdateGroup(groupID, name)
	if err != nil {
		log.Printf("Failed to update group: %v", err)
		http.Redirect(w, r, "/admin/groups?error=Failed+to+update+group", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/groups?success=Group+updated+successfully", http.StatusSeeOther)
}

func (h *Handler) AdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := r.Context().Value("session").(*auth.Session)
	if !h.isAdmin(session) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	groupID, _ := strconv.Atoi(r.FormValue("id"))

	fileCount, _ := h.DB.GetGroupFileCount(groupID)
	if fileCount > 0 {
		http.Redirect(w, r, "/admin/groups?error=Cannot+delete+group+with+files+assigned", http.StatusSeeOther)
		return
	}

	err := h.DB.DeleteGroup(groupID)
	if err != nil {
		log.Printf("Failed to delete group: %v", err)
		http.Redirect(w, r, "/admin/groups?error=Failed+to+delete+group", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/groups?success=Group+deleted+successfully", http.StatusSeeOther)
}
