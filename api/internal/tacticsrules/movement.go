// Package tacticsrules ports legal cardinal tactical movement from
// ChroniclesOfMatthiasTactics.js (functions chroniclesTacticsLegalMoves and
// chroniclesTacticsMove). This is a pure server-side decision, NOT a complete
// Engine.Apply implementation. Only the authoritative game engine may supply
// Snapshot and resolve trap/AI/initiative effects after a legal decision.
package tacticsrules

import "errors"

var ErrIllegalMove = errors.New("illegal tactics move")
var ErrInvalidSnapshot = errors.New("invalid authoritative tactics snapshot")

type Cell struct {
 X int `json:"x"`
 Y int `json:"y"`
}
type PartyMember struct {
 ID string `json:"id"`
 HP int `json:"hp"`
}
type Enemy struct {
 ID string `json:"id"`
 HP int `json:"hp"`
 Cell Cell `json:"cell"`
}
type Actor struct {
 Kind string `json:"kind"`
 ID string `json:"id"`
}
type Snapshot struct {
 Grid []string
 Anchor Cell
 Phase string
 TurnPhase string
 Party []PartyMember
 PartyPositions map[string]Cell
 Enemies []Enemy
 Initiative []Actor
 InitiativeCursor int
}
type Move struct {
 To Cell
 Direction string
 Facing int
 Tile byte
}
type Decision struct {
 From Cell
 Move Move
 MemberID string
 Combat bool
 AdvancesTurn bool
}

var directions = []struct{
 key string
 dx,dy int
}{
 {"north",0,-1},
 {"east",1,0},
 {"south",0,1},
 {"west",-1,0},
}

func tile(s Snapshot,p Cell) byte {
 if p.Y<0 || p.Y>=len(s.Grid) || p.X<0 || p.X>=len(s.Grid[0]) {return '#'}
 return s.Grid[p.Y][p.X]
}
func traversable(tile byte)bool {return tile!='#' && tile!='X' && tile!=0}
func validGrid(grid []string) bool {
 if len(grid)==0 || len(grid)>256 || len(grid[0])==0 || len(grid[0])>256 {
  return false
 }
 width:=len(grid[0])
 for _,row:=range grid {
  if len(row)!=width{return false}
  for i:=0;i<len(row);i++ {
   if row[i]<33 || row[i]>126{return false}
  }
 }
 return true
}
func same(a,b Cell)bool{return a.X==b.X && a.Y==b.Y}

func origin(s Snapshot,memberID string)(Cell,bool,error){
 if !validGrid(s.Grid){return Cell{},false,ErrInvalidSnapshot}
 if s.Phase!="explore" && s.Phase!="combat"{return Cell{},false,ErrIllegalMove}
 if !traversable(tile(s,s.Anchor)){return Cell{},false,ErrInvalidSnapshot}
 alive:=map[string]bool{}
 for _,member:=range s.Party {
  if member.ID=="" || member.HP<0 || alive[member.ID]{
   return Cell{},false,ErrInvalidSnapshot
  }
  alive[member.ID]=member.HP>0
 }
 if !alive[memberID]{return Cell{},false,ErrIllegalMove}
 combat:=s.Phase=="combat"
 if combat {
  if len(s.Initiative)==0 || s.InitiativeCursor<0 || s.InitiativeCursor>=len(s.Initiative) {
   return Cell{},true,ErrInvalidSnapshot
  }
  actor:=s.Initiative[s.InitiativeCursor]
  if actor.Kind!="party" || actor.ID!=memberID {
   return Cell{},true,ErrIllegalMove
  }
  seen:=map[Cell]bool{}
  for _,member:=range s.Party{
   if member.HP==0{continue}
   cell,ok:=s.PartyPositions[member.ID]
   if !ok || !traversable(tile(s,cell)) || seen[cell]{
    return Cell{},true,ErrInvalidSnapshot
   }
   seen[cell]=true
  }
  return s.PartyPositions[memberID],true,nil
 }
 if len(s.Initiative)!=0{return Cell{},false,ErrInvalidSnapshot}
 if s.TurnPhase=="enemy"{return Cell{},false,ErrIllegalMove}
 return s.Anchor,false,nil
}

// LegalMoves rechecks the current stored board, current initiative actor,
// living party footprint and alive enemies. Map exits X and walls # are
// blocked in both modes, as in Chronicles. Traps/interactive floor tiles are
// not resolved here: their authoritative effects must run AFTER a move.
func LegalMoves(s Snapshot,memberID string)([]Move,error){
 from,combat,err:=origin(s,memberID)
 if err!=nil{return nil,err}
 occupied:=map[Cell]bool{}
 for _,enemy:=range s.Enemies {
  if enemy.HP<0 || enemy.ID==""{return nil,ErrInvalidSnapshot}
  if enemy.HP>0 {
   if !traversable(tile(s,enemy.Cell)){return nil,ErrInvalidSnapshot}
   occupied[enemy.Cell]=true
  }
 }
 if combat {
  for _,member:=range s.Party{
   if member.ID!=memberID && member.HP>0 {
    occupied[s.PartyPositions[member.ID]]=true
   }
  }
 }
 moves:=make([]Move,0,4)
 for facing,direction:=range directions {
  to:=Cell{X:from.X+direction.dx,Y:from.Y+direction.dy}
  current:=tile(s,to)
  if !traversable(current) || occupied[to]{continue}
  moves=append(moves,Move{To:to,Direction:direction.key,Facing:facing,Tile:current})
 }
 return moves,nil
}

// DecideMove does NOT mutate a run or advance initiative. It returns the
// legal move and whether a downstream real game reducer owes a combat turn;
// traps, encounters, enemy movement, player death and save CAS belong there.
func DecideMove(s Snapshot,memberID string,to Cell)(Decision,error){
 from,combat,err:=origin(s,memberID)
 if err!=nil{return Decision{},err}
 moves,err:=LegalMoves(s,memberID)
 if err!=nil{return Decision{},err}
 for _,move:=range moves{
  if same(move.To,to){
   return Decision{From:from,Move:move,MemberID:memberID,
    Combat:combat,AdvancesTurn:combat},nil
  }
 }
 return Decision{},ErrIllegalMove
}
