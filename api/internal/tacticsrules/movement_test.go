package tacticsrules

import (
 "errors"
 "reflect"
 "testing"
)

var crypt = []string{
 "#######",
 "#..X..#",
 "#.###.#",
 "#...#.#",
 "#.#S#.#",
 "#P.E..#",
 "#######",
}

func originalBoard()Snapshot{
 return Snapshot{
  Grid:append([]string{},crypt...),
  Anchor:Cell{1,5},Phase:"explore",TurnPhase:"party",
  Party:[]PartyMember{{"matthias",7},{"rook",10},{"bishop",6},{"knight",8}},
  Enemies:[]Enemy{{ID:"corrupted-pawn",HP:4,Cell:Cell{3,5}}},
 }
}
func TestParityExplorationCardinalAndNoTurn(t *testing.T){
 s:=originalBoard()
 moves,err:=LegalMoves(s,"matthias")
 if err!=nil{t.Fatal(err)}
 want:=[]Move{
  {To:Cell{1,4},Direction:"north",Facing:0,Tile:'.'},
  {To:Cell{2,5},Direction:"east",Facing:1,Tile:'.'},
 }
 if !reflect.DeepEqual(moves,want){t.Fatalf("got %+v; want %+v",moves,want)}
 decided,err:=DecideMove(s,"matthias",Cell{1,4})
 if err!=nil{t.Fatal(err)}
 if decided.AdvancesTurn || decided.Combat || decided.From!=(Cell{1,5}) || decided.Move.Facing!=0{
  t.Fatalf("exploration must be free movement: %+v",decided)
 }
 for _,cell:=range []Cell{{1,6},{0,5},{2,4},{1,3},{3,5}}{
  if _,err:=DecideMove(s,"matthias",cell);!errors.Is(err,ErrIllegalMove){
   t.Errorf("invalid/diagonal/occupied move accepted %+v: %v",cell,err)
  }
 }
}
func TestParityExitBlockedButTrapTilePassableForDownstreamEffects(t *testing.T){
 s:=originalBoard()
 s.Anchor=Cell{2,1}
 if _,err:=DecideMove(s,"rook",Cell{3,1});!errors.Is(err,ErrIllegalMove){
  t.Fatalf("Chronicles X exit must block movement: %v",err)
 }
 s.Anchor=Cell{3,5}
 s.Enemies=nil
 decision,err:=DecideMove(s,"rook",Cell{3,4})
 if err!=nil || decision.Move.Tile!='S' || decision.AdvancesTurn{
  t.Fatalf("trap/content cell should be admitted but NOT resolved here: %+v %v",decision,err)
 }
}
func combatBoard()Snapshot{
 s:=originalBoard()
 s.Phase="combat"
 s.PartyPositions=map[string]Cell{
  "matthias":{1,5},"rook":{2,5},"bishop":{1,4},"knight":{4,5},
 }
 s.Initiative=[]Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"corrupted-pawn"},{Kind:"party",ID:"rook"}}
 return s
}
func TestParityInitiativeOccupancyAndCombatTurn(t *testing.T){
 s:=combatBoard()
 moves,err:=LegalMoves(s,"matthias")
 if err!=nil{t.Fatal(err)}
 if len(moves)!=0{t.Fatalf("two allies and walls block combat movement, got %+v",moves)}
 if _,err:=DecideMove(s,"rook",Cell{2,4});!errors.Is(err,ErrIllegalMove){
  t.Fatalf("wrong initiative actor may not move: %v",err)
 }
 s.PartyPositions["bishop"]=Cell{3,3}
 move,err:=DecideMove(s,"matthias",Cell{1,4})
 if err!=nil || !move.AdvancesTurn || !move.Combat || move.MemberID!="matthias"{
  t.Fatalf("combat action must advance initiative: %+v %v",move,err)
 }
 s.InitiativeCursor=1
 if _,err:=LegalMoves(s,"matthias");!errors.Is(err,ErrIllegalMove){
  t.Fatalf("enemy initiative turn must deny party: %v",err)
 }
 s.InitiativeCursor=2
 if _,err:=DecideMove(s,"rook",Cell{3,5});!errors.Is(err,ErrIllegalMove){
  t.Fatalf("active enemy blocks destination: %v",err)
 }
 s.Enemies[0].HP=0
 move,err=DecideMove(s,"rook",Cell{3,5})
 if err!=nil || move.Move.Direction!="east" {
  t.Fatalf("defeated enemy must release tile: %+v %v",move,err)
 }
}
func TestNoTerminalEnemyTurnDeadActorOrInvalidSnapshot(t *testing.T){
 for _,phase:=range []string{"defeated","escaped","unrecognized"}{
  s:=originalBoard();s.Phase=phase
  if _,err:=LegalMoves(s,"matthias");err==nil{t.Errorf("phase %s accepted",phase)}
 }
 s:=originalBoard();s.TurnPhase="enemy"
 if _,err:=DecideMove(s,"matthias",Cell{1,4});!errors.Is(err,ErrIllegalMove){
  t.Fatalf("enemy turn accepted: %v",err)
 }
 s=originalBoard();s.Party[0].HP=0
 if _,err:=LegalMoves(s,"matthias");!errors.Is(err,ErrIllegalMove){
  t.Fatalf("dead actor accepted: %v",err)
 }
 for _,mutate:=range []func(*Snapshot){
  func(s *Snapshot){s.Grid=[]string{"###","#"}},
  func(s *Snapshot){s.Anchor=Cell{0,0}},
  func(s *Snapshot){s.Party=append(s.Party,PartyMember{ID:"rook",HP:4})},
  func(s *Snapshot){s.Phase="combat"},
  func(s *Snapshot){s.Enemies=[]Enemy{{ID:"pawn",HP:1,Cell:Cell{0,0}}}},
  func(s *Snapshot){s.Phase="combat";s.Initiative=[]Actor{{Kind:"party",ID:"matthias"}}},
  func(s *Snapshot){s.Phase="explore";s.Initiative=[]Actor{{Kind:"party",ID:"matthias"}}},
 }{
  bad:=originalBoard();mutate(&bad)
  if _,err:=LegalMoves(bad,"matthias");err==nil{
   t.Errorf("invalid state accepted: %+v",bad)
  }
 }
}
