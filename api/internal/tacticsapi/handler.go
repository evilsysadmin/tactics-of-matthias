// Package tacticsapi defines the independent, owner-scoped HTTP boundary for
// Tactics campaigns. Authentication, storage and game rule execution are
// required dependencies. There are intentionally no production fake defaults.
package tacticsapi

import (
    "context"
    "encoding/json"
    "errors"
    "io"
    "net/http"
    "strings"
    "unicode/utf8"
)

var ErrIllegalAction = errors.New("illegal tactics action")

type Authenticator interface {
    // Authenticate verifies a valid, unexpired token for the TACTICS audience
    // and returns its stable authenticated subject; never trust request headers.
    Authenticate(context.Context, string) (string, error)
}
type Store interface {
    // Load and CompareAndSwap MUST scope every query by BOTH owner and run ID.
    // CompareAndSwap must update the world version atomically.
    Load(context.Context, string, string) (Run, bool, error)
    CompareAndSwap(context.Context, string, string, int64, Run) (bool, error)
}
type Engine interface {
    // Apply validates current actor, initiative, movement, attack reach, world
    // and outcomes against the server's stored state; client intents are hints.
    Apply(context.Context, Run, Action) (Run, error)
}

type Map struct {
    ID string `json:"id"`
    Version int `json:"version,omitempty"`
    Title string `json:"title"`
    Grid []string `json:"grid"`
}
type Run struct {
    ID string `json:"runId"`
    Owner string `json:"-"`
    MapID string `json:"currentMapId"`
    WorldVersion int64 `json:"worldVersion"`
    Status string `json:"runStatus"`
    Map Map `json:"map"`
    State json.RawMessage `json:"state"`
}
type Action struct {
    Kind string `json:"kind"`
    MemberID string `json:"memberId"`
    X *int `json:"x,omitempty"`
    Y *int `json:"y,omitempty"`
    EnemyID string `json:"enemyId,omitempty"`
}
type actionRequest struct {
    ExpectedWorldVersion *int64 `json:"expectedWorldVersion"`
    Action Action `json:"action"`
}
type Handler struct {
    auth Authenticator
    store Store
    engine Engine
}

func NewHandler(auth Authenticator, store Store, engine Engine) (*Handler, error) {
    if auth == nil || store == nil || engine == nil {
        return nil, errors.New("Tactics HTTP API requires verifier, owner-scoped store and authoritative engine")
    }
    return &Handler{auth: auth, store: store, engine: engine}, nil
}

