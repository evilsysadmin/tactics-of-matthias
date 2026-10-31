package tacticsengine

import (
 "encoding/json"
 "errors"
 "testing"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)

// A Chronicles-style combat encounter with a living distant enemy who has
// not detected the party. Only "pawn" participates in the saved round.
func encounterRun() tacticsapi.Run {
 run:=savedRun()
 var state StateV1
 _=json.Unmarshal(run.State,&state)
 state.Phase="combat"
 state.Party=[]tacticsrules.PartyMember{
  {ID:"matthias",HP:7},{ID:"rook",HP:10},
 }
 state.PartyPositions=map[string]tacticsrules.Cell{
  "matthias":{X:1,Y:5},"rook":{X:2,Y:5},
 }
 state.Enemies=append(state.Enemies,tacticsrules.Enemy{
  ID:"far-goblin",HP:5,Cell:tacticsrules.Cell{X:5,Y:3},
 })
 state.EngagedEnemyIDs=[]string{"pawn"}
 state.Initiative=&Initiative{
  Round:1,Cursor:0,
  Order:[]tacticsrules.Actor{
   {Kind:"party",ID:"matthias"},
   {Kind:"enemy",ID:"pawn"},
   {Kind:"party",ID:"rook"},
  },
 }
 run.State,_=json.Marshal(state)
 return run
}

func mutateEncounter(run tacticsapi.Run,modify func(*StateV1)) tacticsapi.Run {
 var state StateV1
 _=json.Unmarshal(run.State,&state)
 modify(&state)
 run.State,_=json.Marshal(state)
 return run
}

func TestDecodeCombatHonorsOnlyPersistedEngagedEnemies(t *testing.T) {
 run:=encounterRun()
 state,board,err:=DecodeState(run)
 if err!=nil {t.Fatalf("valid engaged subset denied: %v",err)}
 if len(state.EngagedEnemyIDs)!=1||state.EngagedEnemyIDs[0]!="pawn" ||
  len(board.Enemies)!=2 || len(board.Initiative)!=3 {
  t.Fatalf("world roster incorrectly collapsed to combat roster: %+v / %+v",state,board)
 }
 // Rules advance the saved encounter, not the entire map roster.
 step,err:=tacticsrules.AdvanceEngagedTurn(board,state.Initiative.Round,state.EngagedEnemyIDs)
 if err!=nil || step.Next==nil || step.Next.ID!="pawn"{
  t.Fatalf("saved engagement cannot continue deterministically: %+v %v",step,err)
 }
}

func TestDecodeRejectsPhantomMissingOrCorruptedEngagements(t *testing.T) {
 base:=encounterRun()
 cases:=map[string]func(*StateV1){
  "no engagement roster":func(s *StateV1){s.EngagedEnemyIDs=nil},
  "duplicate engagement":func(s *StateV1){s.EngagedEnemyIDs=[]string{"pawn","pawn"}},
  "unknown engaged enemy":func(s *StateV1){s.EngagedEnemyIDs=[]string{"ghost"}},
  "unselected enemy in initiative":func(s *StateV1){
   s.Initiative.Order=append(s.Initiative.Order,tacticsrules.Actor{Kind:"enemy",ID:"far-goblin"})
  },
  "missing participating enemy slot":func(s *StateV1){
   s.Initiative.Order=[]tacticsrules.Actor{{Kind:"party",ID:"matthias"},{Kind:"party",ID:"rook"}}
  },
  "missing living party slot":func(s *StateV1){
   s.Initiative.Order=[]tacticsrules.Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"pawn"}}
  },
  "repeated initiative actor":func(s *StateV1){
   s.Initiative.Order=append(s.Initiative.Order,s.Initiative.Order[0])
  },
  "invisible formation member":func(s *StateV1){
   s.PartyPositions["ghost"]=tacticsrules.Cell{X:5,Y:5}
  },
  "out of bounds formation":func(s *StateV1){
   s.PartyPositions["rook"]=tacticsrules.Cell{X:99,Y:5}
  },
  "occupied living formation":func(s *StateV1){
   s.PartyPositions["rook"]=s.PartyPositions["matthias"]
  },
  "no living engaged enemies":func(s *StateV1){
   s.Enemies[0].HP=0
  },
  "wrong initiative cursor":func(s *StateV1){s.Initiative.Cursor=99},
 }
 for name,change:=range cases{
  t.Run(name,func(t *testing.T){
   corrupt:=mutateEncounter(base,change)
   if _,_,err:=DecodeState(corrupt);!errors.Is(err,ErrCorruptRun){
    t.Fatalf("corrupt saved encounter accepted: %v",err)
   }
  })
 }
}

func TestExplorationAndDefeatClearEncounterFootprintAndIDs(t *testing.T){
 base:=encounterRun()
 for name,change:=range map[string]func(*StateV1){
  "phantom ID in exploration":func(s *StateV1){
   s.Phase="explore";s.Initiative=nil;s.PartyPositions=nil
  },
  "phantom footprint in exploration":func(s *StateV1){
   s.Phase="explore";s.Initiative=nil;s.EngagedEnemyIDs=nil
  },
  "phantom ID in defeat":func(s *StateV1){
   s.Phase="defeated";s.Initiative=nil;s.PartyPositions=nil
   for i:=range s.Party{s.Party[i].HP=0}
  },
 } {
  t.Run(name,func(t *testing.T){
   bad:=mutateEncounter(base,change)
   if name=="phantom ID in defeat"{bad.Status="defeated"}
   if _,_,err:=DecodeState(bad);!errors.Is(err,ErrCorruptRun){
    t.Fatalf("combat ghosts survived transition: %v",err)
   }
  })
 }
 ended:=mutateEncounter(base,func(s *StateV1){
  s.Phase="explore";s.EngagedEnemyIDs=nil
  s.Initiative=nil;s.PartyPositions=nil
 })
 if _,_,err:=DecodeState(ended);err!=nil{
  t.Fatalf("clean exploration recovery rejected: %v",err)
 }
 defeated:=mutateEncounter(base,func(s *StateV1){
  s.Phase="defeated";s.EngagedEnemyIDs=nil
  s.Initiative=nil;s.PartyPositions=nil
  for i:=range s.Party{s.Party[i].HP=0}
 })
 defeated.Status="defeated"
 if _,_,err:=DecodeState(defeated);err!=nil{
  t.Fatalf("clean saved defeat rejected: %v",err)
 }
}

func TestValidatedEncounterAllowsAtomicNextVersionWithoutSpawningOtherEnemies(t *testing.T){
 original:=encounterRun()
 next:=mutateEncounter(original,func(s *StateV1){
  s.Initiative.Cursor=1
  s.Turns++
 })
 next.WorldVersion++
 if err:=validNext(original,next);err!=nil{
  t.Fatalf("valid single engaged turn rejected: %v",err)
 }
 after,_,err:=DecodeState(next)
 if err!=nil||len(after.EngagedEnemyIDs)!=1||after.EngagedEnemyIDs[0]!="pawn" {
  t.Fatalf("checkpoint silently changed encounter roster: %+v %v",after,err)
 }
}
