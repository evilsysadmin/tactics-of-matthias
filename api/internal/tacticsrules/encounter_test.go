package tacticsrules

import (
 "errors"
 "reflect"
 "testing"
)

func encounterBoard()Snapshot{
 return Snapshot{
  Grid:[]string{
   "#########",
   "#.......#",
   "#.......#",
   "#.......#",
   "#.......#",
   "#########",
  },
  Anchor:Cell{X:1,Y:1},Phase:"explore",TurnPhase:"party",
  Party:[]PartyMember{{ID:"matthias",HP:8}},
  Enemies:[]Enemy{{ID:"spider",HP:4,Cell:Cell{X:3,Y:2}}},
 }
}
func iPtr(value int)*int{return &value}
func TestBresenhamEngagementNotRangedAttackGeometry(t *testing.T){
 board:=encounterBoard()
 if !EnemySightline(board.Grid,board.Enemies[0].Cell,board.Anchor){
  t.Fatal("non-45-degree Bresenham encounter sightline should pass")
 }
 if LineIsClear(board,board.Enemies[0].Cell,board.Anchor,"spider"){
  t.Fatal("ranged attack geometry must remain restricted")
 }
 ids,err:=EngagedEnemies(board,map[string]EnemyEngagement{
  "spider":{AttackReach:1,EngageRange:iPtr(3)},
 })
 if err!=nil||!reflect.DeepEqual(ids,[]string{"spider"}){
  t.Fatalf("visible engaged enemy missing: %v %v",ids,err)
 }
 ids,err=EngagedEnemies(board,map[string]EnemyEngagement{
  "spider":{AttackReach:1},
 })
 if err!=nil||len(ids)!=0{
  t.Fatalf("no explicit engagement range must default to reach: %v %v",ids,err)
 }
}

func TestWallsAndSolidCornersBlockSightline(t *testing.T){
 base:=encounterBoard()
 base.Grid[1]="#.#.....#" // party at 1,1; wall 2,1
 base.Enemies[0].Cell=Cell{X:3,Y:1}
 if EnemySightline(base.Grid,base.Enemies[0].Cell,base.Anchor){
  t.Fatal("straight wall did not block")
 }
 grid:=[]string{"#####","#.#.#","##..#","#...#","#####"}
 if EnemySightline(grid,Cell{X:1,Y:1},Cell{X:2,Y:2}){
  t.Fatal("solid diagonal corner did not block")
 }
 grid[1]="#...#"
 if !EnemySightline(grid,Cell{X:1,Y:1},Cell{X:2,Y:2}){
  t.Fatal("single open corner should allow vision")
 }
 if EnemySightline(grid,Cell{X:-1,Y:1},Cell{X:2,Y:2}){
  t.Fatal("out-of-map endpoint accepted")
 }
}

func TestEngagementFiltersDeadAndKeepsRosterOrder(t *testing.T){
 board:=encounterBoard()
 board.Enemies=[]Enemy{
  {ID:"dead",HP:0,Cell:Cell{X:2,Y:1}},
  {ID:"spider",HP:4,Cell:Cell{X:3,Y:2}},
  {ID:"goblin",HP:3,Cell:Cell{X:2,Y:1}},
  {ID:"far",HP:7,Cell:Cell{X:7,Y:4}},
 }
 ids,err:=EngagedEnemies(board,map[string]EnemyEngagement{
  "spider":{AttackReach:3},
  "goblin":{AttackReach:1},
  "far":{AttackReach:1},
 })
 if err!=nil||!reflect.DeepEqual(ids,[]string{"spider","goblin"}){
  t.Fatalf("wrong live engaged subset: %v %v",ids,err)
 }
}

func TestInvalidProfilesCannotFabricateDetection(t *testing.T){
 for _,profile:=range []EnemyEngagement{
  {},{AttackReach:17},{AttackReach:1,EngageRange:iPtr(0)},
  {AttackReach:1,EngageRange:iPtr(65)},
 }{
  board:=encounterBoard()
  if _,err:=EngagedEnemies(board,map[string]EnemyEngagement{"spider":profile});
   !errors.Is(err,ErrInvalidEncounter){t.Errorf("invalid profile %+v accepted: %v",profile,err)}
 }
 board:=encounterBoard()
 if _,err:=EngagedEnemies(board,nil);!errors.Is(err,ErrInvalidEncounter){
  t.Fatalf("missing authored profiles accepted: %v",err)
 }
 board.Enemies[0].HP=0
 if ids,err:=EngagedEnemies(board,nil);err!=nil||len(ids)!=0{
  t.Fatalf("dead enemy should not trigger initiative: %v %v",ids,err)
 }
}

func TestEncounterCannotStartOnEnemyTurnOrCorruptRoster(t *testing.T){
 changes:=[]func(*Snapshot){
  func(b *Snapshot){b.Phase="combat"},
  func(b *Snapshot){b.TurnPhase="enemy"},
  func(b *Snapshot){b.Party[0].HP=0},
  func(b *Snapshot){b.Initiative=[]Actor{{Kind:"enemy",ID:"spider"}}},
  func(b *Snapshot){b.Enemies[0].Cell=b.Anchor},
  func(b *Snapshot){b.Party=append(b.Party,PartyMember{ID:"matthias",HP:1})},
  func(b *Snapshot){b.Enemies=append(b.Enemies,Enemy{ID:"spider",HP:3,Cell:Cell{2,3}})},
  func(b *Snapshot){b.Grid=[]string{"####","#.#","###"}},
 }
 for _,change:=range changes{
  board:=encounterBoard()
  board.Party=append([]PartyMember(nil),board.Party...)
  board.Enemies=append([]Enemy(nil),board.Enemies...)
  change(&board)
  if _,err:=EngagedEnemies(board,map[string]EnemyEngagement{
   "spider":{AttackReach:3},
  });!errors.Is(err,ErrInvalidEncounter){
   t.Errorf("invalid encounter accepted: %+v error %v",board,err)
  }
 }
}
