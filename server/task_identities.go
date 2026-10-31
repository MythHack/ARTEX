package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Autumn-27/artex/db"
)

func parseTaskIDFromPath(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func parseIdentityBody(r *http.Request) (*db.TaskIdentity, error) {
	var it db.TaskIdentity
	if err := decode(r, &it); err != nil {
		return nil, err
	}
	it.Name = strings.TrimSpace(it.Name)
	if it.Name == "" {
		return nil, errIdentityNameRequired
	}
	switch it.Kind {
	case "password", "cookie", "token", "header":
	default:
		return nil, errIdentityKindInvalid
	}
	return &it, nil
}

var (
	errIdentityNameRequired = errText("身份名称不能为空")
	errIdentityKindInvalid  = errText("身份类型必须是 password/cookie/token/header")
)

type errText string

func (e errText) Error() string { return string(e) }

func (s *Server) listTaskIdentities(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseTaskIDFromPath(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid task id")
		return
	}
	items, err := s.m.pg.ListTaskIdentities(taskID, false)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createTaskIdentity(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseTaskIDFromPath(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid task id")
		return
	}
	it, err := parseIdentityBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	it.TaskID = taskID
	it.Enabled = true
	id, err := s.m.pg.CreateTaskIdentity(it)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	it.ID = id
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) updateTaskIdentity(w http.ResponseWriter, r *http.Request) {
	it, err := parseIdentityBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := strconv.ParseInt(r.PathValue("identity_id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid identity id")
		return
	}
	it.ID = id
	if err := s.m.pg.UpdateTaskIdentity(it); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) deleteTaskIdentity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("identity_id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid identity id")
		return
	}
	if err := s.m.pg.DeleteTaskIdentity(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}
