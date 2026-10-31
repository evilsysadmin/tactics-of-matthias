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

func encounterEntryRun() tacticsapi.Run {
 run:=savedRun()
 state,_,_:=DecodeState(run)
 state.Enemies=append(state.Enemies,tacticsrules.Enemy{
  ID:"far-goblin",HP:8,Cell:tacticsrules.Cell{X:5,Y:3},
 })
 run.State,_=json.Marshal(state)
 return run
}
func entryProfiles()map[string]tacticsrules.EnemyEngagement{
 return map[string]tacticsrules.EnemyEngagement{
  "pawn":{AttackReach:2},"far-goblin":{AttackReach:1},
 }
}
func entryAgility()tacticsrules.Agility{
 return tacticsrules.Agility{
  Party:map[string]int64{
   "matthias":4,"rook":2,"bishop":3,"knight":5,
  },
  Enemies:map[string]int64{"pawn":7},
 }
}
func entryCatalog() *catalog{
 return &catalog{world:PrivateMapContent{
  MapID:"crypt",Revision:0,Grid:crypt,
 }}
}
func TestStartEncounterComposesRealSightDeploymentAndInitiative(t *testing.T){
 run:=encounterEntryRun()
 before:=append([]byte(nil),run.State...)
 c:=entryCatalog()
 staged,err:=StageEncounterStart(context.Background(),run,c,
  entryProfiles(),entryAgility(),bytes.NewReader([]byte{0,0,0,0,7}))
 if err!=nil{t.Fatal(err)}
 if c.calls!=1 || staged.InitialActor!=(tacticsrules.Actor{Kind:"enemy",ID:"pawn"})||
  !reflect.DeepEqual(staged.EngagedEnemyIDs,[]string{"pawn"}){
  t.Fatalf("enemy did not open combat: %+v",staged)
 }
 next,board,err:=DecodeState(staged.Pending)
 if err!=nil{t.Fatal(err)}
 if next.Phase!="combat"||next.TurnPhase!="enemy"||
  len(next.PartyPositions)!=4||len(next.Initiative.Order)!=5||
  next.Initiative.Round!=1||next.Initiative.Cursor!=0||
  len(board.Enemies)!=2{
  t.Fatalf("invalid encounter snapshot: %+v / %+v",next,board)
 }
 for _,cell:=range next.PartyPositions{
  if cell==(tacticsrules.Cell{X:3,Y:5})||cell==(tacticsrules.Cell{X:5,Y:3}){
   t.Fatalf("party deployed onto alive enemy: %+v",next.PartyPositions)
  }
 }
 for _,actor:=range next.Initiative.Order{
  if actor.ID=="far-goblin"{t.Fatal("distant enemy got initiative")}
 }
 if !bytes.Equal(run.State,before)||staged.Pending.WorldVersion!=run.WorldVersion{
  t.Fatal("starting encounter mutated or prematurely committed the run")
 }
 if err:=validNext(run,staged.Pending);!errors.Is(err,ErrInvalidOutcome){
  t.Fatalf("incomplete enemy-first stage is CAS-eligible: %v",err)
 }
}
func TestNoVisibleEnemyCannotCreateCombatOrConsumeRandomness(t *testing.T){
 run:=encounterEntryRun()
 state,_,_:=DecodeState(run)
 state.Enemies[0].HP=0
 run.State,_=json.Marshal(state)
 result,err:=StageEncounterStart(context.Background(),run,entryCatalog(),
  entryProfiles(),entryAgility(),bytes.NewReader(nil))
 if !errors.Is(err,tacticsrules.ErrNoEngagement)||
  len(result.EngagedEnemyIDs)!=0{
  t.Fatalf("invisible enemy started an initiative: %+v %v",result,err)
 }
}
func TestEncounterCannotUseUntrustedOrChangedAuthoredContent(t *testing.T){
 run:=encounterEntryRun()
 for _,tc:=range []struct{
  name string
  c *catalog
 }{
  {"catalog not available",&catalog{err:errors.New("offline")}},
  {"wrong revision",&catalog{world:PrivateMapContent{
   MapID:"crypt",Revision:7,Grid:crypt,
  }}},
  {"wrong terrain",&catalog{world:PrivateMapContent{
   MapID:"crypt",Revision:0,Grid:[]string{"#..#"},
  }}},
  {"wrong ID",&catalog{world:PrivateMapContent{
   MapID:"intruder",Revision:0,Grid:crypt,
  }}},
 }{
  t.Run(tc.name,func(t *testing.T){
   if _,err:=StageEncounterStart(context.Background(),run,tc.c,
    entryProfiles(),entryAgility(),bytes.NewReader([]byte{0,0,0,0,7}));
    !errors.Is(err,ErrInvalidContent){
    t.Fatalf("incompatible private catalog accepted: %v",err)
   }
  })
 }
 if _,err:=StageEncounterStart(context.Background(),run,nil,
  entryProfiles(),entryAgility(),bytes.NewReader(nil));
  !errors.Is(err,ErrInvalidContent){t.Fatal("nil catalog accepted")}
 if _,err:=StageEncounterStart(context.Background(),run,entryCatalog(),
  nil,entryAgility(),bytes.NewReader(nil));err==nil{
  t.Fatal("missing authored enemy engagement profile accepted")
 }
 if _,err:=StageEncounterStart(context.Background(),run,entryCatalog(),
  entryProfiles(),tacticsrules.Agility{},bytes.NewReader(nil));err==nil{
  t.Fatal("missing server-owned agility accepted")
 }
}
func TestUnreachableFormationCannotStartEnemyInitiative(t *testing.T){
 run:=encounterEntryRun()
 grid:=[]string{"#####","#PE##","#####"}
 state,_,_:=DecodeState(run)
 state.X,state.Y=1,1
 state.Party=[]tacticsrules.PartyMember{
  {ID:"matthias",HP:7},{ID:"rook",HP:10},
  {ID:"bishop",HP:6},{ID:"knight",HP:8},
 }
 state.Enemies=[]tacticsrules.Enemy{
  {ID:"pawn",HP:4,Cell:tacticsrules.Cell{X:2,Y:1}},
 }
 run.Map=tacticsapi.Map{ID:"crypt",Grid:grid}
 run.State,_=json.Marshal(state)
 cat:=&catalog{world:PrivateMapContent{MapID:"crypt",Grid:grid}}
 if _,err:=StageEncounterStart(context.Background(),run,cat,
  entryProfiles(),entryAgility(),bytes.NewReader([]byte{7,7,7,7,7}));
  !errors.Is(err,tacticsrules.ErrNoFormation){
  t.Fatalf("combat assembled without a legal formation: %v",err)
 }
}
func TestCannotStartTwiceOrFromTerminalSnapshot(t *testing.T){
 run:=encounterRun()
 if _,err:=StageEncounterStart(context.Background(),run,entryCatalog(),
  entryProfiles(),entryAgility(),bytes.NewReader(nil));
  !errors.Is(err,ErrCorruptRun){
  t.Fatalf("started a second initiative: %v",err)
 }
}
