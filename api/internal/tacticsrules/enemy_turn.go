package tacticsrules

// Enemy turn INTENT for the two supported Chronicles tactical movement
// policies: cardinal-chase and hold. This is a pure server-side decision;
// it does not deal damage, increment rounds, resolve statuses, or save runs.
// Unsupported authored AI policies are errors, not substitute "basic AI".
import (
 "errors"
 "sort"
)

var ErrInvalidEnemyTurn = errors.New("invalid authoritative Tactics enemy turn")

type EnemyTurnProfile struct {
 Movement string
 AttackReach int
 RequiresLineOfSight bool
}
type EnemyTurnIntent struct {
 EnemyID string
 Kind string // "attack", "move", "wait"
 From Cell
 To *Cell // only when Kind == "move"
 TargetID string // only when Kind == "attack"
}

type enemyTarget struct {
 ID string
 Cell Cell
 Distance int
 Row int
 Rank int
}

// Chronicles' party target rows, as defined in CHRONICLES_PARTY. This is
// canonical rules data; authored profiles/HP and positions still come from
// the authenticated server.
func tacticsFrontRow(id string)(int,bool){
 switch id{
 case "matthias","rook": return 0,true
 case "bishop","knight": return 1,true
 default: return 0,false
 }
}

func enemyCanStrike(board Snapshot,from,to Cell,profile EnemyTurnProfile)bool{
 distance:=manhattan(from,to)
 if distance<1||distance>profile.AttackReach{return false}
 if distance==1{return true}
 // The legacy Tactics enemy attack is cardinal for reach > 1.
 if from.X!=to.X&&from.Y!=to.Y{return false}
 if !profile.RequiresLineOfSight{return true}
 xStep,yStep:=sign(to.X-from.X),sign(to.Y-from.Y)
 for cell:= (Cell{X:from.X+xStep,Y:from.Y+yStep});cell!=to;{
  if tile(board,cell)=='#'{return false}
  cell.X+=xStep
  cell.Y+=yStep
 }
 return true
}

func enemyAttackTargets(board Snapshot,enemy Enemy,profile EnemyTurnProfile,round,enemyIndex int)(
 []enemyTarget,error,
){
 count:=0
 for _,member:=range board.Party{if member.HP>0{count++}}
 if count==0{return nil,ErrInvalidEnemyTurn}
 rotation:=(round%count+enemyIndex%count)%count
 livingIndex:=0
 candidates:=make([]enemyTarget,0,count)
 for _,member:=range board.Party{
  if member.HP==0{continue}
  row,known:=tacticsFrontRow(member.ID)
  if !known{return nil,ErrInvalidEnemyTurn}
  position,ok:=board.PartyPositions[member.ID]
  if !ok{return nil,ErrInvalidEnemyTurn}
  distance:=manhattan(enemy.Cell,position)
  if enemyCanStrike(board,enemy.Cell,position,profile){
   candidates=append(candidates,enemyTarget{
    ID:member.ID,Cell:position,Distance:distance,Row:row,
    Rank:(livingIndex-rotation+count)%count,
   })
  }
  livingIndex++
 }
 sort.Slice(candidates,func(i,j int)bool{
  a,b:=candidates[i],candidates[j]
  if a.Distance!=b.Distance{return a.Distance<b.Distance}
  if a.Row!=b.Row{return a.Row<b.Row}
  if a.Rank!=b.Rank{return a.Rank<b.Rank}
  return a.ID<b.ID
 })
 return candidates,nil
}

