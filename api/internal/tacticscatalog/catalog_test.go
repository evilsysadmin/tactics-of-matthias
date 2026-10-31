package tacticscatalog

import (
 "context"
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsengine"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)
var ashGrid=[]string{
 "#########","#...#..X#","#.#.#.#.#","#.......#",
 "#.###.#.#","#P......#","#########",
}
func manifestFixture()map[string]any{
 return map[string]any{
  "schemaVersion":1,"mapRevision":3,"mapId":"ash-vault","grid":ashGrid,
  "traps":[]any{map[string]any{
   "id":"vault-slag-vent",
   "cell":map[string]int{"x":5,"y":5},
   "when":[]any{map[string]any{"key":"vaultSlagVentSpent","equals":false}},
   "effects":[]any{
    map[string]any{"type":"set","key":"vaultSlagVentSpent","value":true},
    map[string]any{"type":"damage-party","amount":1},
   },
  }},
 }
}
func writeManifest(t *testing.T,dir,id string,m any){
 t.Helper()
 b,err:=json.Marshal(m);if err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(dir,id+".json"),b,0600);err!=nil{t.Fatal(err)}
}
func TestCatalogUsesAshVaultRevision3AndKeepsTrapHidden(t *testing.T){
 dir:=t.TempDir()
 writeManifest(t,dir,"ash-vault",manifestFixture())
 c,err:=NewDirectory(dir);if err!=nil{t.Fatal(err)}
 defer c.Close()
 loaded,err:=c.LoadMapContent(context.Background(),"ash-vault")
 if err!=nil||loaded.MapID!="ash-vault"||loaded.Revision!=3||len(loaded.Traps)!=1{
  t.Fatalf("invalid source parity: %+v %v",loaded,err)
 }
 state:=tacticsengine.StateV1{
  Version:1,MapID:"ash-vault",X:4,Y:5,Phase:"explore",TurnPhase:"party",
  Flags:map[string]bool{"vaultSlagVentSpent":false},
  Party:[]tacticsrules.PartyMember{{ID:"matthias",HP:7}},
  Enemies:[]tacticsrules.Enemy{},
 }
 raw,_:=json.Marshal(state)
 run:=tacticsapi.Run{
  ID:"owner-run",Owner:"alice",MapID:"ash-vault",WorldVersion:8,
  Status:"active",Map:tacticsapi.Map{ID:"ash-vault",Version:3,Grid:ashGrid},
  State:raw,
 }
 _,board,err:=tacticsengine.DecodeState(run)
 if err!=nil{t.Fatal(err)}
 move,err:=tacticsrules.DecideMove(board,"matthias",tacticsrules.Cell{X:5,Y:5})
 if err!=nil{t.Fatal(err)}
 staged,err:=tacticsengine.StageMoveEntry(context.Background(),run,move,c)
 if err!=nil{t.Fatal(err)}
 next,_,err:=tacticsengine.DecodeState(staged.Pending)
 if err!=nil||staged.TriggeredTrapID!="vault-slag-vent"||
  next.Party[0].HP!=6||!next.Flags["vaultSlagVentSpent"]||
  staged.Pending.WorldVersion!=8{
  t.Fatalf("expected uncommitted real trap effect: %+v %+v %v",next,staged,err)
 }
 wire,_:=json.Marshal(staged.Pending)
 if strings.Contains(string(wire),"vault-slag-vent")||
  strings.Contains(string(wire),"damage-party"){t.Fatal("private trap leaked")}
 run.Map.Version=4
 if _,err:=tacticsengine.StageMoveEntry(context.Background(),run,move,c);
  !errors.Is(err,tacticsengine.ErrInvalidContent){
  t.Fatalf("map revision drift accepted: %v",err)
 }
}
func TestFailClosedForUnknownAuthoredContent(t *testing.T){
 dir:=t.TempDir()
 c,err:=NewDirectory(dir);if err!=nil{t.Fatal(err)}
 defer c.Close()
 for _,id:=range []string{"absent","../secret","/tmp","a/b","bad.json",""}{
  if _,err:=c.LoadMapContent(context.Background(),id);!errors.Is(err,ErrCatalog){
   t.Errorf("invalid or missing map %q accepted: %v",id,err)
  }
 }
 tests:=map[string]func(map[string]any){
  "unknown-feature":func(m map[string]any){m["treasures"]=[]any{map[string]any{"id":"relic"}}},
  "unknown-effect":func(m map[string]any){
   m["traps"].([]any)[0].(map[string]any)["effects"]=[]any{map[string]any{"type":"grant-gold"}}
  },
  "wrong-revision":func(m map[string]any){m["mapRevision"]=0},
  "wrong-ID":func(m map[string]any){m["mapId"]="impostor"},
  "wrong-schema":func(m map[string]any){m["schemaVersion"]=2},
  "uneven-grid":func(m map[string]any){m["grid"]=[]string{"##","#..#"}},
  "wall-trap":func(m map[string]any){
   m["traps"].([]any)[0].(map[string]any)["cell"]=map[string]int{"x":0,"y":0}
  },
 }
 for name,edit:=range tests{
  m:=manifestFixture();edit(m)
  writeManifest(t,dir,"ash-vault",m)
  if _,err:=c.LoadMapContent(context.Background(),"ash-vault");
   !errors.Is(err,ErrCatalog){t.Errorf("bad %s accepted: %v",name,err)}
 }
}
func TestCatalogDirectoryIsolationAndSize(t *testing.T){
 if _,err:=NewDirectory("relative/maps");!errors.Is(err,ErrCatalog){
  t.Fatal("relative path accepted")
 }
 if _,err:=NewDirectory(filepath.Join(t.TempDir(),"missing"));!errors.Is(err,ErrCatalog){
  t.Fatal("nonexistent root accepted")
 }
 outside:=t.TempDir();inside:=t.TempDir()
 writeManifest(t,outside,"secret",manifestFixture())
 if err:=os.Symlink(filepath.Join(outside,"secret.json"),
  filepath.Join(inside,"secret.json"));err!=nil{t.Skip("symlinks unavailable")}
 c,err:=NewDirectory(inside);if err!=nil{t.Fatal(err)}
 defer c.Close()
 if _,err:=c.LoadMapContent(context.Background(),"secret");
  !errors.Is(err,ErrCatalog){t.Fatalf("symlink escaped private root: %v",err)}
 if err:=os.WriteFile(filepath.Join(inside,"large.json"),
  []byte(strings.Repeat(" ",maxManifestBytes+1)),0600);err!=nil{t.Fatal(err)}
 if _,err:=c.LoadMapContent(context.Background(),"large");
  !errors.Is(err,ErrCatalog){t.Fatalf("oversized manifest accepted: %v",err)}
}
