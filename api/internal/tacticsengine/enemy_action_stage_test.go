package tacticsengine

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "reflect"
 "testing"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var enemyArenaGrid=[]string{
 "########","#......#","#......#",
 "#......#","#......#","########",
}
func enemyStageRun()tacticsapi.Run{
 state:=StateV1{
  Version:1,MapID:"arena",X:1,Y:1,
  Phase:"combat",TurnPhase:"enemy",Direction:1,
  Party:[]tacticsrules.PartyMember{{ID:"matthias",HP:7}},
  PartyPositions:map[string]tacticsrules.Cell{
   "matthias":{X:5,Y:2},
  },
  Enemies:[]tacticsrules.Enemy{
   {ID:"goblin",HP:5,Cell:tacticsrules.Cell{X:2,Y:2}},
  },
  EngagedEnemyIDs:[]string{"goblin"},
  Initiative:&Initiative{Round:1,Cursor:0,Order:[]tacticsrules.Actor{
   {Kind:"enemy",ID:"goblin"},{Kind:"party",ID:"matthias"},
  }},
 }
 b,_:=json.Marshal(state)
 return tacticsapi.Run{
  ID:"run-owned",Owner:"alice",MapID:"arena",Status:"active",
  WorldVersion:11,Map:tacticsapi.Map{ID:"arena",Version:3,Grid:enemyArenaGrid},State:b,
 }
}
func enemyStageCatalog()*catalog{
 return &catalog{world:PrivateMapContent{
  MapID:"arena",Revision:3,Grid:enemyArenaGrid,
 }}
}
func chaseProfile()map[string]tacticsrules.EnemyTurnProfile{
 return map[string]tacticsrules.EnemyTurnProfile{
  "goblin":{Movement:"cardinal-chase",AttackReach:1},
 }
}
func TestEnemyMoveStagesOneActionAndAdvancesToHero(t *testing.T){
 run:=enemyStageRun()
 before:=append([]byte(nil),run.State...)
 content:=enemyStageCatalog()
 stage,err:=StageEnemyNonAttack(context.Background(),run,content,chaseProfile())
 if err!=nil{t.Fatal(err)}
 if stage.Intent.Kind!="move"||stage.Intent.To==nil||
  *stage.Intent.To!=(tacticsrules.Cell{X:3,Y:2})||
  stage.NextActor!=(tacticsrules.Actor{Kind:"party",ID:"matthias"}){
  t.Fatalf("enemy did not move exactly one legal tile: %+v",stage)
 }
 state,_,err:=DecodeState(stage.Pending)
 if err!=nil||state.TurnPhase!="party"||state.Initiative.Cursor!=1||
  state.Initiative.Round!=1||state.Enemies[0].Cell!=(tacticsrules.Cell{X:3,Y:2})||
  state.Party[0].HP!=7{
  t.Fatalf("staged enemy movement broke round: %+v %v",state,err)
 }
 if stage.Pending.WorldVersion!=run.WorldVersion||
  !bytes.Equal(run.State,before){
  t.Fatal("AI stage mutated original or advanced world version")
 }
 if err:=validNext(run,stage.Pending);!errors.Is(err,ErrInvalidOutcome){
  t.Fatalf("uncommitted stage became eligible for CAS: %v",err)
 }
 // Replaying a pending enemy action while the hero has the turn is denied.
 if _,err:=StageEnemyNonAttack(context.Background(),stage.Pending,content,
  chaseProfile());!errors.Is(err,ErrCorruptRun){
  t.Fatalf("enemy acted twice: %v",err)
 }
}
func TestHoldConsumesOneInitiativeSlotWithoutDamage(t *testing.T){
 run:=enemyStageRun()
 hold:=map[string]tacticsrules.EnemyTurnProfile{"goblin":{
  Movement:"hold",AttackReach:1,
 }}
 stage,err:=StageEnemyNonAttack(context.Background(),run,enemyStageCatalog(),hold)
 if err!=nil{t.Fatal(err)}
 state,_,err:=DecodeState(stage.Pending)
 if err!=nil||stage.Intent.Kind!="wait"||
  state.Enemies[0].Cell!=(tacticsrules.Cell{X:2,Y:2})||
  state.TurnPhase!="party"||state.Initiative.Cursor!=1{
  t.Fatalf("hold was not a single wait action: %+v %+v %v",stage,state,err)
 }
}
func TestPendingHitCannotSpendEnemyTurnBeforeDamageResolver(t *testing.T){
 run:=enemyStageRun()
 var s StateV1
 if err:=json.Unmarshal(run.State,&s);err!=nil{t.Fatal(err)}
 s.PartyPositions["matthias"]=tacticsrules.Cell{X:3,Y:2}
 run.State,_=json.Marshal(s)
 prior:=append([]byte(nil),run.State...)
 stage,err:=StageEnemyNonAttack(context.Background(),run,enemyStageCatalog(),
  chaseProfile())
 if !errors.Is(err,ErrEnemyAttackUnresolved)||stage.Pending.ID!=""{
  t.Fatalf("enemy's attack bypassed real Effects: %+v %v",stage,err)
 }
 if !bytes.Equal(prior,run.State){t.Fatal("pending attack mutated run")}
}
func TestEnemyActionsRejectInvalidAuthoredMapAndProfile(t *testing.T){
 run:=enemyStageRun()
 for name,cat:=range map[string]*catalog{
  "missing":{err:errors.New("content unavailable")},
  "wrong revision":{world:PrivateMapContent{
   MapID:"arena",Revision:4,Grid:enemyArenaGrid}},
  "wrong grid":{world:PrivateMapContent{
   MapID:"arena",Revision:3,Grid:[]string{"##","#."}}},
  "wrong map":{world:PrivateMapContent{
   MapID:"impostor",Revision:3,Grid:enemyArenaGrid}},
 }{
  t.Run(name,func(t *testing.T){
   if _,err:=StageEnemyNonAttack(context.Background(),run,cat,
    chaseProfile());!errors.Is(err,ErrInvalidContent){
    t.Fatalf("invalid authored map accepted: %v",err)
   }
  })
 }
 if _,err:=StageEnemyNonAttack(context.Background(),run,nil,chaseProfile());
  !errors.Is(err,ErrInvalidContent){t.Fatal("nil catalog accepted")}
 if _,err:=StageEnemyNonAttack(context.Background(),run,enemyStageCatalog(),
  nil);!errors.Is(err,tacticsrules.ErrInvalidEnemyTurn){
  t.Fatalf("missing authored AI unexpectedly defaulted: %v",err)
 }
}
func TestEnemyStagePreservesOtherEngagedEnemiesAndRoundOrder(t *testing.T){
 run:=enemyStageRun()
 var state StateV1
 _=json.Unmarshal(run.State,&state)
 state.Enemies=append(state.Enemies,tacticsrules.Enemy{
  ID:"spider",HP:5,Cell:tacticsrules.Cell{X:3,Y:4},
 })
 state.EngagedEnemyIDs=append(state.EngagedEnemyIDs,"spider")
 state.Initiative.Order=[]tacticsrules.Actor{
  {Kind:"enemy",ID:"goblin"},{Kind:"enemy",ID:"spider"},
  {Kind:"party",ID:"matthias"},
 }
 run.State,_=json.Marshal(state)
 result,err:=StageEnemyNonAttack(context.Background(),run,enemyStageCatalog(),
  chaseProfile())
 if err!=nil{t.Fatal(err)}
 next,_,err:=DecodeState(result.Pending)
 if err!=nil||result.NextActor!=(tacticsrules.Actor{Kind:"enemy",ID:"spider"})||
  next.TurnPhase!="enemy"||next.Initiative.Cursor!=1||
  !reflect.DeepEqual(next.EngagedEnemyIDs,[]string{"goblin","spider"}){
  t.Fatalf("consecutive enemy actors lost initiative: %+v %+v %v",result,next,err)
 }
}
