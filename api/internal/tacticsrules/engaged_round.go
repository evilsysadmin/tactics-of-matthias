package tacticsrules

// This gate joins the server-only enemy visibility selector to the existing
// AGI+1d8 scheduler. It never deploys the party, performs an enemy AI step or
// commits a run. Only enemies who actually engaged receive an initiative slot.
import (
 "errors"
 "io"
)

var ErrNoEngagement=errors.New("no visible enemy currently engages the party")

type EngagementRound struct {
 EngagedEnemyIDs []string
 Initiative InitiativeRound
}

// RollEngagedInitiative MUST receive validated authored enemy profiles and
// authoritative AGI. Unlike RollInitiative with the whole world roster, it
// rolls only the detected enemy subset, as Chronicles does at contact.
func RollEngagedInitiative(
 board Snapshot,profiles map[string]EnemyEngagement,stats Agility,entropy io.Reader,
)(EngagementRound,error){
 ids,err:=EngagedEnemies(board,profiles)
 if err!=nil{return EngagementRound{},err}
 if len(ids)==0{return EngagementRound{},ErrNoEngagement}
 selected:=make(map[string]bool,len(ids))
 for _,id:=range ids{selected[id]=true}
 scoped:=board
 scoped.Enemies=make([]Enemy,0,len(ids))
 for _,enemy:=range board.Enemies{
  if selected[enemy.ID]{scoped.Enemies=append(scoped.Enemies,enemy)}
 }
 rolled,err:=RollInitiative(scoped,stats,entropy)
 if err!=nil{return EngagementRound{},err}
 return EngagementRound{
  EngagedEnemyIDs:append([]string(nil),ids...),
  Initiative:rolled,
 },nil
}

// AdvanceEngagedTurn respects the original encounter roster, rather than
// forcing far-away live enemies into a fight. Engaged IDs MUST be persisted
// by the future real encounter reducer and never taken from an HTTP action.
//
// When all participating enemies die, combat ends even if other enemies on
// the map are still alive. Rejects missing/duplicate/phantom combat slots and
// missing live party actors instead of silently granting bonus turns.
func AdvanceEngagedTurn(board Snapshot,round int,engagedEnemyIDs []string)(TurnStep,error){
 if board.Phase!="combat"||len(engagedEnemyIDs)==0||
  len(engagedEnemyIDs)>len(board.Enemies){return TurnStep{},ErrInvalidInitiative}
 if err:=checkRoster(board);err!=nil{return TurnStep{},err}
 enemies:=make(map[string]Enemy,len(board.Enemies))
 for _,enemy:=range board.Enemies{enemies[enemy.ID]=enemy}
 picked:=make(map[string]bool,len(engagedEnemyIDs))
 for _,id:=range engagedEnemyIDs{
  if id==""||picked[id]{return TurnStep{},ErrInvalidInitiative}
  if _,ok:=enemies[id];!ok{return TurnStep{},ErrInvalidInitiative}
  picked[id]=true
 }
 selectedSlots:=make(map[string]bool,len(board.Initiative))
 for _,actor:=range board.Initiative{
  key:=actorKey(actor)
  if selectedSlots[key]{return TurnStep{},ErrInvalidInitiative}
  selectedSlots[key]=true
  if actor.Kind=="enemy"&&!picked[actor.ID]{
   return TurnStep{},ErrInvalidInitiative
  }
 }
 for _,id:=range engagedEnemyIDs{
  if !selectedSlots["enemy:"+id]{return TurnStep{},ErrInvalidInitiative}
 }
 for _,member:=range board.Party{
  if member.HP>0&&!selectedSlots["party:"+member.ID]{
   return TurnStep{},ErrInvalidInitiative
  }
 }
 scoped:=board
 scoped.Enemies=make([]Enemy,0,len(engagedEnemyIDs))
 for _,enemy:=range board.Enemies{
  if picked[enemy.ID]{scoped.Enemies=append(scoped.Enemies,enemy)}
 }
 return AdvanceTurn(scoped,round)
}
