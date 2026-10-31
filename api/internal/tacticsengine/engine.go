// Package tacticsengine bridges authenticated API runs to the real Tactics
// legality rules. This is not a complete game engine: the mandatory Effects
// port must resolve initiative, AI, authored content, damage and rewards.
// A caller cannot construct an engine without a complete effects resolver.
package tacticsengine

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var ErrCorruptRun=errors.New("invalid server-authored Tactics state")
var ErrInvalidOutcome=errors.New("invalid authoritative Tactics outcome")
const MaxStateBytes=256*1024

type Initiative struct{
 Round int `json:"round"`
 Cursor int `json:"cursor"`
 Order []tacticsrules.Actor `json:"order"`
}

// StateV1 is exclusively a server-persisted world. It is NEVER supplied to
// Apply by HTTP callers. The grid is not stored here: it comes from Run.Map,
// which belongs to the server-authorized run.
type StateV1 struct{
 Version int `json:"version"`
 MapID string `json:"mapId"`
 X int `json:"x"`
 Y int `json:"y"`
 Phase string `json:"phase"`
 TurnPhase string `json:"turnPhase"`
 Party []tacticsrules.PartyMember `json:"party"`
 PartyPositions map[string]tacticsrules.Cell `json:"partyPositions,omitempty"`
 Enemies []tacticsrules.Enemy `json:"enemies"`
 Initiative *Initiative `json:"initiative,omitempty"`
}

// Effects must implement all consequences of an already validated action.
// There is no production default: trap handling, content, enemy AI, status,
// rewards and actual scheduler advancement must be explicit.
type Effects interface{
 AttackProfiles(context.Context,tacticsapi.Run)(map[string]tacticsrules.AttackProfile,map[string]int,error)
 ResolveMove(context.Context,tacticsapi.Run,tacticsrules.Decision)(tacticsapi.Run,error)
 ResolveAttack(context.Context,tacticsapi.Run,tacticsrules.AttackDecision)(tacticsapi.Run,error)
}
type Engine struct{effects Effects}
var _ tacticsapi.Engine=(*Engine)(nil)

func New(effects Effects)(*Engine,error){
 if effects==nil{return nil,errors.New("real Tactics gameplay effects resolver is required")}
 return &Engine{effects:effects},nil
}
func present(known map[string]json.RawMessage,keys ...string)bool{
 for _,key:=range keys{
  data,ok:=known[key]
  if !ok || len(data)==0 || bytes.Equal(bytes.TrimSpace(data),[]byte("null")){return false}
 }
 return true
}
func inside(grid []string,cell tacticsrules.Cell)bool{
 if cell.Y<0 || cell.Y>=len(grid) || len(grid)==0{return false}
 if cell.X<0 || cell.X>=len(grid[cell.Y]){return false}
 return grid[cell.Y][cell.X]!='#' && grid[cell.Y][cell.X]!='X'
}
func validMap(run tacticsapi.Run)bool{
 if run.ID=="" || run.Owner=="" || run.MapID=="" || run.Map.ID!=run.MapID{return false}
 grid:=run.Map.Grid
 if len(grid)==0 || len(grid)>256 || len(grid[0])==0 || len(grid[0])>256{return false}
 width:=len(grid[0])
 for _,row:=range grid{
  if len(row)!=width{return false}
  for _,tile:=range []byte(row){if tile<33 || tile>126{return false}}
 }
 return true
}

