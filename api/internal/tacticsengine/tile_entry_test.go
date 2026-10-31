package tacticsengine

import (
 "context"
 "encoding/json"
 "errors"
 "reflect"
 "strings"
 "testing"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

var vault=[]string{
 "######","#....#","#....#","#....#","#....#","#....#","######",
}
func asBool(v bool)*bool{return &v}
func asInt(v int)*int{return &v}
func privateTrap()tacticsrules.AuthoredTrap{
 return tacticsrules.AuthoredTrap{
  ID:"spike-plate",Cell:tacticsrules.Cell{X:1,Y:4},
  When:[]tacticsrules.TrapCondition{{Key:"testTrapSpent",Equals:false}},
  Effects:[]tacticsrules.TrapEffect{
   {Type:"set",Key:"testTrapSpent",Value:asBool(true)},
   {Type:"damage-party",Amount:asInt(2)},
  },
 }
}
type catalog struct {
 world PrivateMapContent
 err error
 calls int
}
func (c *catalog)LoadMapContent(_ context.Context,_ string)(PrivateMapContent,error){
 c.calls++;return c.world,c.err
}
func vaultRun(hp int)tacticsapi.Run{
 state:=StateV1{
  Version:1,MapID:"ash-vault",X:1,Y:5,Phase:"explore",TurnPhase:"party",
  Turns:6,Flags:map[string]bool{"testTrapSpent":false},
  Party:[]tacticsrules.PartyMember{{ID:"matthias",HP:hp},{ID:"rook",HP:0}},
  Enemies:[]tacticsrules.Enemy{},
 }
 raw,_:=json.Marshal(state)
 return tacticsapi.Run{
  ID:"owner-run",Owner:"alice",MapID:"ash-vault",Status:"active",WorldVersion:7,
  Map:tacticsapi.Map{ID:"ash-vault",Grid:vault},
  State:raw,
 }
}
func step(t *testing.T,run tacticsapi.Run,to tacticsrules.Cell,c *catalog)MoveStage{
 t.Helper()
 _,board,err:=DecodeState(run);if err!=nil{t.Fatal(err)}
 decision,err:=tacticsrules.DecideMove(board,"matthias",to)
 if err!=nil{t.Fatal(err)}
 stage,err:=StageMoveEntry(context.Background(),run,decision,c)
 if err!=nil{t.Fatal(err)}
 return stage
}
func TestServerOwnedTrapFirstEntryThenReentry(t *testing.T){
 authored:=&catalog{world:PrivateMapContent{MapID:"ash-vault",Grid:vault,
  Traps:[]tacticsrules.AuthoredTrap{privateTrap()}}}
 run:=vaultRun(7)
 stage:=step(t,run,tacticsrules.Cell{X:1,Y:4},authored)
 if stage.TriggeredTrapID!="spike-plate"||stage.AdvancesTurn||
  stage.Pending.WorldVersion!=run.WorldVersion||stage.Pending.Status!="active"{
  t.Fatalf("wrong stage: %+v",stage)
 }
 state,_,err:=DecodeState(stage.Pending)
 if err!=nil||state.X!=1||state.Y!=4||state.Turns!=6||
  !state.Flags["testTrapSpent"]||state.Party[0].HP!=5||state.Party[1].HP!=0{
  t.Fatalf("trap did not persist into stage: %+v %v",state,err)
 }
 original,_,_:=DecodeState(run)
 if original.Party[0].HP!=7||original.Flags["testTrapSpent"]{
  t.Fatal("input run changed before CAS")
 }
 away:=step(t,stage.Pending,tacticsrules.Cell{X:1,Y:5},authored)
 back:=step(t,away.Pending,tacticsrules.Cell{X:1,Y:4},authored)
 next,_,_:=DecodeState(back.Pending)
 if back.TriggeredTrapID!=""||next.Party[0].HP!=5||next.Turns!=6{
  t.Fatalf("one-time trap retriggered on reentry: %+v",next)
 }
 if authored.calls!=3{t.Fatalf("expected fresh private lookup per move; got %d",authored.calls)}
 // Neither the run nor its bootstrap JSON exposes private trap definitions.
 wire,_:=json.Marshal(stage.Pending)
 if strings.Contains(string(wire),"spike-plate")||strings.Contains(string(wire),"damage-party"){
  t.Fatal("private trap leaked into bootstrap run")
 }
}
func TestFatalTrapCanStageValidatedDefeat(t *testing.T){
 c:=&catalog{world:PrivateMapContent{MapID:"ash-vault",Grid:vault,
  Traps:[]tacticsrules.AuthoredTrap{privateTrap()}}}
 run:=vaultRun(1)
 result:=step(t,run,tacticsrules.Cell{X:1,Y:4},c)
 state,_,err:=DecodeState(result.Pending)
 if err!=nil||result.Pending.Status!="defeated"||
  state.Phase!="defeated"||state.Initiative!=nil||state.Party[0].HP!=0{
  t.Fatalf("fatal trap must stage defeat: %+v %v",state,err)
 }
 // Stage still has the OLD version; only the complete reducer may advance
 // worldVersion and make the server accept this terminal checkpoint.
 if err:=validNext(run,result.Pending);!errors.Is(err,ErrInvalidOutcome){
  t.Fatalf("incomplete stage could be committed: %v",err)
 }
 confirmed:=result.Pending
 confirmed.WorldVersion++
 if err:=validNext(run,confirmed);err!=nil{
  t.Fatalf("consistent terminal state should validate: %v",err)
 }
}
func TestPrivateCatalogMustMatchStoredMapAndRejectUnknownEffects(t *testing.T){
 run:=vaultRun(7)
 _,board,_:=DecodeState(run)
 decision,_:=tacticsrules.DecideMove(board,"matthias",tacticsrules.Cell{X:1,Y:4})
 valid:=PrivateMapContent{MapID:"ash-vault",Grid:vault,
  Traps:[]tacticsrules.AuthoredTrap{privateTrap()}}
 for _,alter:=range []func(*PrivateMapContent){
  func(c *PrivateMapContent){c.MapID="foreign"},
  func(c *PrivateMapContent){c.Grid=[]string{"#..#"}},
  func(c *PrivateMapContent){c.Traps[0].Effects=append(
   c.Traps[0].Effects,tacticsrules.TrapEffect{Type:"grant-gold"})},
 }{
  content:=PrivateMapContent{MapID:valid.MapID,Grid:append([]string(nil),valid.Grid...),
   Traps:append([]tacticsrules.AuthoredTrap(nil),valid.Traps...)}
  alter(&content)
  _,err:=StageMoveEntry(context.Background(),run,decision,&catalog{world:content})
  if !errors.Is(err,ErrInvalidContent){t.Fatalf("unapproved authoring passed: %v",err)}
 }
 c:=&catalog{world:valid,err:errors.New("catalog outage")}
 if _,err:=StageMoveEntry(context.Background(),run,decision,c);
  !errors.Is(err,ErrInvalidContent){t.Fatal("catalog failure was silently ignored")}
 if _,err:=StageMoveEntry(context.Background(),run,decision,nil);
  !errors.Is(err,ErrInvalidContent){t.Fatal("missing catalog accepted")}
}
func TestForgedMoveNeverCallsPrivateCatalog(t *testing.T){
 run:=vaultRun(7)
 c:=&catalog{world:PrivateMapContent{MapID:"ash-vault",Grid:vault}}
 fake:=tacticsrules.Decision{
  MemberID:"matthias",From:tacticsrules.Cell{X:1,Y:5},
  Move:tacticsrules.Move{To:tacticsrules.Cell{X:4,Y:1}},
 }
 if _,err:=StageMoveEntry(context.Background(),run,fake,c);
  !errors.Is(err,tacticsapi.ErrIllegalAction){t.Fatalf("forged move: %v",err)}
 if c.calls!=0{t.Fatal("invalid action fetched private content")}
}
func TestStateFlagsAndTerminalMustBeCoherent(t *testing.T){
 run:=vaultRun(1)
 var state StateV1
 if err:=json.Unmarshal(run.State,&state);err!=nil{t.Fatal(err)}
 for _,change:=range []func(*StateV1,*tacticsapi.Run){
  func(s *StateV1,r *tacticsapi.Run){s.Flags=map[string]bool{"bad key":false}},
  func(s *StateV1,r *tacticsapi.Run){s.Turns=-1},
  func(s *StateV1,r *tacticsapi.Run){s.Direction=10},
  func(s *StateV1,r *tacticsapi.Run){r.Status="defeated"},
  func(s *StateV1,r *tacticsapi.Run){s.Phase="defeated"},
  func(s *StateV1,r *tacticsapi.Run){s.Party[0].HP=0},
 }{
  copyState:=state
  copyState.Party=append([]tacticsrules.PartyMember(nil),state.Party...)
  next:=run
  change(&copyState,&next)
  raw,_:=json.Marshal(copyState)
  next.State=raw
  if _,_,err:=DecodeState(next);!errors.Is(err,ErrCorruptRun){
   t.Errorf("corrupted flags/terminal state accepted: %+v, status=%s, err=%v",
    copyState,next.Status,err)
  }
 }
 copyState:=state
 copyState.Party=[]tacticsrules.PartyMember{{ID:"matthias",HP:0},{ID:"rook",HP:0}}
 copyState.Phase="defeated"
 copyState.Initiative=nil
 next:=run
 next.Status="defeated"
 next.State,_=json.Marshal(copyState)
 if _,_,err:=DecodeState(next);err!=nil{t.Fatalf("valid terminal denied: %v",err)}
 if reflect.DeepEqual(run,next){t.Fatal("terminal changed no state")}
}

func TestStageRequiresExactMapRevisionEvenIfGridMatches(t *testing.T){
 run:=vaultRun(7);run.Map.Version=3
 _,board,err:=DecodeState(run);if err!=nil{t.Fatal(err)}
 move,err:=tacticsrules.DecideMove(board,"matthias",tacticsrules.Cell{X:1,Y:4})
 if err!=nil{t.Fatal(err)}
 content:=PrivateMapContent{
  MapID:"ash-vault",Revision:4,Grid:vault,
  Traps:[]tacticsrules.AuthoredTrap{privateTrap()},
 }
 if _,err:=StageMoveEntry(context.Background(),run,move,&catalog{world:content});
  !errors.Is(err,ErrInvalidContent){t.Fatalf("wrong map revision accepted: %v",err)}
 content.Revision=3
 if _,err:=StageMoveEntry(context.Background(),run,move,&catalog{world:content});
  err!=nil{t.Fatalf("correct map revision rejected: %v",err)}
}

func TestFatalCombatTrapClearsEngagementAsWellAsInitiative(t *testing.T){
 run:=encounterRun()
 var state StateV1
 if err:=json.Unmarshal(run.State,&state);err!=nil{t.Fatal(err)}
 // The only living hero is Matthias. A fatal spike after a legal
 // initiative move must not leave orphan combat IDs in the defeated run.
 for i:=range state.Party{
  if state.Party[i].ID=="rook"{state.Party[i].HP=0}
 }
 run.State,_=json.Marshal(state)
 _,board,err:=DecodeState(run);if err!=nil{t.Fatal(err)}
 dec,err:=tacticsrules.DecideMove(board,"matthias",tacticsrules.Cell{X:1,Y:4})
 if err!=nil{t.Fatal(err)}
 hit:=tacticsrules.AuthoredTrap{
  ID:"fatal-spikes",Cell:tacticsrules.Cell{X:1,Y:4},
  Effects:[]tacticsrules.TrapEffect{{Type:"damage-party",Amount:asInt(20)}},
 }
 content:=&catalog{world:PrivateMapContent{
  MapID:"crypt",Revision:run.Map.Version,Grid:crypt,
  Traps:[]tacticsrules.AuthoredTrap{hit},
 }}
 stage,err:=StageMoveEntry(context.Background(),run,dec,content)
 if err!=nil{t.Fatalf("valid lethal combat trap rejected: %v",err)}
 ended,_,err:=DecodeState(stage.Pending)
 if err!=nil||stage.Pending.Status!="defeated"||ended.Phase!="defeated"||
  ended.Initiative!=nil||len(ended.EngagedEnemyIDs)!=0||
  len(ended.PartyPositions)!=0||ended.Party[0].HP!=0 {
  t.Fatalf("combat ghosts survived fatal trap: %+v %v",ended,err)
 }
 if stage.Pending.WorldVersion!=run.WorldVersion{
  t.Fatal("stage should remain uncommitted")
 }
 next:=stage.Pending
 next.WorldVersion++
 if err:=validNext(run,next);err!=nil{
  t.Fatalf("complete defeated outcome should be valid after full resolution: %v",err)
 }
}
