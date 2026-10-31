package tacticsrules

// Port of tacticsAttackDistance/tacticsLineIsClear from Chronicles' portable
// target geometry. These are pure server decisions; they NEVER apply damage,
// enemy turns, abilities, loot, XP or change a saved run.

import "errors"

var ErrIllegalAttack=errors.New("illegal tactics attack")

type AttackProfile struct {
 Pattern string
 Reach int
}
type AttackInput struct {
 Board Snapshot
 // Profiles and modifiers MUST be resolved from the server's authenticated
 // RPG character builds, never from HTTP request fields.
 Profiles map[string]AttackProfile
 ReachBonus map[string]int
}
type AttackDecision struct {
 MemberID string
 EnemyID string
 From Cell
 Target Cell
 Pattern string
 Distance int
 RequiresInitiative bool
 AdvancesTurn bool
}

// AttackDistance mirrors the JS geometries EXACTLY. Orthogonal and line
// permit zero-distance as a geometry result, but DecideAttack rejects it.
func AttackDistance(from,to Cell,pattern string)(int,bool){
 dx:=to.X-from.X;if dx<0{dx=-dx}
 dy:=to.Y-from.Y;if dy<0{dy=-dy}
 switch pattern{
 case "adjacent":
  if dx+dy==1{return 1,true}
 case "orthogonal":
  if dx==0 || dy==0{return dx+dy,true}
 case "diagonal":
  if dx==dy && dx>0{return dx,true}
 case "line":
  if dx==0 || dy==0 || dx==dy{
   if dx>dy{return dx,true}
   return dy,true
  }
 }
 return 0,false
}
func sign(value int)int{
 if value>0{return 1};if value<0{return -1};return 0
}

// LineIsClear checks intermediate wall cells and living enemies, not the
// destination itself, consistent with the current Chronicles tactical rules.
// Non-straight/non-diagonal rays fail without stepping through the board.
func LineIsClear(board Snapshot,from,to Cell,ignoredEnemyID string)bool{
 spanX:=to.X-from.X;if spanX<0{spanX=-spanX}
 spanY:=to.Y-from.Y;if spanY<0{spanY=-spanY}
 if spanX!=0 && spanY!=0 && spanX!=spanY{return false}
 dx:=sign(to.X-from.X);dy:=sign(to.Y-from.Y)
 x,y:=from.X+dx,from.Y+dy
 for x!=to.X || y!=to.Y{
  if tile(board,Cell{x,y})=='#'{return false}
  for _,enemy:=range board.Enemies{
   if enemy.HP>0 && enemy.ID!=ignoredEnemyID && same(enemy.Cell,Cell{x,y}){
    return false
   }
  }
  x+=dx;y+=dy
 }
 return true
}

// DecideAttack revalidates initiative, member, class reach/pattern, active
// target and line of sight against the SERVER's current board. An exploration
// attack must FIRST enter initiative; it grants NO free damage. The real
// Engine.Apply must implement that turn boundary and damage afterward.
func DecideAttack(input AttackInput,memberID,enemyID string)(AttackDecision,error){
 board:=input.Board
 from,combat,err:=origin(board,memberID)
 if err!=nil{return AttackDecision{},err}
 profile,ok:=input.Profiles[memberID]
 if !ok || profile.Reach<1 || profile.Reach>256{
  return AttackDecision{},ErrInvalidSnapshot
 }
 switch profile.Pattern{
 case "adjacent","orthogonal","diagonal","line":
 default:
  return AttackDecision{},ErrInvalidSnapshot
 }
 bonus:=input.ReachBonus[memberID]
 if bonus < -256 || bonus>256{return AttackDecision{},ErrInvalidSnapshot}
 reach:=profile.Reach
 if bonus>0{reach+=bonus}
 var target Cell
 found:=false
 if enemyID==""{return AttackDecision{},ErrIllegalAttack}
 for _,enemy:=range board.Enemies {
  if enemy.HP<0 || enemy.ID==""{return AttackDecision{},ErrInvalidSnapshot}
  if enemy.ID!=enemyID{continue}
  if found{return AttackDecision{},ErrInvalidSnapshot}
  if enemy.HP==0{return AttackDecision{},ErrIllegalAttack}
  target=enemy.Cell
  found=true
 }
 if !found{return AttackDecision{},ErrIllegalAttack}
 if !traversable(tile(board,target)){return AttackDecision{},ErrInvalidSnapshot}
 distance,patternValid:=AttackDistance(from,target,profile.Pattern)
 if !patternValid || distance<1 || distance>reach ||
  !LineIsClear(board,from,target,enemyID){
  return AttackDecision{},ErrIllegalAttack
 }
 return AttackDecision{
  MemberID:memberID,EnemyID:enemyID,
  From:from,Target:target,Pattern:profile.Pattern,Distance:distance,
  RequiresInitiative:!combat,AdvancesTurn:combat,
 },nil
}