// DecodeState validates a SERVER-stored, versioned run and builds a rules
// snapshot from it. Unknown schema versions/properties and client-authored
// terrain are rejected rather than producing default heroes or positions.
func DecodeState(run tacticsapi.Run)(StateV1,tacticsrules.Snapshot,error){
 var zero StateV1
 if !validMap(run) || len(run.State)==0 || len(run.State)>MaxStateBytes ||
  !json.Valid(run.State){return zero,tacticsrules.Snapshot{},ErrCorruptRun}
 var keys map[string]json.RawMessage
 if err:=json.Unmarshal(run.State,&keys);err!=nil || !present(keys,
  "version","mapId","x","y","phase","party","enemies"){
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 decoder:=json.NewDecoder(bytes.NewReader(run.State))
 decoder.DisallowUnknownFields()
 var state StateV1
 if err:=decoder.Decode(&state);err!=nil{return zero,tacticsrules.Snapshot{},ErrCorruptRun}
 var trailing any
 if err:=decoder.Decode(&trailing);err!=io.EOF{
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 if state.Version!=1 || state.MapID!=run.MapID ||
  len(state.Party)==0 || len(state.Party)>16 || len(state.Enemies)>256 ||
  !inside(run.Map.Grid,tacticsrules.Cell{X:state.X,Y:state.Y}){
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 if run.Status!="active" || (state.Phase!="explore" && state.Phase!="combat"){
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 if state.TurnPhase!="" && state.TurnPhase!="party" && state.TurnPhase!="enemy"{
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 unique:=map[string]bool{}
 occupied:=map[tacticsrules.Cell]bool{}
 aliveParty:=0
 for _,member:=range state.Party{
  if member.ID=="" || member.HP<0 || unique["party:"+member.ID]{
   return zero,tacticsrules.Snapshot{},ErrCorruptRun
  }
  unique["party:"+member.ID]=true
  if member.HP>0{aliveParty++}
  if state.Phase=="combat" && member.HP>0{
   cell,ok:=state.PartyPositions[member.ID]
   if !ok || !inside(run.Map.Grid,cell) || occupied[cell]{
    return zero,tacticsrules.Snapshot{},ErrCorruptRun
   }
   occupied[cell]=true
  }
 }
 if aliveParty==0{return zero,tacticsrules.Snapshot{},ErrCorruptRun}
 for _,enemy:=range state.Enemies{
  if enemy.ID=="" || enemy.HP<0 || unique["enemy:"+enemy.ID]{
   return zero,tacticsrules.Snapshot{},ErrCorruptRun
  }
  unique["enemy:"+enemy.ID]=true
  if enemy.HP>0{
   if !inside(run.Map.Grid,enemy.Cell) || occupied[enemy.Cell]{
    return zero,tacticsrules.Snapshot{},ErrCorruptRun
   }
   occupied[enemy.Cell]=true
  }
 }
 var order []tacticsrules.Actor
 var cursor int
 if state.Phase=="combat"{
  if state.Initiative==nil || len(state.Initiative.Order)==0 ||
   state.Initiative.Round<1 || state.Initiative.Cursor<0 ||
   state.Initiative.Cursor>=len(state.Initiative.Order) {
   return zero,tacticsrules.Snapshot{},ErrCorruptRun
  }
  seen:=map[string]bool{}
  for _,actor:=range state.Initiative.Order{
   key:=actor.Kind+":"+actor.ID
   if (actor.Kind!="party" && actor.Kind!="enemy") ||
    !unique[key] || seen[key]{return zero,tacticsrules.Snapshot{},ErrCorruptRun}
   seen[key]=true
  }
  for _,enemy:=range state.Enemies{
   if enemy.HP>0 && !seen["enemy:"+enemy.ID]{
    return zero,tacticsrules.Snapshot{},ErrCorruptRun
   }
  }
  order=append([]tacticsrules.Actor(nil),state.Initiative.Order...)
  cursor=state.Initiative.Cursor
 }else if state.Initiative!=nil{
  return zero,tacticsrules.Snapshot{},ErrCorruptRun
 }
 board:=tacticsrules.Snapshot{
  Grid:append([]string(nil),run.Map.Grid...),
  Anchor:tacticsrules.Cell{X:state.X,Y:state.Y},
  Phase:state.Phase,TurnPhase:state.TurnPhase,
  Party:append([]tacticsrules.PartyMember(nil),state.Party...),
  PartyPositions:state.PartyPositions,
  Enemies:append([]tacticsrules.Enemy(nil),state.Enemies...),
  Initiative:order,InitiativeCursor:cursor,
 }
 return state,board,nil
}
func validNext(before,next tacticsapi.Run)error{
 if next.ID!=before.ID || next.Owner!=before.Owner ||
  next.WorldVersion!=before.WorldVersion+1 || next.MapID=="" ||
  next.Map.ID!=next.MapID{return ErrInvalidOutcome}
 if next.Status!="active"{
  // Terminal transitions need their own fully validated reducer/contract;
  // this first slice deliberately cannot fabricate a completed campaign.
  return ErrInvalidOutcome
 }
 if _,_,err:=DecodeState(next);err!=nil{return ErrInvalidOutcome}
 if bytes.Equal(before.State,next.State) && before.MapID==next.MapID{
  return ErrInvalidOutcome
 }
 return nil
}
func (e *Engine) Apply(ctx context.Context,run tacticsapi.Run,action tacticsapi.Action)(tacticsapi.Run,error){
 if e==nil || e.effects==nil{return tacticsapi.Run{},errors.New("Tactics effects not configured")}
 _,board,err:=DecodeState(run)
 if err!=nil{return tacticsapi.Run{},err}
 var next tacticsapi.Run
 switch action.Kind{
 case "move":
  if action.X==nil || action.Y==nil || action.EnemyID!=""{
   return tacticsapi.Run{},tacticsapi.ErrIllegalAction
  }
  decision,err:=tacticsrules.DecideMove(board,action.MemberID,
   tacticsrules.Cell{X:*action.X,Y:*action.Y})
  if err!=nil{
   if errors.Is(err,tacticsrules.ErrIllegalMove){return tacticsapi.Run{},tacticsapi.ErrIllegalAction}
   return tacticsapi.Run{},fmt.Errorf("Tactics movement state: %w",err)
  }
  next,err=e.effects.ResolveMove(ctx,run,decision)
  if err!=nil{return tacticsapi.Run{},err}
 case "attack":
  if action.EnemyID=="" || action.X!=nil || action.Y!=nil{
   return tacticsapi.Run{},tacticsapi.ErrIllegalAction
  }
  profiles,bonus,err:=e.effects.AttackProfiles(ctx,run)
  if err!=nil{return tacticsapi.Run{},err}
  decision,err:=tacticsrules.DecideAttack(tacticsrules.AttackInput{
   Board:board,Profiles:profiles,ReachBonus:bonus,
  },action.MemberID,action.EnemyID)
  if err!=nil{
   if errors.Is(err,tacticsrules.ErrIllegalMove)||errors.Is(err,tacticsrules.ErrIllegalAttack){
    return tacticsapi.Run{},tacticsapi.ErrIllegalAction
   }
   return tacticsapi.Run{},fmt.Errorf("Tactics attack state: %w",err)
  }
  next,err=e.effects.ResolveAttack(ctx,run,decision)
  if err!=nil{return tacticsapi.Run{},err}
 default:
  return tacticsapi.Run{},tacticsapi.ErrIllegalAction
 }
 if err:=validNext(run,next);err!=nil{return tacticsapi.Run{},err}
 return next,nil
}
