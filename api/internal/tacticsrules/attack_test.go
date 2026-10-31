package tacticsrules

import (
 "errors"
 "testing"
)

func TestTargetPatternParityWithChronicles(t *testing.T){
 from:=Cell{3,3}
 cases:=[]struct{to Cell;pattern string;distance int;ok bool}{
  {Cell{4,3},"adjacent",1,true},
  {Cell{4,4},"adjacent",0,false},
  {Cell{5,3},"adjacent",0,false},
  {Cell{3,5},"orthogonal",2,true},
  {Cell{4,4},"orthogonal",0,false},
  {Cell{3,3},"orthogonal",0,true},
  {Cell{5,5},"diagonal",2,true},
  {Cell{3,3},"diagonal",0,false},
  {Cell{6,5},"diagonal",0,false},
  {Cell{3,6},"line",3,true},
  {Cell{5,5},"line",2,true},
  {Cell{6,5},"line",0,false},
  {Cell{4,3},"unknown",0,false},
 }
 for _,tc:=range cases{
  d,ok:=AttackDistance(from,tc.to,tc.pattern)
  if d!=tc.distance || ok!=tc.ok{
   t.Errorf("%q %v: %v/%d expected %v/%d",tc.pattern,tc.to,ok,d,tc.ok,tc.distance)
  }
 }
}
func TestLineOfSightMatchesChroniclesIntermediateBlockers(t *testing.T){
 board:=Snapshot{Grid:[]string{
  "########","#......#","#......#","#......#",
  "#......#","#......#","#......#","########",
 }}
 from,to:=Cell{3,3},Cell{3,6}
 if !LineIsClear(board,from,to,""){t.Fatal("open corridor rejected")}
 board.Grid[5]="#..#...#"
 if LineIsClear(board,from,to,""){t.Fatal("intermediate wall did not block")}
 board.Grid[5]="#......#"
 board.Enemies=[]Enemy{{ID:"blocker",HP:2,Cell:Cell{3,4}}}
 if LineIsClear(board,from,to,""){t.Fatal("living intermediate enemy did not block")}
 if !LineIsClear(board,from,to,"blocker"){t.Fatal("ignored enemy not skipped")}
 board.Enemies[0].HP=0
 if !LineIsClear(board,from,to,""){t.Fatal("dead enemy blocked ray")}
 if LineIsClear(board,from,Cell{5,4},""){t.Fatal("non-cardinal and non-diagonal ray allowed")}
}
func TestRookReachesTwoCellsButMatthiasNeedsAdjacency(t *testing.T){
 s:=originalBoard()
 profiles:=map[string]AttackProfile{
  "rook":{Pattern:"orthogonal",Reach:2},
  "matthias":{Pattern:"adjacent",Reach:1},
 }
 input:=AttackInput{Board:s,Profiles:profiles}
 hit,err:=DecideAttack(input,"rook","corrupted-pawn")
 if err!=nil{t.Fatal(err)}
 if !hit.RequiresInitiative || hit.AdvancesTurn || hit.Distance!=2 ||
  hit.Target!=(Cell{3,5}) || hit.From!=(Cell{1,5}){
  t.Fatalf("exploration attack must start initiative BEFORE damage: %+v",hit)
 }
 if _,err:=DecideAttack(input,"matthias","corrupted-pawn");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("Matthias must be adjacent: %v",err)
 }
 s.Anchor=Cell{2,5}
 input.Board=s
 hit,err=DecideAttack(input,"matthias","corrupted-pawn")
 if err!=nil || hit.Distance!=1{t.Fatalf("adjacent attack failed: %+v %v",hit,err)}
}
func TestDiagonalWallsAndClassReach(t *testing.T){
 s:=originalBoard()
 s.Anchor=Cell{1,3}
 input:=AttackInput{Board:s,Profiles:map[string]AttackProfile{
  "bishop":{Pattern:"diagonal",Reach:4},
 }}
 if _,err:=DecideAttack(input,"bishop","corrupted-pawn");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("crypt wall at (2,4) must block bishop: %v",err)
 }
 s.Anchor=Cell{4,5}
 s.Enemies=[]Enemy{{ID:"near-trap",HP:3,Cell:Cell{3,4}}}
 input.Board=s
 if hit,err:=DecideAttack(input,"bishop","near-trap");err!=nil || hit.Distance!=1{
  t.Fatalf("bishop diagonal into S tile failed: %+v %v",hit,err)
 }
 s.Anchor=Cell{1,5}
 s.Enemies=[]Enemy{{ID:"far-pawn",HP:3,Cell:Cell{4,5}}}
 input.Board=s
 input.Profiles=map[string]AttackProfile{"rook":{Pattern:"orthogonal",Reach:2}}
 if _,err:=DecideAttack(input,"rook","far-pawn");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("out-of-reach target accepted: %v",err)
 }
 input.ReachBonus=map[string]int{"rook":1}
 if hit,err:=DecideAttack(input,"rook","far-pawn");err!=nil || hit.Distance!=3{
  t.Fatalf("validated reach bonus not applied: %+v %v",hit,err)
 }
 s.Enemies=append(s.Enemies,Enemy{ID:"blocker",HP:2,Cell:Cell{2,5}})
 input.Board=s
 if _,err:=DecideAttack(input,"rook","far-pawn");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("intermediate living enemy failed to block ray: %v",err)
 }
}
func TestAttackRechecksTurnAndNeverGrantsFreeCombatHit(t *testing.T){
 s:=combatBoard()
 s.Initiative=[]Actor{{Kind:"party",ID:"knight"},{Kind:"enemy",ID:"corrupted-pawn"}}
 input:=AttackInput{Board:s,Profiles:map[string]AttackProfile{"knight":{Pattern:"line",Reach:3}}}
 hit,err:=DecideAttack(input,"knight","corrupted-pawn")
 if err!=nil || !hit.AdvancesTurn || hit.RequiresInitiative{
  t.Fatalf("combat hit must spend real actor's turn: %+v %v",hit,err)
 }
 if _,err:=DecideAttack(input,"rook","corrupted-pawn");!errors.Is(err,ErrIllegalMove){
  t.Fatalf("stale non-active portrait allowed attack: %v",err)
 }
 s.InitiativeCursor=1
 input.Board=s
 if _,err:=DecideAttack(input,"knight","corrupted-pawn");!errors.Is(err,ErrIllegalMove){
  t.Fatalf("enemy initiative actor did not block: %v",err)
 }
}
func TestAttackFailsClosedForMissingEnemyDeadEnemyAndInvalidProfiles(t *testing.T){
 s:=originalBoard()
 input:=AttackInput{Board:s,Profiles:map[string]AttackProfile{"rook":{Pattern:"orthogonal",Reach:2}}}
 if _,err:=DecideAttack(input,"rook","ghost");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("missing target allowed: %v",err)
 }
 s.Enemies[0].HP=0
 input.Board=s
 if _,err:=DecideAttack(input,"rook","corrupted-pawn");!errors.Is(err,ErrIllegalAttack){
  t.Fatalf("dead enemy still targetable: %v",err)
 }
 s.Enemies[0].HP=3
 input.Board=s
 for _,profile:=range []AttackProfile{{Pattern:"unknown",Reach:1},{Pattern:"line",Reach:0}}{
  input.Profiles=map[string]AttackProfile{"rook":profile}
  if _,err:=DecideAttack(input,"rook","corrupted-pawn");!errors.Is(err,ErrInvalidSnapshot){
   t.Fatalf("untrusted profile accepted %+v: %v",profile,err)
  }
 }
}
