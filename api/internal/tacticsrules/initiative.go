package tacticsrules

// Pure server-side initiative scheduler, ported from
// chroniclesRollInitiative/chroniclesAdvanceCombatInitiative. Authoritative
// agility comes from server-owned character/enemy builds. An action's real
// damage, trap effects, AI and saving are deliberately NOT implemented here.

import (
 "crypto/rand"
 "errors"
 "io"
 "math"
 "math/big"
 "sort"
)

var ErrInvalidInitiative = errors.New("invalid Tactics initiative state")
const InitiativeDieSides=8
const maxInitiativeAgility=int64(math.MaxInt64-InitiativeDieSides)

type Agility struct {
 Party map[string]int64
 Enemies map[string]int64
}
type InitiativeEntry struct {
 Actor
 Agility int64 `json:"agility"`
 Roll int `json:"roll"`
 Initiative int64 `json:"initiative"`
}
type InitiativeRound struct {
 Version int `json:"version"`
 Die string `json:"die"`
 Round int `json:"round"`
 Cursor int `json:"cursor"`
 Order []InitiativeEntry `json:"order"`
}
type TurnStep struct {
 Phase string
 Round int
 Cursor int
 Order []Actor
 Next *Actor
 // Ending combat must also clear the tactical party formation and ephemeral
 // buffs, but the full state reducer (not this scheduler) owns that cleanup.
 Ended bool
}

func actorKey(actor Actor)string{return actor.Kind+":"+actor.ID}

func checkRoster(board Snapshot) error {
 if !validGrid(board.Grid){return ErrInvalidInitiative}
 party:=map[string]bool{}
 for _,member:=range board.Party {
  if member.ID=="" || member.HP<0 || party[member.ID]{return ErrInvalidInitiative}
  party[member.ID]=true
 }
 enemy:=map[string]bool{}
 for _,unit:=range board.Enemies{
  if unit.ID=="" || unit.HP<0 || enemy[unit.ID]{return ErrInvalidInitiative}
  enemy[unit.ID]=true
 }
 return nil
}

func rollD8(entropy io.Reader)(int,error){
 if entropy==nil{entropy=rand.Reader}
 die,err:=rand.Int(entropy,big.NewInt(InitiativeDieSides))
 if err!=nil{return 0,err}
 return int(die.Int64())+1,nil
}

// RollInitiative matches the Chronicles AGI+1d8 descending score, then AGI,
// roll, kind and ID tie-breaks. Unlike legacy save recovery it rejects
// duplicate roster IDs: new server-authored worlds must never invent turns.
func RollInitiative(board Snapshot,stats Agility,entropy io.Reader)(InitiativeRound,error){
 if err:=checkRoster(board);err!=nil{return InitiativeRound{},err}
 if board.Phase!="explore" && board.Phase!="combat"{return InitiativeRound{},ErrInvalidInitiative}
 entries:=make([]InitiativeEntry,0,len(board.Party)+len(board.Enemies))
 haveParty,haveEnemy:=false,false
 appendActor:=func(kind,id string,hp int,agilities map[string]int64)error{
  if hp==0{return nil}
  agility,ok:=agilities[id]
  if !ok || agility<0 || agility>maxInitiativeAgility{return ErrInvalidInitiative}
  roll,err:=rollD8(entropy);if err!=nil{return err}
  entries=append(entries,InitiativeEntry{
   Actor:Actor{Kind:kind,ID:id},Agility:agility,Roll:roll,
   Initiative:agility+int64(roll),
  })
  return nil
 }
 for _,member:=range board.Party{
  if member.HP>0{haveParty=true}
  if err:=appendActor("party",member.ID,member.HP,stats.Party);err!=nil{return InitiativeRound{},err}
 }
 for _,enemy:=range board.Enemies{
  if enemy.HP>0{haveEnemy=true}
  if err:=appendActor("enemy",enemy.ID,enemy.HP,stats.Enemies);err!=nil{return InitiativeRound{},err}
 }
 if !haveParty || !haveEnemy{return InitiativeRound{},ErrInvalidInitiative}
 sort.SliceStable(entries,func(i,j int)bool{
  a,b:=entries[i],entries[j]
  if a.Initiative!=b.Initiative{return a.Initiative>b.Initiative}
  if a.Agility!=b.Agility{return a.Agility>b.Agility}
  if a.Roll!=b.Roll{return a.Roll>b.Roll}
  if a.Kind!=b.Kind{return a.Kind<b.Kind}
  return a.ID<b.ID
 })
 return InitiativeRound{Version:1,Die:"1d8",Round:1,Cursor:0,Order:entries},nil
}

