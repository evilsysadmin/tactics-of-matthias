package tacticsengine

// The non-damaging portion of an enemy's authored initiative action. This
// returns an UNCOMMITTED server-only stage and NEVER implements gameplay
// Effects on its own: a hit needs real damage/status/multi-strike handling.
import (
 "context"
 "encoding/json"
 "errors"
 "reflect"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var ErrEnemyAttackUnresolved = errors.New("Tactics enemy hit requires the real damage and status resolver")

type EnemyTurnStage struct{
 Pending tacticsapi.Run
 Intent tacticsrules.EnemyTurnIntent
 NextActor tacticsrules.Actor
}

// StageEnemyNonAttack permits ONLY move/wait and advances exactly one
// initiative slot (even if the next actor is another enemy). Its run's
// worldVersion remains unchanged, so the HTTP CAS contract cannot accept
// this partial combat turn. Profiles and catalogs are server dependencies.
func StageEnemyNonAttack(
 ctx context.Context,run tacticsapi.Run,catalog ContentCatalog,
 profiles map[string]tacticsrules.EnemyTurnProfile,
)(EnemyTurnStage,error){
 if catalog==nil{return EnemyTurnStage{},ErrInvalidContent}
 state,board,err:=DecodeState(run)
 if err!=nil{return EnemyTurnStage{},err}
 if state.Phase!="combat"||state.TurnPhase!="enemy"||
  state.Initiative==nil{return EnemyTurnStage{},ErrCorruptRun}
 content,err:=catalog.LoadMapContent(ctx,run.MapID)
 if err!=nil{return EnemyTurnStage{},ErrInvalidContent}
 if content.MapID!=run.MapID||content.Revision!=run.Map.Version||
  !reflect.DeepEqual(content.Grid,run.Map.Grid){
  return EnemyTurnStage{},ErrInvalidContent
 }
 if err:=tacticsrules.ValidateAuthoredTraps(content.Grid,content.Traps);err!=nil{
  return EnemyTurnStage{},ErrInvalidContent
 }
 intent,err:=tacticsrules.DecideEnemyTurn(
  board,state.EngagedEnemyIDs,state.Initiative.Round,profiles)
 if err!=nil{return EnemyTurnStage{},err}
 if intent.Kind=="attack"{
  // Never consume the enemy's initiative slot while its damage/status
  // resolution is missing. Not even a nominal zero-damage attack.
  return EnemyTurnStage{},ErrEnemyAttackUnresolved
 }
 if intent.Kind!="move"&&intent.Kind!="wait"{
  return EnemyTurnStage{},tacticsrules.ErrInvalidEnemyTurn
 }
 step,err:=tacticsrules.AdvanceEngagedTurn(
  board,state.Initiative.Round,state.EngagedEnemyIDs)
 if err!=nil||step.Ended||step.Next==nil{
  return EnemyTurnStage{},tacticsrules.ErrInvalidInitiative
 }
 next:=state
 next.Enemies=append([]tacticsrules.Enemy(nil),state.Enemies...)
 if intent.Kind=="move"{
  if intent.To==nil{
   return EnemyTurnStage{},tacticsrules.ErrInvalidEnemyTurn
  }
  found:=false
  for i:=range next.Enemies{
   if next.Enemies[i].ID==intent.EnemyID{
    next.Enemies[i].Cell=*intent.To
    found=true
    break
   }
  }
  if !found{return EnemyTurnStage{},tacticsrules.ErrInvalidEnemyTurn}
 }
 next.Initiative=&Initiative{
  Round:step.Round,Cursor:step.Cursor,
  Order:append([]tacticsrules.Actor(nil),step.Order...),
 }
 next.TurnPhase=step.Next.Kind
 raw,err:=json.Marshal(next)
 if err!=nil{return EnemyTurnStage{},ErrCorruptRun}
 pending:=run
 pending.State=raw
 if _,_,err:=DecodeState(pending);err!=nil{return EnemyTurnStage{},err}
 return EnemyTurnStage{Pending:pending,Intent:intent,
  NextActor:*step.Next},nil
}
