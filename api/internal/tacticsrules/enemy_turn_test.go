package tacticsrules

import (
 "errors"
 "reflect"
 "testing"
)

func enemyBoard()Snapshot{
 return Snapshot{
  Grid:[]string{
   "#########",
   "#.......#",
   "#.......#",
   "#.......#",
   "#.......#",
   "#.......#",
   "#########",
  },
  Anchor:Cell{X:1,Y:1},Phase:"combat",TurnPhase:"enemy",
  Party:[]PartyMember{{ID:"matthias",HP:7},{ID:"rook",HP:8},
   {ID:"bishop",HP:6},{ID:"knight",HP:5}},
  PartyPositions:map[string]Cell{
   "matthias":{X:4,Y:3},"rook":{X:5,Y:2},
   "bishop":{X:2,Y:4},"knight":{X:6,Y:4},
  },
  Enemies:[]Enemy{{ID:"goblin",HP:5,Cell:Cell{X:4,Y:2}}},
  Initiative:[]Actor{
   {Kind:"enemy",ID:"goblin"},{Kind:"party",ID:"matthias"},
   {Kind:"party",ID:"rook"},{Kind:"party",ID:"bishop"},
   {Kind:"party",ID:"knight"},
  },
 }
}
func enemyProfiles()map[string]EnemyTurnProfile{
 return map[string]EnemyTurnProfile{
  "goblin":{Movement:"cardinal-chase",AttackReach:1,RequiresLineOfSight:true},
 }
}
func TestEnemyAttacksLivingFrontlineAndRotatesEqualTargets(t *testing.T){
 b:=enemyBoard()
 first,err:=DecideEnemyTurn(b,[]string{"goblin"},1,enemyProfiles())
 if err!=nil{t.Fatal(err)}
 if first.Kind!="attack"||first.TargetID!="rook"||first.To!=nil{
  t.Fatalf("round-1 rotated front-line tie wrongly: %+v",first)
 }
 later,err:=DecideEnemyTurn(b,[]string{"goblin"},2,enemyProfiles())
 if err!=nil||later.Kind!="attack"||later.TargetID!="matthias"{
  t.Fatalf("round-2 target selection wrong: %+v, %v",later,err)
 }
 if b.Party[0].HP!=7||b.Enemies[0].Cell!=(Cell{X:4,Y:2}){
  t.Fatal("AI intent mutated authoritative board")
 }
 b.Party[0].HP=0
 b.PartyPositions=map[string]Cell{"rook":{X:5,Y:2},
  "bishop":{X:2,Y:4},"knight":{X:6,Y:4}}
 next,err:=DecideEnemyTurn(b,[]string{"goblin"},2,enemyProfiles())
 if err!=nil||next.TargetID!="rook"{t.Fatalf("attacked a downed hero: %+v %v",next,err)}
}
func TestCardinalEnemyMovesOneStepTowardsAttackPosition(t *testing.T){
 b:=enemyBoard()
 b.Party=[]PartyMember{{ID:"matthias",HP:7}}
 b.PartyPositions=map[string]Cell{"matthias":{X:6,Y:1}}
 b.Enemies[0].Cell=Cell{X:2,Y:1}
 b.Initiative=[]Actor{{Kind:"enemy",ID:"goblin"},{Kind:"party",ID:"matthias"}}
 b.Anchor=Cell{X:1,Y:3}
 b.Grid[1]="#..#....#" // walls forbid approaching directly from the left
 intent,err:=DecideEnemyTurn(b,[]string{"goblin"},1,enemyProfiles())
 if err!=nil || intent.Kind!="move"||intent.To==nil||
  *intent.To!=(Cell{X:2,Y:2})||intent.TargetID!=""{
  t.Fatalf("wrong BFS first step across wall: %+v %v",intent,err)
 }
 // Even if that step eventually places an enemy next to a player, it
 // consumes the whole actor turn; no combined move+attack is emitted.
 if b.Enemies[0].Cell!=(Cell{X:2,Y:1}){
  t.Fatal("AI intent applied movement prematurely")
 }
}
func TestBlockedEnemyWaitsWithoutPhantomAttackOrTeleport(t *testing.T){
 b:=enemyBoard()
 b.Party=[]PartyMember{{ID:"matthias",HP:7}}
 b.PartyPositions=map[string]Cell{"matthias":{X:6,Y:2}}
 b.Initiative=[]Actor{{Kind:"enemy",ID:"goblin"},{Kind:"party",ID:"matthias"}}
 b.Grid=[]string{
  "#########","#.......#","#..#.#..#","#..###..#",
  "#..#.#..#","#.......#","#########",
 }
 b.Enemies[0].Cell=Cell{X:4,Y:3}
 b.Anchor=Cell{X:1,Y:1}
 intent,err:=DecideEnemyTurn(b,[]string{"goblin"},1,enemyProfiles())
 if err!=nil||intent.Kind!="wait"||intent.To!=nil||intent.TargetID!=""{
  t.Fatalf("boxed-in actor should wait: %+v %v",intent,err)
 }
}
func TestEnemyHoldAndRangedLineOfSight(t *testing.T){
 b:=enemyBoard()
 b.Party=[]PartyMember{{ID:"matthias",HP:7}}
 b.PartyPositions=map[string]Cell{"matthias":{X:6,Y:2}}
 b.Initiative=[]Actor{{Kind:"enemy",ID:"goblin"},{Kind:"party",ID:"matthias"}}
 b.Enemies[0].Cell=Cell{X:3,Y:2}
 b.Anchor=Cell{X:1,Y:1}
 b.Grid[2]="#...#...#"
 hold:=map[string]EnemyTurnProfile{"goblin":{
  Movement:"hold",AttackReach:4,RequiresLineOfSight:true,
 }}
 noShot,err:=DecideEnemyTurn(b,[]string{"goblin"},1,hold)
 if err!=nil||noShot.Kind!="wait"{t.Fatalf("shot through wall: %+v %v",noShot,err)}
 hold["goblin"]=EnemyTurnProfile{
  Movement:"hold",AttackReach:4,RequiresLineOfSight:false,
 }
 shot,err:=DecideEnemyTurn(b,[]string{"goblin"},1,hold)
 if err!=nil||shot.Kind!="attack"||shot.TargetID!="matthias"{
  t.Fatalf("authored no-LOS ranged shot rejected: %+v %v",shot,err)
 }
 b.PartyPositions["matthias"]=Cell{X:5,Y:3}
 offAxis,err:=DecideEnemyTurn(b,[]string{"goblin"},1,hold)
 if err!=nil||offAxis.Kind!="wait"{
  t.Fatalf("ranged enemy attacked diagonally: %+v %v",offAxis,err)
 }
}
func TestEnemyTurnRejectsFakeTurnsAndUnsupportedAIPolicies(t *testing.T){
 for _,tc:=range []struct{
  name string
  mutate func(*Snapshot)
  engaged []string
  profiles map[string]EnemyTurnProfile
  round int
 }{
  {"not enemy turn",func(b *Snapshot){b.InitiativeCursor=1},
   []string{"goblin"},enemyProfiles(),1},
  {"enemy outside combat",func(b *Snapshot){b.Phase="explore"},
   []string{"goblin"},enemyProfiles(),1},
  {"no engaged roster",func(b *Snapshot){},nil,enemyProfiles(),1},
  {"wrong engaged roster",func(b *Snapshot){},[]string{"ghost"},enemyProfiles(),1},
  {"missing profile",func(b *Snapshot){},[]string{"goblin"},nil,1},
  {"unsupported special movement",func(b *Snapshot){},[]string{"goblin"},
   map[string]EnemyTurnProfile{"goblin":{Movement:"knight-chase",AttackReach:1}},1},
  {"invalid reach",func(b *Snapshot){},[]string{"goblin"},
   map[string]EnemyTurnProfile{"goblin":{Movement:"hold",AttackReach:17}},1},
  {"dead enemy",func(b *Snapshot){b.Enemies[0].HP=0},
   []string{"goblin"},enemyProfiles(),1},
  {"out-of-map hero",func(b *Snapshot){b.PartyPositions["matthias"]=Cell{X:999,Y:4}},
   []string{"goblin"},enemyProfiles(),1},
  {"duplicate enemy",func(b *Snapshot){b.Enemies=append(b.Enemies,b.Enemies[0])},
   []string{"goblin"},enemyProfiles(),1},
  {"no valid round",func(b *Snapshot){},[]string{"goblin"},enemyProfiles(),0},
 }{
  t.Run(tc.name,func(t *testing.T){
   board:=enemyBoard()
   board.Party=append([]PartyMember(nil),board.Party...)
   board.Enemies=append([]Enemy(nil),board.Enemies...)
   board.PartyPositions=map[string]Cell{}
   for k,v:=range enemyBoard().PartyPositions{board.PartyPositions[k]=v}
   tc.mutate(&board)
   if _,err:=DecideEnemyTurn(board,tc.engaged,tc.round,tc.profiles);
    !errors.Is(err,ErrInvalidEnemyTurn){
    t.Fatalf("invalid enemy turn accepted: %v",err)
   }
  })
 }
}
func TestNoDoubleTurnAndEnemyOccupancy(t *testing.T){
 b:=enemyBoard()
 b.Party=[]PartyMember{{ID:"matthias",HP:8}}
 b.PartyPositions=map[string]Cell{"matthias":{X:6,Y:2}}
 b.Enemies=append(b.Enemies,Enemy{ID:"blocker",HP:5,Cell:Cell{X:5,Y:2}})
 b.Initiative=[]Actor{{Kind:"enemy",ID:"goblin"},
  {Kind:"party",ID:"matthias"}}
 b.Enemies[0].Cell=Cell{X:4,Y:2}
 // The distant blocker cannot receive an initiative slot, but it is
 // still physical occupancy for the active enemy's BFS.
 got,err:=DecideEnemyTurn(b,[]string{"goblin"},2,enemyProfiles())
 if err!=nil||got.Kind!="move"||got.To==nil||
  *got.To==(Cell{X:5,Y:2}){
  t.Fatalf("enemy stepped into an unengaged unit: %+v %v",got,err)
 }
 b.InitiativeCursor=1
 if _,err:=DecideEnemyTurn(b,[]string{"goblin"},2,enemyProfiles());
  !errors.Is(err,ErrInvalidEnemyTurn){
  t.Fatalf("enemy acted during party turn: %v",err)
 }
 b.InitiativeCursor=0
 first,err:=DecideEnemyTurn(b,[]string{"goblin"},2,enemyProfiles())
 if err!=nil||!reflect.DeepEqual(got,first){
  t.Fatalf("AI decision was nondeterministic: %+v vs %+v %v",got,first,err)
 }
}