// AdvanceTurn selects the next living actor after the current slot, retaining
// the ORIGINAL round wrap boundary even when dead actors are pruned. This
// never executes an enemy AI turn or produces a saved checkpoint.
func AdvanceTurn(board Snapshot,round int)(TurnStep,error){
 if err:=checkRoster(board);err!=nil{return TurnStep{},err}
 if board.Phase!="combat" || len(board.Initiative)==0 ||
  board.InitiativeCursor<0 || board.InitiativeCursor>=len(board.Initiative) ||
  round<1 || round==math.MaxInt {return TurnStep{},ErrInvalidInitiative}
 alive:=map[string]bool{}
 partyAlive,enemyAlive:=false,false
 for _,member:=range board.Party {
  alive[actorKey(Actor{Kind:"party",ID:member.ID})]=member.HP>0
  partyAlive=partyAlive || member.HP>0
 }
 for _,enemy:=range board.Enemies {
  alive[actorKey(Actor{Kind:"enemy",ID:enemy.ID})]=enemy.HP>0
  enemyAlive=enemyAlive || enemy.HP>0
 }
 seen:=map[string]bool{}
 livingOrder:=make([]Actor,0,len(board.Initiative))
 inOrderEnemy:=false
 for _,actor:=range board.Initiative{
  key:=actorKey(actor)
  _,known:=alive[key]
  if !known || seen[key] || (actor.Kind!="party" && actor.Kind!="enemy"){
   return TurnStep{},ErrInvalidInitiative
  }
  seen[key]=true
  if alive[key]{
   livingOrder=append(livingOrder,actor)
   if actor.Kind=="enemy"{inOrderEnemy=true}
  }
 }
 // All living enemies must have a legitimate initiative slot; otherwise a
 // damaged/restored order could silently skip an enemy's entire turn.
 if enemyAlive && !inOrderEnemy{return TurnStep{},ErrInvalidInitiative}
 for _,enemy:=range board.Enemies{
  if enemy.HP>0 && !seen[actorKey(Actor{Kind:"enemy",ID:enemy.ID})]{
   return TurnStep{},ErrInvalidInitiative
  }
 }
 if !partyAlive{
  return TurnStep{Phase:"defeated",Round:round,Ended:true},nil
 }
 if !enemyAlive {
  return TurnStep{Phase:"explore",Round:round,Ended:true},nil
 }
 if len(livingOrder)==0{return TurnStep{},ErrInvalidInitiative}
 current:=board.InitiativeCursor
 for offset:=1;offset<=len(board.Initiative);offset++{
  index:=(current+offset)%len(board.Initiative)
  next:=board.Initiative[index]
  if !alive[actorKey(next)]{continue}
  at:=-1
  for i,actor:=range livingOrder{
   if actor==next{at=i;break}
  }
  if at<0{return TurnStep{},ErrInvalidInitiative}
  nextRound:=round
  if index<=current{nextRound++}
  picked:=next
  return TurnStep{Phase:"combat",Round:nextRound,Cursor:at,
   Order:livingOrder,Next:&picked},nil
 }
 return TurnStep{},ErrInvalidInitiative
}
