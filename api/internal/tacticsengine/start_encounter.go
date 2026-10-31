package tacticsengine

// StageEncounterStart composes the existing authoritative visibility,
// deployment and AGI+1d8 rules. It does NOT execute opening enemy AI turns,
// player actions or checkpoints; callers must finish those effects before
// publishing a new world version. This package does not expose an HTTP route.
import (
 "context"
 "encoding/json"
 "io"
 "reflect"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

type EncounterStage struct {
 Pending tacticsapi.Run
 InitialActor tacticsrules.Actor
 EngagedEnemyIDs []string
}

// Encounter profiles, agility and private content MUST be supplied by the
// authenticated server (not by Action, browser bootstrap or game URL).
// Unimplemented authored objects must be rejected by ContentCatalog, never
// quietly ignored when generating placement.
func StageEncounterStart(
 ctx context.Context,run tacticsapi.Run,catalog ContentCatalog,
 profiles map[string]tacticsrules.EnemyEngagement,
 agility tacticsrules.Agility,entropy io.Reader,
)(EncounterStage,error){
 if catalog==nil{return EncounterStage{},ErrInvalidContent}
 state,board,err:=DecodeState(run)
 if err!=nil{return EncounterStage{},err}
 if state.Phase!="explore"||state.Initiative!=nil||
  len(state.EngagedEnemyIDs)!=0||len(state.PartyPositions)!=0{
  return EncounterStage{},ErrCorruptRun
 }
 // Load the private manifest and bind it to this run's exact authored
 // revision, ID and geometry before allocating or rolling any actor.
 content,err:=catalog.LoadMapContent(ctx,run.MapID)
 if err!=nil{return EncounterStage{},ErrInvalidContent}
 if content.MapID!=run.MapID || content.Revision!=run.Map.Version ||
  !reflect.DeepEqual(content.Grid,run.Map.Grid){
  return EncounterStage{},ErrInvalidContent
 }
 if err:=tacticsrules.ValidateAuthoredTraps(content.Grid,content.Traps);err!=nil{
  return EncounterStage{},ErrInvalidContent
 }
 // The v1 catalog supports traps only. No "hidden object" placement
 // omission is possible because unknown authored keys are rejected.
 formation,err:=tacticsrules.PlanPartyDeployment(board,state.Direction,nil)
 if err!=nil{return EncounterStage{},err}
 rolled,err:=tacticsrules.RollEngagedInitiative(board,profiles,agility,entropy)
 if err!=nil{return EncounterStage{},err}
 if len(rolled.Initiative.Order)==0{
  return EncounterStage{},tacticsrules.ErrInvalidInitiative
 }
 order:=make([]tacticsrules.Actor,0,len(rolled.Initiative.Order))
 for _,entry:=range rolled.Initiative.Order{
  order=append(order,entry.Actor)
 }
 next:=state
 next.Phase="combat"
 next.TurnPhase="party"
 if order[0].Kind=="enemy"{next.TurnPhase="enemy"}
 next.PartyPositions=formation
 next.EngagedEnemyIDs=append([]string(nil),rolled.EngagedEnemyIDs...)
 next.Initiative=&Initiative{
  Round:rolled.Initiative.Round,
  Cursor:rolled.Initiative.Cursor,
  Order:order,
 }
 payload,err:=json.Marshal(next)
 if err!=nil{return EncounterStage{},ErrCorruptRun}
 pending:=run
 pending.State=payload
 if _,_,err:=DecodeState(pending);err!=nil{
  // Fail closed rather than emit a partially started combat stage.
  return EncounterStage{},err
 }
 // The old worldVersion is intentional: no CAS/HTTP response can commit an
 // encounter without the full enemy-action and content-effects reducer.
 return EncounterStage{
  Pending:pending,
  InitialActor:order[0],
  EngagedEnemyIDs:append([]string(nil),rolled.EngagedEnemyIDs...),
 },nil
}
