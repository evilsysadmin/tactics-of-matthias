package tacticsrules

// This module ports the authored on-entry Tactics trap semantics from
// chroniclesTacticsMove -> triggerTrapAtPosition -> chroniclesApplyContentAction.
// It is pure: callers MUST provide server-authored trap definitions and the
// server's persisted flags/party. It cannot query or fabricate local maps.

import (
  "errors"
  "fmt"
)

var ErrInvalidTrap = errors.New("invalid authoritative Tactics trap")

type TrapCondition struct {
  Key string `json:"key"`
  Equals bool `json:"equals"`
}
type TrapEffect struct {
  Type string `json:"type"`
  Key string `json:"key,omitempty"`
  Value *bool `json:"value,omitempty"`
  Amount *int `json:"amount,omitempty"`
}
type AuthoredTrap struct {
  ID string `json:"id"`
  Cell Cell `json:"cell"`
  When []TrapCondition `json:"when"`
  Effects []TrapEffect `json:"effects"`
}
type TrapState struct {
  Phase string
  Flags map[string]bool
  Party []PartyMember
}
type TrapResult struct {
  State TrapState
  TriggeredID string
}

func safeTrapID(s string)bool{
 if len(s)<1 || len(s)>128{return false}
 for i:=0;i<len(s);i++{
  c:=s[i]
  if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='_'||c=='-'){
   return false
  }
 }
 return true
}
func safeFlagKey(s string)bool{
 if len(s)<1 || len(s)>64{return false}
 for i:=0;i<len(s);i++{
  c:=s[i]
  if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='_'||c=='-'){
   return false
  }
 }
 return true
}

// ValidateAuthoredTraps rejects *all* unsupported content before any effect
// is applied, including traps elsewhere on the map. Partial/unknown content
// must never silently be treated as a harmless passage.
func ValidateAuthoredTraps(grid []string,traps []AuthoredTrap)error{
 if !validGrid(grid)||len(traps)>128{return ErrInvalidTrap}
 ids:=map[string]bool{}
 cells:=map[Cell]bool{}
 for _,trap:=range traps {
  if !safeTrapID(trap.ID)||ids[trap.ID]||cells[trap.Cell]||
   trap.Cell.Y<0||trap.Cell.Y>=len(grid)||
   trap.Cell.X<0||trap.Cell.X>=len(grid[0])||
   !traversable(grid[trap.Cell.Y][trap.Cell.X])||
   len(trap.When)>16||len(trap.Effects)==0||len(trap.Effects)>16{
   return ErrInvalidTrap
  }
  ids[trap.ID]=true;cells[trap.Cell]=true
  for _,condition:=range trap.When{
   if !safeFlagKey(condition.Key){return ErrInvalidTrap}
  }
  for _,effect:=range trap.Effects{
   switch effect.Type{
   case "set":
    if !safeFlagKey(effect.Key)||effect.Value==nil||effect.Amount!=nil{
     return ErrInvalidTrap
    }
   case "damage-party":
    if effect.Key!=""||effect.Value!=nil||effect.Amount==nil||
     *effect.Amount<0||*effect.Amount>10000{return ErrInvalidTrap}
   default:
    return fmt.Errorf("%w: unsupported effect %q",ErrInvalidTrap,effect.Type)
   }
  }
 }
 return nil
}

// ApplyEntryTrap executes the FIRST authored trap at the entered tile iff
// every condition matches an EXPLICITLY stored flag value. Missing is not
// false: Chronicles' original 'equals: false' requires an actual false flag.
//
// Tactics uses straight deterministic damage. The MM3 luck-saving mechanic
// belongs to Chronicles' separate MM3 rules and is not applied here.
func ApplyEntryTrap(grid []string,traps []AuthoredTrap,
 state TrapState,entered Cell)(TrapResult,error){
 if err:=ValidateAuthoredTraps(grid,traps);err!=nil{return TrapResult{},err}
 if state.Phase!="explore"&&state.Phase!="combat"{
  return TrapResult{},ErrInvalidTrap
 }
 if entered.Y<0||entered.Y>=len(grid)||entered.X<0||
  entered.X>=len(grid[0])||!traversable(grid[entered.Y][entered.X]){
  return TrapResult{},ErrInvalidTrap
 }
 if len(state.Party)==0||len(state.Party)>16||len(state.Flags)>256{
  return TrapResult{},ErrInvalidTrap
 }
 roster:=map[string]bool{}
 for _,member:=range state.Party{
  if !safeTrapID(member.ID)||member.HP<0||roster[member.ID]{
   return TrapResult{},ErrInvalidTrap
  }
  roster[member.ID]=true
 }
 flags:=make(map[string]bool,len(state.Flags))
 for key,value:=range state.Flags{
  if !safeFlagKey(key){return TrapResult{},ErrInvalidTrap}
  flags[key]=value
 }
 party:=append([]PartyMember(nil),state.Party...)
 result:=TrapResult{State:TrapState{Phase:state.Phase,Flags:flags,Party:party}}
 for _,trap:=range traps{
  if trap.Cell!=entered{continue}
  active:=true
  for _,condition:=range trap.When{
   value,exists:=flags[condition.Key]
   if !exists||value!=condition.Equals{active=false;break}
  }
  if !active{continue}
  for _,effect:=range trap.Effects{
   switch effect.Type{
   case "set": flags[effect.Key]=*effect.Value
   case "damage-party":
    for i,member:=range party{
     if member.HP==0{continue}
     damage:=*effect.Amount
     if damage>member.HP{damage=member.HP}
     party[i].HP-=damage
    }
   }
  }
  result.TriggeredID=trap.ID
  defeated:=true
  for _,member:=range party{if member.HP>0{defeated=false;break}}
  if defeated{result.State.Phase="defeated"}
  break
 }
 return result,nil
}
