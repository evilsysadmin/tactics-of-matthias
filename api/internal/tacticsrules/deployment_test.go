package tacticsrules

import (
 "errors"
 "reflect"
 "testing"
)

func deployBoard()Snapshot{
 return Snapshot{
  Grid:[]string{
   "#########",
   "#...#...#",
   "#...#...#",
   "#...#...#",
   "#########",
  },
  Anchor:Cell{X:2,Y:2},Phase:"explore",TurnPhase:"party",
  Party:[]PartyMember{
   {ID:"matthias",HP:7},{ID:"rook",HP:10},
   {ID:"bishop",HP:6},{ID:"knight",HP:8},
  },
 }
}
func TestPartyFormationIsConnectedReachableAndDeterministic(t *testing.T){
 b:=deployBoard()
 first,err:=PlanPartyDeployment(b,1,nil)
 if err!=nil{t.Fatal(err)}
 second,err:=PlanPartyDeployment(b,1,nil)
 if err!=nil||!reflect.DeepEqual(first,second){
  t.Fatalf("unstable formation %+v / %+v %v",first,second,err)
 }
 if len(first)!=4{t.Fatalf("missing party members %+v",first)}
 used:=map[Cell]bool{}
 cells:=make([]deployCandidate,0,4)
 for _,id:=range partyGridOrder{
  cell,ok:=first[id]
  if !ok||used[cell]||cell.X<1||cell.X>3||cell.Y<1||cell.Y>3{
   t.Fatalf("formation teleported through masonry or overlapped: %+v",first)
  }
  used[cell]=true
  cells=append(cells,deployCandidate{Cell:cell})
 }
 if !connectedCluster(cells){t.Fatalf("party no longer connected: %+v",first)}
}
func TestTightCorridorNeverDropsLastLivingHero(t *testing.T){
 b:=Snapshot{
  Grid:[]string{"######","#...##","######"},
  Anchor:Cell{X:2,Y:1},Phase:"explore",TurnPhase:"party",
  Party:[]PartyMember{
   {ID:"matthias",HP:0},{ID:"rook",HP:0},
   {ID:"bishop",HP:3},{ID:"knight",HP:2},
  },
 }
 deployed,err:=PlanPartyDeployment(b,1,nil)
 if err!=nil{t.Fatal(err)}
 if len(deployed)!=2||deployed["bishop"]==deployed["knight"]||
  deployed["bishop"].Y!=1||deployed["knight"].Y!=1{
  t.Fatalf("living rear members lost in narrow corridor: %+v",deployed)
 }
 b.Grid=[]string{"#####","#.###","#####"}
 b.Anchor=Cell{X:1,Y:1}
 if _,err:=PlanPartyDeployment(b,1,nil);!errors.Is(err,ErrNoFormation){
  t.Fatalf("one cell cannot deploy two living heroes: %v",err)
 }
}
func TestEnemyAndAuthoredObjectsBlockSpawnAndNoWallTeleport(t *testing.T){
 b:=deployBoard()
 b.Enemies=[]Enemy{{ID:"pawn",HP:3,Cell:Cell{X:2,Y:1}}}
 authored:=[]Cell{{X:1,Y:2}}
 formation,err:=PlanPartyDeployment(b,0,authored)
 if err!=nil{t.Fatal(err)}
 for _,cell:=range formation{
  if cell==(Cell{X:2,Y:1})||cell==(Cell{X:1,Y:2})||cell.X>=5{
   t.Fatalf("spawned on enemy, authored object or other side of wall: %+v",formation)
  }
 }
 // Another wall-separated hall cannot supply extra tiles.
 b.Grid=[]string{
  "#########","#..##...#","#########",
 }
 b.Anchor=Cell{X:1,Y:1}
 b.Enemies=nil
 if _,err:=PlanPartyDeployment(b,0,nil);!errors.Is(err,ErrNoFormation){
  t.Fatalf("borrowed inaccessible positions from other corridor: %v",err)
 }
}
func TestDirectionAffectsPlacementAndDeadActorsHaveNoCell(t *testing.T){
 b:=deployBoard()
 b.Party[2].HP=0
 north,err:=PlanPartyDeployment(b,0,nil)
 if err!=nil{t.Fatal(err)}
 south,err:=PlanPartyDeployment(b,2,nil)
 if err!=nil{t.Fatal(err)}
 if len(north)!=3||len(south)!=3{
  t.Fatalf("wrong living roster %v / %v",north,south)
 }
 if _,ok:=north["bishop"];ok{t.Fatal("dead bishop spawned")}
 if _,ok:=south["bishop"];ok{t.Fatal("dead bishop spawned")}
 if reflect.DeepEqual(north,south){
  t.Fatalf("orientation had no effect on role positioning: %v / %v",north,south)
 }
}
func TestDeploymentRejectsCorruptOrUntrustedInputs(t *testing.T){
 for name,change:=range map[string]func(*Snapshot){
  "wrong phase":func(b *Snapshot){b.Phase="combat"},
  "enemy turn":func(b *Snapshot){b.TurnPhase="enemy"},
  "ghost initiative":func(b *Snapshot){b.Initiative=[]Actor{{Kind:"enemy",ID:"pawn"}}},
  "unknown member":func(b *Snapshot){b.Party[0].ID="visitor"},
  "duplicate member":func(b *Snapshot){b.Party[1].ID="matthias"},
  "negative HP":func(b *Snapshot){b.Party[0].HP=-2},
  "dead all":func(b *Snapshot){for i:=range b.Party{b.Party[i].HP=0}},
  "bad grid":func(b *Snapshot){b.Grid[2]="#...#..##"},
  "enemy on anchor":func(b *Snapshot){b.Enemies=[]Enemy{{ID:"pawn",HP:2,Cell:b.Anchor}}},
  "stale footprint":func(b *Snapshot){b.PartyPositions=map[string]Cell{"rook":{X:1,Y:1}}},
 }{
  t.Run(name,func(t *testing.T){
   b:=deployBoard()
   b.Party=append([]PartyMember(nil),b.Party...)
   b.Grid=append([]string(nil),b.Grid...)
   change(&b)
   _,err:=PlanPartyDeployment(b,0,nil)
   if err==nil{t.Fatal("invalid deployment accepted")}
  })
 }
 b:=deployBoard()
 if _,err:=PlanPartyDeployment(b,4,nil);!errors.Is(err,ErrInvalidDeployment){
  t.Fatal("invalid direction accepted")
 }
 if _,err:=PlanPartyDeployment(b,1,[]Cell{{X:0,Y:0}});!errors.Is(err,ErrInvalidDeployment){
  t.Fatal("authored object in wall accepted")
 }
}
