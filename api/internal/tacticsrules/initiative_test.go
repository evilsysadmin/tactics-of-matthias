package tacticsrules

import (
 "bytes"
 "errors"
 "io"
 "reflect"
 "testing"
)

func TestInitiativeOriginalScoresAndTieBreaks(t *testing.T){
 board:=originalBoard()
 board.Party=[]PartyMember{{ID:"zeta",HP:1},{ID:"alpha",HP:2}}
 board.Enemies=[]Enemy{{ID:"e2",HP:5,Cell:Cell{3,5}},{ID:"e1",HP:3,Cell:Cell{4,5}}}
 stats:=Agility{
  Party:map[string]int64{"zeta":4,"alpha":4},
  Enemies:map[string]int64{"e2":4,"e1":4},
 }
 round,err:=RollInitiative(board,stats,bytes.NewReader(bytes.Repeat([]byte{0},4)))
 if err!=nil{t.Fatal(err)}
 if round.Round!=1 || round.Cursor!=0 || round.Version!=1 || round.Die!="1d8"{
  t.Fatalf("wrong initiative contract: %+v",round)
 }
 got:=[]string{}
 for _,actor:=range round.Order{
  got=append(got,actor.Kind+":"+actor.ID)
  if actor.Initiative!=5 || actor.Roll!=1{t.Fatalf("wrong score: %+v",actor)}
 }
 want:=[]string{"enemy:e1","enemy:e2","party:alpha","party:zeta"}
 if !reflect.DeepEqual(got,want){t.Fatalf("initiative tie sort got %v expected %v",got,want)}
}
func TestInitiativeAgilityAndDieAreIndependent(t *testing.T){
 board:=originalBoard()
 board.Party=[]PartyMember{{ID:"rook",HP:8},{ID:"knight",HP:4}}
 board.Enemies=[]Enemy{{ID:"goblin",HP:3,Cell:Cell{3,5}}}
 stats:=Agility{
  Party:map[string]int64{"rook":2,"knight":5},
  Enemies:map[string]int64{"goblin":4},
 }
 // 7, 0, 3 => d8 rolls 8, 1, 4; scores 10,6,8.
 r,err:=RollInitiative(board,stats,bytes.NewReader([]byte{7,0,3}))
 if err!=nil{t.Fatal(err)}
 if len(r.Order)!=3 || r.Order[0].ID!="rook" || r.Order[1].ID!="goblin" ||
  r.Order[2].ID!="knight" {t.Fatalf("wrong initiative order: %+v",r.Order)}
 if _,err:=RollInitiative(board,stats,bytes.NewReader([]byte{}));!errors.Is(err,io.EOF){
  t.Fatalf("entropy exhaustion must fail closed, got %v",err)
 }
}
func TestAdvanceSkipsDefeatedCombatantsAndRoundWrap(t *testing.T){
 board:=combatBoard()
 board.Party=[]PartyMember{{"matthias",6},{"rook",0},{"bishop",4},{"knight",8}}
 board.Enemies=[]Enemy{{ID:"corrupted-pawn",HP:5,Cell:Cell{3,5}}}
 board.Initiative=[]Actor{
  {Kind:"party",ID:"matthias"}, {Kind:"party",ID:"rook"},
  {Kind:"enemy",ID:"corrupted-pawn"}, {Kind:"party",ID:"bishop"},
 }
 board.InitiativeCursor=0
 step,err:=AdvanceTurn(board,2)
 if err!=nil{t.Fatal(err)}
 if step.Next==nil || step.Next.ID!="corrupted-pawn" || step.Cursor!=1 ||
  step.Round!=2 || len(step.Order)!=3 {t.Fatalf("dead actor was not skipped: %+v",step)}
 board.InitiativeCursor=3
 step,err=AdvanceTurn(board,2)
 if err!=nil{t.Fatal(err)}
 if step.Next==nil || step.Next.ID!="matthias" || step.Round!=3 || step.Cursor!=0{
  t.Fatalf("round wrap was lost: %+v",step)
 }
}
func TestEndingCombatDoesNotTeleportOrMutateAnchor(t *testing.T){
 board:=combatBoard()
 board.Enemies[0].HP=0
 before:=board.Anchor
 transition,err:=AdvanceTurn(board,4)
 if err!=nil || transition.Phase!="explore" || !transition.Ended ||
  board.Anchor!=before{t.Fatalf("winning fight not ended cleanly: %+v %v",transition,err)}
 board.Enemies[0].HP=4
 for i:=range board.Party{board.Party[i].HP=0}
 transition,err=AdvanceTurn(board,4)
 if err!=nil || transition.Phase!="defeated" || !transition.Ended{
  t.Fatalf("wiped party not defeated: %+v %v",transition,err)
 }
}
func TestInitiativeRejectsPhantomTurnsAndMalformedRosters(t *testing.T){
 board:=combatBoard()
 invalid:=[]func(*Snapshot){
  func(b *Snapshot){b.Initiative=append(b.Initiative,b.Initiative[0])},
  func(b *Snapshot){b.Initiative=append(b.Initiative,Actor{Kind:"enemy",ID:"ghost"})},
  func(b *Snapshot){b.InitiativeCursor=77},
  func(b *Snapshot){b.Party=append(b.Party,b.Party[0])},
  func(b *Snapshot){b.Enemies=append(b.Enemies,b.Enemies[0])},
  func(b *Snapshot){b.Enemies=append(b.Enemies,Enemy{ID:"enemy-off-order",HP:1,Cell:Cell{3,3}})},
  func(b *Snapshot){b.Phase="explore"},
 }
 for _,change:=range invalid{
  b:=board
  b.Party=append([]PartyMember(nil),board.Party...)
  b.Enemies=append([]Enemy(nil),board.Enemies...)
  b.Initiative=append([]Actor(nil),board.Initiative...)
  change(&b)
  if _,err:=AdvanceTurn(b,3);!errors.Is(err,ErrInvalidInitiative){
   t.Errorf("malformed initiative accepted: %+v, error=%v",b,err)
  }
 }
}
func TestRollRejectsMissingStatsAndDeadOnlyCombat(t *testing.T){
 board:=originalBoard()
 if _,err:=RollInitiative(board,Agility{},nil);!errors.Is(err,ErrInvalidInitiative){
  t.Fatalf("missing authored AGI accepted: %v",err)
 }
 board.Enemies[0].HP=0
 if _,err:=RollInitiative(board,Agility{Party:map[string]int64{
  "matthias":4,"rook":2,"bishop":3,"knight":5,
 }},nil);!errors.Is(err,ErrInvalidInitiative){
  t.Fatalf("enemy-free initiative started: %v",err)
 }
}
