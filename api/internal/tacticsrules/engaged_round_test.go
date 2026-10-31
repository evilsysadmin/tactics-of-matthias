package tacticsrules

import (
 "bytes"
 "errors"
 "reflect"
 "testing"
)

func encounterWithDistantEnemy() Snapshot {
 b:=encounterBoard()
 b.Enemies=[]Enemy{
  {ID:"spider",HP:4,Cell:Cell{X:2,Y:1}},
  {ID:"far-goblin",HP:7,Cell:Cell{X:7,Y:4}},
 }
 return b
}
func TestOnlyEngagedEnemiesGetAgilityRollsAndTurns(t *testing.T){
 b:=encounterWithDistantEnemy()
 profiles:=map[string]EnemyEngagement{
  "spider":{AttackReach:1},
  "far-goblin":{AttackReach:1},
 }
 result,err:=RollEngagedInitiative(b,profiles,Agility{
  Party:map[string]int64{"matthias":3},
  Enemies:map[string]int64{"spider":5}, // far monster has NO roll or stats lookup
 },bytes.NewReader([]byte{3,6}))
 if err!=nil{t.Fatal(err)}
 if !reflect.DeepEqual(result.EngagedEnemyIDs,[]string{"spider"}){
  t.Fatalf("non-engaged enemy entered combat: %+v",result)
 }
 order:=result.Initiative.Order
 if len(order)!=2||order[0].ID!="spider"||order[1].ID!="matthias"||
  order[0].Initiative!=12||order[1].Initiative!=7{
  t.Fatalf("server AGI+1d8 subset wrong: %+v",order)
 }
 b.Phase="combat"
 b.Initiative=[]Actor{order[0].Actor,order[1].Actor}
 b.InitiativeCursor=0
 step,err:=AdvanceEngagedTurn(b,1,result.EngagedEnemyIDs)
 if err!=nil||step.Next==nil||step.Next.ID!="matthias"||step.Round!=1{
  t.Fatalf("distant enemy should not get a turn: %+v %v",step,err)
 }
 b.InitiativeCursor=1
 step,err=AdvanceEngagedTurn(b,1,result.EngagedEnemyIDs)
 if err!=nil||step.Next==nil||step.Next.ID!="spider"||step.Round!=2{
  t.Fatalf("round wrap must preserve engaged enemy: %+v %v",step,err)
 }
}
func TestCombatEndsWhenLastEngagedEnemyDiesNotLastWorldEnemy(t *testing.T){
 b:=encounterWithDistantEnemy()
 b.Phase="combat"
 b.Initiative=[]Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"spider"}}
 b.InitiativeCursor=0
 b.Enemies[0].HP=0
 end,err:=AdvanceEngagedTurn(b,3,[]string{"spider"})
 if err!=nil||!end.Ended||end.Phase!="explore"||
  b.Enemies[1].HP!=7{
  t.Fatalf("unengaged live world monster held combat hostage: %+v %v",end,err)
 }
}
func TestMalformedSubsetCannotSilentlySkipAnEngagedTurn(t *testing.T){
 base:=encounterWithDistantEnemy()
 base.Phase="combat"
 base.Initiative=[]Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"spider"}}
 for _,tc:=range []struct{ids []string;initiative []Actor;party []PartyMember}{
  {nil,base.Initiative,base.Party},
  {[]string{"spider","spider"},base.Initiative,base.Party},
  {[]string{"ghost"},base.Initiative,base.Party},
  {[]string{"spider","far-goblin"},base.Initiative,base.Party},
  {[]string{"spider"},[]Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"far-goblin"}},base.Party},
  {[]string{"spider"},[]Actor{{Kind:"enemy",ID:"spider"}},base.Party},
  {[]string{"spider"},[]Actor{{Kind:"party",ID:"matthias"},{Kind:"enemy",ID:"spider"},{Kind:"enemy",ID:"spider"}},base.Party},
 }{
  b:=base;b.Initiative=tc.initiative;b.Party=tc.party
  if _,err:=AdvanceEngagedTurn(b,1,tc.ids);!errors.Is(err,ErrInvalidInitiative){
   t.Errorf("invented/missing subset %#v / %#v accepted: %v",tc.ids,tc.initiative,err)
  }
 }
}
func TestNoEngagementDoesNotConsumeEntropyOrCreateCombat(t *testing.T){
 b:=encounterWithDistantEnemy()
 b.Enemies[0].HP=0
 result,err:=RollEngagedInitiative(b,map[string]EnemyEngagement{
  "far-goblin":{AttackReach:1},
 },Agility{},bytes.NewReader(nil))
 if !errors.Is(err,ErrNoEngagement)||len(result.Initiative.Order)!=0{
  t.Fatalf("no visible opponents must not roll combat: %+v %v",result,err)
 }
}
