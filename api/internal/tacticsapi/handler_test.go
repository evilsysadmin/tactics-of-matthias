package tacticsapi

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
)

type tokenAuth struct{}
func (tokenAuth) Authenticate(_ context.Context, token string) (string, error) {
    switch token {
    case "signed-alice": return "alice", nil
    case "signed-bob": return "bob", nil
    }
    return "", errors.New("invalid token")
}
type memoryStore struct {
    mu sync.Mutex
    runs map[string]Run
    loaded int
    swaps int
}
func newStore() *memoryStore {
    return &memoryStore{runs:map[string]Run{
        "alice:run-1": {
            ID:"run-1", Owner:"alice", MapID:"crypt", WorldVersion:2,
            Status:"active", Map:Map{ID:"crypt", Title:"Crypt", Grid:[]string{"####","#..#","####"}},
            State:json.RawMessage(`{"x":1,"y":1,"turn":0}`),
        },
    }}
}
func (s *memoryStore) Load(_ context.Context, owner, id string) (Run,bool,error) {
    s.mu.Lock(); defer s.mu.Unlock()
    s.loaded++
    run,ok:=s.runs[owner+":"+id]
    return run,ok,nil
}
func (s *memoryStore) CompareAndSwap(_ context.Context, owner,id string,expected int64,next Run) (bool,error) {
    s.mu.Lock(); defer s.mu.Unlock()
    s.swaps++
    old,ok:=s.runs[owner+":"+id]
    if !ok || old.WorldVersion!=expected || next.Owner!=owner || next.ID!=id {return false,nil}
    s.runs[owner+":"+id]=next
    return true,nil
}
type rules struct{ calls int }
func (e *rules) Apply(_ context.Context, run Run, action Action) (Run,error) {
    e.calls++
    if action.Kind!="move" || *action.X!=2 || *action.Y!=1 {
        return Run{},ErrIllegalAction
    }
    run.WorldVersion++
    run.State=json.RawMessage(`{"x":2,"y":1,"turn":1}`)
    return run,nil
}
func req(method,path,token,payload string) *http.Request {
    r:=httptest.NewRequest(method,path,strings.NewReader(payload))
    if token!="" {r.Header.Set("Authorization","Bearer "+token)}
    return r
}
func call(h http.Handler,r *http.Request) *httptest.ResponseRecorder {
    w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w
}
func newHandler(t *testing.T) (*Handler,*memoryStore,*rules) {
    t.Helper()
    store:=newStore();engine:=&rules{}
    handler,err:=NewHandler(tokenAuth{},store,engine)
    if err!=nil {t.Fatal(err)}
    return handler,store,engine
}
func TestRequiresConfiguredAuthority(t *testing.T) {
    for _,c:=range []struct{auth Authenticator;store Store;engine Engine}{
        {nil,newStore(),&rules{}},{tokenAuth{},nil,&rules{}},{tokenAuth{},newStore(),nil},
    } {
        if _,err:=NewHandler(c.auth,c.store,c.engine);err==nil {
            t.Fatal("handler accepted missing authority")
        }
    }
}
func TestBootstrapIsOwnerScopedAndNeverLeaksOwner(t *testing.T) {
    h,store,_:=newHandler(t)
    for _,tc:=range []struct{token string;want int}{
        {"",401},{"unknown",401},{"signed-bob",404},{"signed-alice",200},
    } {
        w:=call(h,req("GET","/api/tactics/runs/run-1/bootstrap",tc.token,""))
        if w.Code!=tc.want {t.Fatalf("%q: status %d body %s",tc.token,w.Code,w.Body)}
        if w.Header().Get("Cache-Control")!="no-store" {t.Fatal("bootstrap is cacheable")}
        if tc.want==200 {
            var response struct{Source,RunID,CurrentMapID string;WorldVersion int64;State map[string]any}
            if err:=json.Unmarshal(w.Body.Bytes(),&response);err!=nil {t.Fatal(err)}
            if response.Source!="remote" || response.RunID!="run-1" || response.WorldVersion!=2 ||
                response.CurrentMapID!="crypt" {t.Fatalf("unexpected bootstrap: %+v",response)}
            if strings.Contains(w.Body.String(),"\"owner\"") {t.Fatal("internal owner was leaked")}
        }
    }
    if store.swaps!=0 {t.Fatal("GET mutated run")}
}
func TestActionAuthoritativeCASAndReplayConflict(t *testing.T) {
    h,store,engine:=newHandler(t)
    const move=`{"expectedWorldVersion":2,"action":{"kind":"move","memberId":"rook","x":2,"y":1}}`
    result:=call(h,req("POST","/api/tactics/runs/run-1/actions","signed-alice",move))
    if result.Code!=200 {t.Fatalf("first action: %d %s",result.Code,result.Body)}
    if !strings.Contains(result.Body.String(),"\"worldVersion\":3") {t.Fatal("world version did not advance")}
    repeated:=call(h,req("POST","/api/tactics/runs/run-1/actions","signed-alice",move))
    if repeated.Code!=409 {t.Fatalf("stale action: %d",repeated.Code)}
    denied:=call(h,req("POST","/api/tactics/runs/run-1/actions","signed-bob",move))
    if denied.Code!=404 {t.Fatalf("other owner: %d",denied.Code)}
    if engine.calls!=1 || store.swaps!=1 {t.Fatalf("engine=%d writes=%d",engine.calls,store.swaps)}
    saved,ok,_:=store.Load(context.Background(),"alice","run-1")
    if !ok || saved.WorldVersion!=3 {t.Fatal("CAS did not persist authoritative state")}
}
func TestIllegalOrMalformedActionsFailWithoutWrites(t *testing.T) {
    h,store,engine:=newHandler(t)
    payloads:=[]struct{body string;want int}{
        {`{}`,422},
        {`{"expectedWorldVersion":-1,"action":{"kind":"move","memberId":"rook","x":2,"y":1}}`,422},
        {`{"expectedWorldVersion":2,"action":{"kind":"wait","memberId":"rook"}}`,422},
        {`{"expectedWorldVersion":2,"action":{"kind":"move","memberId":"rook","x":2}}`,422},
        {`{"expectedWorldVersion":2,"action":{"kind":"attack","memberId":"rook","enemyId":"enemy","x":2}}`,422},
        {`{"expectedWorldVersion":2,"action":{"kind":"attack","memberId":"rook","enemyId":"enemy"}}`,422},
        {`{"expectedWorldVersion":2,"action":{"kind":"move","memberId":"rook","x":2,"y":1},"worldState":{"hp":9999}}`,400},
        {`{"expectedWorldVersion":2,"action":{"kind":"move","memberId":"rook","x":2,"y":1}} {}`,400},
    }
    for _,tc:=range payloads {
        result:=call(h,req("POST","/api/tactics/runs/run-1/actions","signed-alice",tc.body))
        if result.Code!=tc.want {t.Fatalf("status %d want %d for %q: %s",result.Code,tc.want,tc.body,result.Body)}
    }
    if store.swaps!=0 || engine.calls!=1 {t.Fatalf("unexpected engine/writes: %+v %+v",engine,store)}
}
func TestRejectsMissingBearerAndBadRoutes(t *testing.T) {
    h,store,_:=newHandler(t)
    for _,tc:=range []struct{method,path,token string;want int}{
        {"GET","/api/tactics/runs/run-1/bootstrap","",401},
        {"GET","/api/tactics/runs/../bootstrap","signed-alice",404},
        {"GET","/api/tactics/runs/run-1/actions","signed-alice",405},
        {"POST","/api/tactics/runs/run-1/bootstrap","signed-alice",405},
        {"GET","/api/chronicles/runs/run-1/bootstrap","signed-alice",404},
    } {
        w:=call(h,req(tc.method,tc.path,tc.token,""))
        if w.Code!=tc.want {t.Errorf("%s %s: got %d wanted %d",tc.method,tc.path,w.Code,tc.want)}
    }
    if store.swaps!=0 {t.Fatal("invalid routes wrote state")}
}
func TestEngineCannotChangeOwnershipOrFakeVersion(t *testing.T) {
    for _,change:=range []func(*Run){
        func(run *Run){run.Owner="bob"},
        func(run *Run){run.WorldVersion=10},
        func(run *Run){run.Map.ID="unapproved-map"},
    } {
        original:=newStore()
        engine:=corruptEngine{change:change}
        handler,_:=NewHandler(tokenAuth{},original,engine)
        w:=call(handler,req("POST","/api/tactics/runs/run-1/actions","signed-alice",
          `{"expectedWorldVersion":2,"action":{"kind":"move","memberId":"rook","x":2,"y":1}}`))
        if w.Code!=503 {t.Fatalf("invalid engine result accepted: %d",w.Code)}
        if original.swaps!=0 {t.Fatal("invalid engine result persisted")}
    }
}
type corruptEngine struct{change func(*Run)}
func (e corruptEngine) Apply(_ context.Context,run Run,_ Action)(Run,error){
    run.WorldVersion++;e.change(&run);return run,nil
}