func validID(value string) bool {
    if len(value) == 0 || len(value) > 128 { return false }
    for _, c := range value {
        if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
            (c >= '0' && c <= '9') || c == '-' || c == '_') { return false }
    }
    return true
}
func validRun(run Run) bool {
    if !validID(run.ID) || !validID(run.MapID) || !validID(run.Owner) ||
        run.Map.ID != run.MapID || run.Map.Version < 0 || run.WorldVersion < 0 ||
        (run.Status != "active" && run.Status != "completed" && run.Status != "defeated") ||
        !json.Valid(run.State) || len(run.State) == 0 { return false }
    height := len(run.Map.Grid)
    if height < 1 || height > 256 { return false }
    width := len(run.Map.Grid[0])
    if width == 0 || width > 256 || width*height > 65536 { return false }
    walkable := false
    for _, row := range run.Map.Grid {
        if len(row) != width || !utf8.ValidString(row) { return false }
        for _, cell := range row {
            if cell < 33 || cell > 126 { return false }
            if cell != '#' { walkable = true }
        }
    }
    return walkable
}
func validAction(action Action) bool {
    if !validID(action.MemberID) { return false }
    switch action.Kind {
    case "move":
        return action.X != nil && action.Y != nil && *action.X >= 0 && *action.Y >= 0 &&
            *action.X < 4096 && *action.Y < 4096 && action.EnemyID == ""
    case "attack":
        return validID(action.EnemyID) && action.X == nil && action.Y == nil
    default:
        return false
    }
}
func writeJSON(w http.ResponseWriter, code int, data any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(data)
}
func problem(w http.ResponseWriter, code int, message string) {
    writeJSON(w, code, map[string]string{"detail":message})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    const prefix = "/api/tactics/runs/"
    if !strings.HasPrefix(r.URL.Path, prefix) { problem(w, http.StatusNotFound, "Not found"); return }
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
    if len(parts) != 2 || !validID(parts[0]) {
        problem(w, http.StatusNotFound, "Not found"); return
    }
    method := ""
    switch parts[1] {
    case "bootstrap": method = http.MethodGet
    case "actions": method = http.MethodPost
    default: problem(w, http.StatusNotFound, "Not found"); return
    }
    if r.Method != method {
        w.Header().Set("Allow", method)
        problem(w, http.StatusMethodNotAllowed, "Method not allowed")
        return
    }
    values := r.Header.Values("Authorization")
    if len(values) != 1 || len(values[0]) > 8192 ||
        !strings.HasPrefix(values[0], "Bearer ") {
        problem(w, http.StatusUnauthorized, "Not authenticated"); return
    }
    token := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
    if token == "" { problem(w, http.StatusUnauthorized, "Not authenticated"); return }
    owner, err := h.auth.Authenticate(r.Context(), token)
    if err != nil || !validID(owner) {
        problem(w, http.StatusUnauthorized, "Not authenticated"); return
    }
    run, found, err := h.store.Load(r.Context(), owner, parts[0])
    if err != nil { problem(w, http.StatusServiceUnavailable, "Tactics storage unavailable"); return }
    if !found { problem(w, http.StatusNotFound, "Run not found"); return }
    if !validRun(run) || run.ID != parts[0] || run.Owner != owner {
        problem(w, http.StatusServiceUnavailable, "Invalid stored Tactics run"); return
    }
    if run.Status != "active" {
        problem(w, http.StatusConflict, "Run is not active"); return
    }
    if method == http.MethodGet {
        writeJSON(w, http.StatusOK, struct {
            Source string `json:"source"`
            Run
        }{Source: "remote", Run: run})
        return
    }

    r.Body = http.MaxBytesReader(w, r.Body, 4096)
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()
    var request actionRequest
    if err := decoder.Decode(&request); err != nil {
        problem(w, http.StatusBadRequest, "Invalid action payload"); return
    }
    var trailing any
    if err := decoder.Decode(&trailing); err != io.EOF {
        problem(w, http.StatusBadRequest, "Invalid action payload"); return
    }
    if request.ExpectedWorldVersion == nil || *request.ExpectedWorldVersion < 0 ||
        !validAction(request.Action) {
        problem(w, http.StatusUnprocessableEntity, "Invalid action"); return
    }
    if *request.ExpectedWorldVersion != run.WorldVersion {
        problem(w, http.StatusConflict, "World version conflict"); return
    }
    next, err := h.engine.Apply(r.Context(), run, request.Action)
    if errors.Is(err, ErrIllegalAction) {
        problem(w, http.StatusUnprocessableEntity, "Illegal action"); return
    }
    if err != nil { problem(w, http.StatusServiceUnavailable, "Tactics rules unavailable"); return }
    if next.ID != run.ID || next.Owner != owner ||
        next.WorldVersion != run.WorldVersion+1 || !validRun(next) {
        problem(w, http.StatusServiceUnavailable, "Invalid Tactics engine result"); return
    }
    saved, err := h.store.CompareAndSwap(r.Context(), owner, run.ID, run.WorldVersion, next)
    if err != nil { problem(w, http.StatusServiceUnavailable, "Tactics storage unavailable"); return }
    if !saved { problem(w, http.StatusConflict, "World version conflict"); return }
    writeJSON(w, http.StatusOK, struct {
        Source string `json:"source"`
        Run
    }{Source: "remote", Run: next})
}