func enemyCardinalNext(board Snapshot,enemy Enemy,profile EnemyTurnProfile,occupied map[Cell]bool)*Cell{
 // The Chronicles BFS enumerates N/E/S/W, looks for the nearest *attack
 // position* (not merely the nearest party square), and returns the FIRST
 // step along that route. Moving never grants an attack in the same turn.
 type entry struct{cell Cell;first *Cell}
 queue:=[]entry{{cell:enemy.Cell}}
 seen:=map[Cell]bool{enemy.Cell:true}
 for head:=0;head<len(queue);head++{
  current:=queue[head]
  if current.first!=nil{
   for _,member:=range board.Party{
    if member.HP>0 && enemyCanStrike(
     board,current.cell,board.PartyPositions[member.ID],profile){
     result:=*current.first
     return &result
    }
   }
  }
  for _,direction:=range directions{
   next:=Cell{X:current.cell.X+direction.dx,Y:current.cell.Y+direction.dy}
   if seen[next]{continue}
   seen[next]=true
   if !traversable(tile(board,next))||occupied[next]||next==board.Anchor{continue}
   first:=current.first
   if first==nil{step:=next;first=&step}
   queue=append(queue,entry{cell:next,first:first})
  }
 }
 return nil
}

// DecideEnemyTurn authorizes ONLY the actor at the current initiative cursor.
// Profiles, enemy locations, turn order and HP are all server-owned. Exactly
// one of attack, one-cell move or wait is emitted; downstream real Effects
// must resolve damage, multi-attacks, paralysis, rewards and initiative.
func DecideEnemyTurn(
 board Snapshot,engagedEnemyIDs []string,round int,
 profiles map[string]EnemyTurnProfile,
)(EnemyTurnIntent,error){
 if board.Phase!="combat"||board.InitiativeCursor<0||
  board.InitiativeCursor>=len(board.Initiative){
  return EnemyTurnIntent{},ErrInvalidEnemyTurn
 }
 actor:=board.Initiative[board.InitiativeCursor]
 if actor.Kind!="enemy"||actor.ID==""{
  return EnemyTurnIntent{},ErrInvalidEnemyTurn
 }
 if step,err:=AdvanceEngagedTurn(board,round,engagedEnemyIDs);err!=nil||step.Ended{
  return EnemyTurnIntent{},ErrInvalidEnemyTurn
 }
 profile,ok:=profiles[actor.ID]
 if !ok || profile.AttackReach<1||profile.AttackReach>16||
  (profile.Movement!="hold"&&profile.Movement!="cardinal-chase"){
  return EnemyTurnIntent{},ErrInvalidEnemyTurn
 }
 occupants:=map[Cell]bool{}
 for _,member:=range board.Party{
  if member.HP==0{continue}
  pos,ok:=board.PartyPositions[member.ID]
  if !ok||!traversable(tile(board,pos))||occupants[pos]{
   return EnemyTurnIntent{},ErrInvalidEnemyTurn
  }
  if _,known:=tacticsFrontRow(member.ID);!known{return EnemyTurnIntent{},ErrInvalidEnemyTurn}
  occupants[pos]=true
 }
 var active Enemy
 activeFound:=false
 enemyIndex:=0
 seenEnemies:=map[Cell]bool{}
 for _,enemy:=range board.Enemies{
  if enemy.HP==0{continue}
  if !traversable(tile(board,enemy.Cell))||seenEnemies[enemy.Cell]||
   occupants[enemy.Cell]||enemy.Cell==board.Anchor{
   return EnemyTurnIntent{},ErrInvalidEnemyTurn
  }
  seenEnemies[enemy.Cell]=true
  if enemy.ID==actor.ID{active=enemy;activeFound=true
  }else{
   occupants[enemy.Cell]=true
  }
  if !activeFound{enemyIndex++}
 }
 if !activeFound{return EnemyTurnIntent{},ErrInvalidEnemyTurn}
 // Index of current active enemy among living map enemies (Chronicles
 // rotates target priority using active-enemy roster order).
 targets,err:=enemyAttackTargets(board,active,profile,round,enemyIndex)
 if err!=nil{return EnemyTurnIntent{},err}
 intent:=EnemyTurnIntent{EnemyID:actor.ID,Kind:"wait",From:active.Cell}
 if len(targets)>0{
  intent.Kind="attack"
  intent.TargetID=targets[0].ID
  return intent,nil
 }
 if profile.Movement=="hold"{return intent,nil}
 if next:=enemyCardinalNext(board,active,profile,occupants);next!=nil{
  intent.Kind="move"
  intent.To=next
 }
 return intent,nil
}
