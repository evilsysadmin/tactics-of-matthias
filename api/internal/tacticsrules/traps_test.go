package tacticsrules

import (
 "errors"
 "reflect"
 "testing"
)

func ptrBool(value bool)*bool{return &value}
func ptrInt(value int)*int{return &value}
var ashVaultGrid=[]string{
 "######","#....#","#....#","#....#","#....#","#....#","######",
}
func spikeTrap() AuthoredTrap {
 return AuthoredTrap{
  ID:"test-spike-plate",Cell:Cell{1,4},
  When:[]TrapCondition{{Key:"testTrapSpent",Equals:false}},
  Effects:[]TrapEffect{
   {Type:"set",Key:"testTrapSpent",Value:ptrBool(true)},
   {Type:"damage-party",Amount:ptrInt(2)},
  },
 }
}
func TestChroniclesTrapFirstEntryReentryAndInactiveFlag(t *testing.T){
 prior:=TrapState{
  Phase:"explore",Flags:map[string]bool{"testTrapSpent":false},
  Party:[]PartyMember{{"matthias",7},{"rook",0},{"bishop",6}},
 }
 originalHP:=append([]PartyMember(nil),prior.Party...)
 entry,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  prior,Cell{1,4})
 if err!=nil{t.Fatal(err)}
 if entry.TriggeredID!="test-spike-plate"||entry.State.Phase!="explore"||
  !entry.State.Flags["testTrapSpent"]{
  t.Fatalf("trap was not spent: %+v",entry)
 }
 want:=[]PartyMember{{"matthias",5},{"rook",0},{"bishop",4}}
 if !reflect.DeepEqual(entry.State.Party,want){
  t.Fatalf("damaged dead member or wrong amount: %+v",entry.State.Party)
 }
 if !reflect.DeepEqual(prior.Party,originalHP)||prior.Flags["testTrapSpent"]{
  t.Fatal("mutated authoritative pre-action snapshot")
 }
 // Walking away and returning must NOT trigger spent traps again.
 away,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  entry.State,Cell{1,5})
 if err!=nil||away.TriggeredID!=""{t.Fatalf("away: %+v %v",away,err)}
 back,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  away.State,Cell{1,4})
 if err!=nil||back.TriggeredID!=""||
  !reflect.DeepEqual(back.State.Party,want){
  t.Fatalf("spent trap activated twice: %+v %v",back,err)
 }
 missing:=prior
 missing.Flags=map[string]bool{}
 unseen,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  missing,Cell{1,4})
 if err!=nil||unseen.TriggeredID!=""{
  t.Fatalf("missing flag is not equals:false: %+v %v",unseen,err)
 }
}
func TestTrapDamageCanDefeatEntirePartyButNotReviveFallen(t *testing.T){
 before:=TrapState{Phase:"explore",
  Flags:map[string]bool{"testTrapSpent":false},
  Party:[]PartyMember{{"matthias",1},{"rook",0}},
 }
 result,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  before,Cell{1,4})
 if err!=nil||result.State.Phase!="defeated"||
  result.State.Party[0].HP!=0||result.State.Party[1].HP!=0{
  t.Fatalf("full knockout should cause defeat: %+v %v",result,err)
 }
 if result.State.Flags["testTrapSpent"]!=true{
  t.Fatal("trap flag must persist even on defeat")
 }
 // A stale turn must not apply a second trap after defeat.
 if _,err:=ApplyEntryTrap(ashVaultGrid,[]AuthoredTrap{spikeTrap()},
  result.State,Cell{1,4});!errors.Is(err,ErrInvalidTrap){
  t.Fatalf("terminal trap damage accepted: %v",err)
 }
}
func TestTrapValidatesAllDefinitionsAndRejectsUnknownEffects(t *testing.T){
 invalid:=[]AuthoredTrap{
  {ID:"unknown",Cell:Cell{2,3},Effects:[]TrapEffect{{Type:"grant-gold"}}},
  {ID:"wall",Cell:Cell{0,0},Effects:spikeTrap().Effects},
  {ID:"duplicate",Cell:Cell{1,4},Effects:spikeTrap().Effects},
  {ID:"bad-damage",Cell:Cell{2,3},Effects:[]TrapEffect{{Type:"damage-party",Amount:ptrInt(-1)}}},
  {ID:"missing-set",Cell:Cell{2,3},Effects:[]TrapEffect{{Type:"set",Key:"flag"}}},
 }
 state:=TrapState{Phase:"explore",Flags:map[string]bool{"testTrapSpent":false},
  Party:[]PartyMember{{ID:"matthias",HP:7}},
 }
 for _,bad:=range invalid{
  authoring:=[]AuthoredTrap{spikeTrap(),bad}
  if _,err:=ApplyEntryTrap(ashVaultGrid,authoring,state,Cell{1,4});
    !errors.Is(err,ErrInvalidTrap){
   t.Errorf("invalid authored data accepted: %+v (%v)",bad,err)
  }
 }
}
func TestEmptyTrapListIsSafeAndLiveEntryMustBeWalkable(t *testing.T){
 state:=TrapState{Phase:"combat",Party:[]PartyMember{{ID:"rook",HP:8}}}
 result,err:=ApplyEntryTrap(ashVaultGrid,nil,state,Cell{2,4})
 if err!=nil||result.TriggeredID!=""||result.State.Party[0].HP!=8{
  t.Fatalf("empty authored trap list changed state %+v %v",result,err)
 }
 if _,err:=ApplyEntryTrap(ashVaultGrid,nil,state,Cell{0,0});
  !errors.Is(err,ErrInvalidTrap){
  t.Fatalf("entered wall accepted: %v",err)
 }
}
