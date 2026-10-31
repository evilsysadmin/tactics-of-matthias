package tacticsengine

// StageMoveEntry is the server-only first phase of a real move resolution:
// 1) verify the move against Run.State, 2) load *private* authored map content,
// 3) apply deterministic on-entry traps, and 4) return an UNCOMMITTED stage.
// Initiative/encounters/enemy AI and final CAS remain mandatory later phases.
import (
 "context"
 "encoding/json"
 "errors"
 "math"
 "reflect"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var ErrInvalidContent = errors.New("server-authored Tactics content is unavailable or mismatched")

type PrivateMapContent struct {
 MapID string
 Grid []string
 Traps []tacticsrules.AuthoredTrap
}

// ContentCatalog must be an AUTHENTICATED SERVER dependency: never source
// this from a browser request or expose it through Run.Map/bootstrap JSON.
type ContentCatalog interface{
 LoadMapContent(context.Context,string)(PrivateMapContent,error)
}
type MoveStage struct{
 Pending tacticsapi.Run
 TriggeredTrapID string
 AdvancesTurn bool
}

// The pending world deliberately retains the SAME worldVersion as the old
// run, so it cannot pass Engine.validNext or be CAS-persisted before the full
// encounter/AI/combat reducer has finished and advanced the version.
func StageMoveEntry(ctx context.Context,run tacticsapi.Run,
 decision tacticsrules.Decision,catalog ContentCatalog)(MoveStage,error){
 if catalog==nil{return MoveStage{},ErrInvalidContent}
 state,board,err:=DecodeState(run)
 if err!=nil{return MoveStage{},err}
 actual,err:=tacticsrules.DecideMove(board,decision.MemberID,decision.Move.To)
 if err!=nil || actual!=decision{return MoveStage{},tacticsapi.ErrIllegalAction}
 content,err:=catalog.LoadMapContent(ctx,run.MapID)
 if err!=nil{return MoveStage{},ErrInvalidContent}
 if content.MapID!=run.MapID || !reflect.DeepEqual(content.Grid,run.Map.Grid){
  return MoveStage{},ErrInvalidContent
 }
 if err:=tacticsrules.ValidateAuthoredTraps(content.Grid,content.Traps);err!=nil{
  return MoveStage{},ErrInvalidContent
 }
 next:=state
 if decision.Combat{
  if next.Turns==math.MaxInt{return MoveStage{},ErrCorruptRun}
  next.Turns++
  updated:=make(map[string]tacticsrules.Cell,len(state.PartyPositions))
  for id,cell:=range state.PartyPositions{updated[id]=cell}
  updated[decision.MemberID]=decision.Move.To
  next.PartyPositions=updated
 }else{
  next.X,next.Y=decision.Move.To.X,decision.Move.To.Y
  next.Direction=decision.Move.Facing
  // Exploratory movement deliberately DOES NOT consume a turn.
 }
 applied,err:=tacticsrules.ApplyEntryTrap(content.Grid,content.Traps,
  tacticsrules.TrapState{Phase:next.Phase,Flags:next.Flags,Party:next.Party},
  decision.Move.To)
 if err!=nil{return MoveStage{},ErrInvalidContent}
 next.Flags=applied.State.Flags
 next.Party=applied.State.Party
 pending:=run
 if applied.State.Phase=="defeated"{
  next.Phase="defeated"
  next.Initiative=nil
  next.PartyPositions=nil
  next.TurnPhase="party"
  pending.Status="defeated"
 }
 snapshot,err:=json.Marshal(next)
 if err!=nil{return MoveStage{},ErrCorruptRun}
 pending.State=snapshot
 // Validates the terminal and active snapshots as well as all identities.
 if _,_,err:=DecodeState(pending);err!=nil{return MoveStage{},err}
 return MoveStage{
  Pending:pending,TriggeredTrapID:applied.TriggeredID,
  AdvancesTurn:decision.AdvancesTurn,
 },nil
}
