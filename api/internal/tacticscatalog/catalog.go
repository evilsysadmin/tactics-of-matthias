// Package tacticscatalog reads private, versioned world manifests.
// No world or fallback map is bundled with runtime.
package tacticscatalog

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "io"
 "os"
 "path/filepath"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsengine"
 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsrules"
)
var ErrCatalog=errors.New("invalid or unavailable Tactics authored catalog")
const maxManifestBytes=1024*1024

type Directory struct{root *os.Root}
var _ tacticsengine.ContentCatalog=(*Directory)(nil)
type manifest struct{
 SchemaVersion int `json:"schemaVersion"`
 MapID string `json:"mapId"`
 MapRevision int `json:"mapRevision"`
 Grid []string `json:"grid"`
 Traps []tacticsrules.AuthoredTrap `json:"traps"`
}

func NewDirectory(path string)(*Directory,error){
 if path==""||!filepath.IsAbs(path){return nil,ErrCatalog}
 root,err:=os.OpenRoot(path)
 if err!=nil{return nil,ErrCatalog}
 return &Directory{root:root},nil
}
func(d *Directory)Close()error{
 if d==nil||d.root==nil{return nil}
 return d.root.Close()
}
func validMapID(id string)bool{
 if len(id)==0||len(id)>128{return false}
 for i:=0;i<len(id);i++{
  c:=id[i]
  if !(c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'||c=='_'||c=='-'){return false}
 }
 return true
}
func(d *Directory)LoadMapContent(ctx context.Context,id string)(tacticsengine.PrivateMapContent,error){
 var zero tacticsengine.PrivateMapContent
 if d==nil||d.root==nil||!validMapID(id){return zero,ErrCatalog}
 if err:=ctx.Err();err!=nil{return zero,err}
 file,err:=d.root.Open(id+".json")
 if err!=nil{return zero,ErrCatalog}
 defer file.Close()
 data,err:=io.ReadAll(io.LimitReader(file,maxManifestBytes+1))
 if err!=nil||len(data)==0||len(data)>maxManifestBytes{return zero,ErrCatalog}
 if err:=ctx.Err();err!=nil{return zero,err}
 decoder:=json.NewDecoder(bytes.NewReader(data))
 decoder.DisallowUnknownFields()
 var value manifest
 if err:=decoder.Decode(&value);err!=nil{return zero,ErrCatalog}
 var trailing any
 if err:=decoder.Decode(&trailing);err!=io.EOF{return zero,ErrCatalog}
 if value.SchemaVersion!=1||value.MapRevision<1||value.MapRevision>1000000||
  value.MapID!=id||len(value.Grid)==0||len(value.Grid)>256||
  len(value.Grid[0])==0||len(value.Grid[0])>256||
  len(value.Grid)*len(value.Grid[0])>65536{return zero,ErrCatalog}
 if err:=tacticsrules.ValidateAuthoredTraps(value.Grid,value.Traps);err!=nil{
  return zero,ErrCatalog
 }
 return tacticsengine.PrivateMapContent{
  MapID:value.MapID,Revision:value.MapRevision,
  Grid:append([]string(nil),value.Grid...),Traps:value.Traps,
 },nil
}
