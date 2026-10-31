package tacticsengine

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var crypt=[]string{
 "#######","#..X..#","#.###.#","#...#.#","#.#S#.#","#P.E..#","#######",
}
func savedRun()tacticsapi.Run{
 state:=StateV1{
  Version:1,MapID:"crypt",X:1,Y:5,Phase:"explore",TurnPhase:"party",
  Party:[]tacticsrules.PartyMember{
   {ID:"matthias",HP:7},{ID:"rook",HP:10},{ID:"bishop",HP:6},{ID:"knight",HP:8},
  },
  Enemies:[]tacticsrules.Enemy{{ID:"pawn",HP:4,Cell:tacticsrules.Cell{X:3,Y:5}}},
 }
 data,_:=json.Marshal(state)
 return tacticsapi.Run{
  ID:"owned-run",Owner:"alice",MapID:"crypt",WorldVersion:8,
  Status:"active",Map:tacticsapi.Map{ID:"crypt",Title:"Crypt",Grid:crypt},State:data,
 }
}
type effects struct{
 moved int
 attacked int
 profilesCalled int
 corrupt bool
}
func (e *effects) AttackProfiles(_ context.Context,_ tacticsapi.Run)(map[string]tacticsrules.AttackProfile,map[string]int,error){
 e.profilesCalled++
 return map[string]tacticsrules.AttackProfile{
  "rook":{Pattern:"orthogonal",Reach:2},
 },nil,nil
}
func (e *effects) ResolveMove(_ context.Context,run tacticsapi.Run,dec tacticsrules.Decision)(tacticsapi.Run,error){
 e.moved++
 state,_,err:=DecodeState(run)
 if err!=nil{return tacticsapi.Run{},err}
 if dec.Combat{return tacticsapi.Run{},errors.New("combat effects require full engine")}
 state.X,state.Y=dec.Move.To.X,dec.Move.To.Y
 if e.corrupt{state.MapID="foreign-map"}
 data,_:=json.Marshal(state)
 run.State=data;run.WorldVersion++
 return run,nil
}
func (e *effects) ResolveAttack(_ context.Context,run tacticsapi.Run,dec tacticsrules.AttackDecision)(tacticsapi.Run,error){
 e.attacked++
 if dec.RequiresInitiative{
  // Nothing attacks for free. Real initiative/AI must be integrated first.
  return tacticsapi.Run{},errors.New("initiative effects are not configured")
 }
 return tacticsapi.Run{},errors.New("combat damage effects not configured")
}
func TestOwnedSnapshotUsesMapFromRunAndRejectsClientGrid(t *testing.T){
 run:=savedRun()
 state,board,err:=DecodeState(run)
 if err!=nil{t.Fatal(err)}
 if state.MapID!="crypt" || len(board.Grid)!=7 || board.Anchor!=(tacticsrules.Cell{X:1,Y:5}){
  t.Fatalf("snapshot mismatch %+v %+v",state,board)
 }
 var payload map[string]any
 if err:=json.Unmarshal(run.State,&payload);err!=nil{t.Fatal(err)}
 payload["grid"]=[]string{"..."};
 raw,_:=json.Marshal(payload);run.State=raw
 if _,_,err:=DecodeState(run);!errors.Is(err,ErrCorruptRun){
  t.Fatalf("snapshot accepted injected grid: %v",err)
 }
}
func TestInvalidSavedWorldFailsClosedBeforeEffects(t *testing.T){
 changes:=[]func(*tacticsapi.Run){
  func(run *tacticsapi.Run){run.Map.ID="other"},
  func(run *tacticsapi.Run){run.Status="defeated"},
  func(run *tacticsapi.Run){run.Map.Grid=[]string{"#.#","##"}},
  func(run *tacticsapi.Run){run.State=json.RawMessage(`{"version":2}`)},
  func(run *tacticsapi.Run){run.State=json.RawMessage("null")},
  func(run *tacticsapi.Run){run.State=json.RawMessage(`{"version":1,"mapId":"crypt","x":1,"y":5,"phase":"explore","party":[],"enemies":[]}`)},
 }
 e:=&effects{}
 engine,_:=New(e)
 x,y:=1,4
 for _,change:=range changes{
  run:=savedRun();change(&run)
  if _,err:=engine.Apply(context.Background(),run,
   tacticsapi.Action{Kind:"move",MemberID:"matthias",X:&x,Y:&y});err==nil{
   t.Errorf("corrupt stored state accepted: %+v",run)
  }
 }
 if e.moved!=0{t.Fatal("bad runs reached effects")}
}
func TestLegalMoveCrossesEngineBoundaryButInvalidMoveNeverDoes(t *testing.T){
 e:=&effects{}
 engine,err:=New(e);if err!=nil{t.Fatal(err)}
 run:=savedRun()
 x,y:=1,4
 next,err:=engine.Apply(context.Background(),run,tacticsapi.Action{
  Kind:"move",MemberID:"matthias",X:&x,Y:&y,
 })
 if err!=nil{t.Fatal(err)}
 if e.moved!=1 || next.WorldVersion!=9 || next.ID!=run.ID ||
  string(next.State)==string(run.State){t.Fatalf("wrong server transition: %+v",next)}
 if _,err:=engine.Apply(context.Background(),run,tacticsapi.Action{
  Kind:"move",MemberID:"matthias",X:&x,Y:&x,
 });!errors.Is(err,tacticsapi.ErrIllegalAction){
  t.Fatalf("diagonal/invalid position was accepted: %v",err)
 }
 if e.moved!=1{t.Fatal("illegal move reached effects")}
}
func TestCombatAndExplorationAttacksRequireEffectsNoFreeDamage(t *testing.T){
 e:=&effects{};engine,_:=New(e)
 run:=savedRun()
 _,err:=engine.Apply(context.Background(),run,tacticsapi.Action{
  Kind:"attack",MemberID:"rook",EnemyID:"pawn",
 })
 if err==nil || e.attacked!=1 || e.profilesCalled!=1{
  t.Fatalf("effect port was bypassed: %+v %v",e,err)
 }
 if _,err:=engine.Apply(context.Background(),run,tacticsapi.Action{
  Kind:"attack",MemberID:"matthias",EnemyID:"pawn",
 });!errors.Is(err,tacticsrules.ErrInvalidSnapshot){
  // Member profile missing is corrupt authoritative configuration, not legal damage.
  t.Fatalf("missing profile accepted or wrongly interpreted: %v",err)
 }
 if e.attacked!=1{t.Fatal("missing profile invoked damage")}
}
func TestInvalidEffectsCannotPersistFakeNextWorld(t *testing.T){
 e:=&effects{corrupt:true};engine,_:=New(e)
 run:=savedRun();x,y:=1,4
 _,err:=engine.Apply(context.Background(),run,tacticsapi.Action{
  Kind:"move",MemberID:"matthias",X:&x,Y:&y,
 })
 if !errors.Is(err,ErrInvalidOutcome){t.Fatalf("fake map outcome accepted: %v",err)}
 if _,err:=New(nil);err==nil{t.Fatal("built engine without gameplay effects")}
}
type auth struct{}
func (auth) Authenticate(_ context.Context,token string)(string,error){
 if token=="signed-alice"{return "alice",nil}
 return "",errors.New("unauthorized")
}
type memory struct{run tacticsapi.Run;writes int}
func (m *memory)Load(_ context.Context,owner,id string)(tacticsapi.Run,bool,error){
 if owner==m.run.Owner && id==m.run.ID{return m.run,true,nil}
 return tacticsapi.Run{},false,nil
}
func (m *memory)CompareAndSwap(_ context.Context,owner,id string,expected int64,next tacticsapi.Run)(bool,error){
 if owner!=m.run.Owner || id!=m.run.ID || expected!=m.run.WorldVersion{return false,nil}
 m.writes++;m.run=next;return true,nil
}
func TestHTTPToStoredRunToLegalityToEffectsToCAS(t *testing.T){
 e:=&effects{};engine,_:=New(e);store:=&memory{run:savedRun()}
 handler,err:=tacticsapi.NewHandler(auth{},store,engine);if err!=nil{t.Fatal(err)}
 action:=`{"expectedWorldVersion":8,"action":{"kind":"move","memberId":"matthias","x":1,"y":4}}`
 request:=func(body string)*http.Request{
  r:=httptest.NewRequest("POST","/api/tactics/runs/owned-run/actions",strings.NewReader(body))
  r.Header.Set("Authorization","Bearer signed-alice")
  return r
 }
 first:=httptest.NewRecorder();handler.ServeHTTP(first,request(action))
 if first.Code!=200 || store.writes!=1 || e.moved!=1{
  t.Fatalf("authenticated action not resolved: %d %s",first.Code,first.Body)
 }
 repeat:=httptest.NewRecorder();handler.ServeHTTP(repeat,request(action))
 if repeat.Code!=409 || store.writes!=1{t.Fatalf("replayed action: %d",repeat.Code)}
 invalid:=httptest.NewRecorder();handler.ServeHTTP(invalid,request(
  `{"expectedWorldVersion":9,"action":{"kind":"move","memberId":"matthias","x":2,"y":4}}`))
 if invalid.Code!=422 || store.writes!=1{
  t.Fatalf("illegal movement wrote checkpoint: %d",invalid.Code)
 }
}

func TestOrdinaryActionsCannotReplaceAuthoredMapOrRevision(t *testing.T){
 original:=savedRun()
 for _,change:=range []struct{
  name string
  alter func(*tacticsapi.Run)
 }{
  {"map geometry",func(r *tacticsapi.Run){
   r.Map.Grid=append([]string(nil),r.Map.Grid...)
   r.Map.Grid[1]="#....##"
  }},
  {"map revision",func(r *tacticsapi.Run){r.Map.Version=5}},
  {"map title",func(r *tacticsapi.Run){r.Map.Title="Unauthorized new campaign"}},
  {"map identifier",func(r *tacticsapi.Run){
   r.MapID="new-map";r.Map.ID="new-map"
  }},
 }{
  t.Run(change.name,func(t *testing.T){
   next:=original
   next.WorldVersion++
   next.State=json.RawMessage(`{"version":1,"mapId":"crypt","x":1,"y":4,"phase":"explore","turnPhase":"party","party":[{"id":"matthias","hp":7}],"enemies":[]}`)
   change.alter(&next)
   if err:=validNext(original,next);!errors.Is(err,ErrInvalidOutcome){
    t.Fatalf("regular action changed map without authoring: %v",err)
   }
  })
 }
}
