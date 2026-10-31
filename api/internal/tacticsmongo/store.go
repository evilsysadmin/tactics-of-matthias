// Package tacticsmongo isolates Tactics persistence from all other ShadowOps
// games. Identity, run creation and tactical rules live in separate services.
package tacticsmongo

import (
 "context"
 "encoding/json"
 "errors"
 "time"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "go.mongodb.org/mongo-driver/v2/bson"
 "go.mongodb.org/mongo-driver/v2/mongo"
 "go.mongodb.org/mongo-driver/v2/mongo/options"
 "go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

const CollectionName = "tactics_runs"
var ErrUnavailable=errors.New("Tactics Mongo unavailable")
var ErrInvalidRun=errors.New("invalid Tactics saved run")
var allowedDatabases=map[string]bool{
 "shadowops_tactics_ci":true,
 "shadowops_tactics_staging":true,
 "shadowops_tactics_production":true,
}
type Store struct {
 col *mongo.Collection
 timeout time.Duration
}
var _ tacticsapi.Store=(*Store)(nil)

type document struct {
 ID string `bson:"_id"`
 Owner string `bson:"owner"`
 MapID string `bson:"currentMapId"`
 WorldVersion int64 `bson:"worldVersion"`
 Status string `bson:"runStatus"`
 Map tacticsapi.Map `bson:"map"`
 StateJSON string `bson:"stateJson"`
 UpdatedAt time.Time `bson:"updatedAt"`
}

// New requires a caller-owned connection to a named Tactics-only database.
// It fails startup if the compound owner index cannot be installed.
func New(ctx context.Context,db *mongo.Database,timeout time.Duration)(*Store,error){
 if db==nil || !allowedDatabases[db.Name()] {return nil,errors.New("requires isolated Tactics database")}
 if timeout<=0 {timeout=3*time.Second}
 col:=db.Collection(CollectionName,options.Collection().SetWriteConcern(writeconcern.Majority()))
 initCtx,cancel:=context.WithTimeout(ctx,timeout)
 defer cancel()
 _,err:=col.Indexes().CreateOne(initCtx,mongo.IndexModel{
  Keys:bson.D{{Key:"owner",Value:1},{Key:"_id",Value:1}},
  Options:options.Index().SetUnique(true),
 })
 if err!=nil{return nil,ErrUnavailable}
 return &Store{col:col,timeout:timeout},nil
}

func (s *Store) Load(ctx context.Context,owner,id string)(tacticsapi.Run,bool,error){
 if s==nil || s.col==nil{return tacticsapi.Run{},false,ErrUnavailable}
 if owner=="" || id=="" {return tacticsapi.Run{},false,nil}
 c,cancel:=context.WithTimeout(ctx,s.timeout);defer cancel()
 var doc document
 err:=s.col.FindOne(c,bson.D{{Key:"_id",Value:id},{Key:"owner",Value:owner}}).Decode(&doc)
 if errors.Is(err,mongo.ErrNoDocuments){return tacticsapi.Run{},false,nil}
 if err!=nil{return tacticsapi.Run{},false,ErrUnavailable}
 state:=json.RawMessage(doc.StateJSON)
 if len(state)==0 || !json.Valid(state){return tacticsapi.Run{},false,ErrInvalidRun}
 return tacticsapi.Run{
  ID:doc.ID,Owner:doc.Owner,MapID:doc.MapID,WorldVersion:doc.WorldVersion,
  Status:doc.Status,Map:doc.Map,State:state,
 },true,nil
}

// CompareAndSwap enforces owner + id + version + active in one Mongo UpdateOne.
// There is no upsert or local fallback. Multiple workers can race safely.
func (s *Store) CompareAndSwap(ctx context.Context,owner,id string,expected int64,next tacticsapi.Run)(bool,error){
 if s==nil || s.col==nil{return false,ErrUnavailable}
 if owner=="" || id=="" || expected<0 || expected==int64(^uint64(0)>>1) ||
  next.Owner!=owner || next.ID!=id || next.WorldVersion!=expected+1 ||
  next.Map.ID!=next.MapID || next.MapID=="" ||
  (next.Status!="active" && next.Status!="completed" && next.Status!="defeated") ||
  len(next.State)==0 || len(next.State)>256*1024 || !json.Valid(next.State){
  return false,ErrInvalidRun
 }
 c,cancel:=context.WithTimeout(ctx,s.timeout);defer cancel()
 result,err:=s.col.UpdateOne(c,
  bson.D{{Key:"_id",Value:id},{Key:"owner",Value:owner},{Key:"worldVersion",Value:expected},{Key:"runStatus",Value:"active"}},
  bson.D{
   {Key:"$set",Value:bson.D{
    {Key:"currentMapId",Value:next.MapID},{Key:"runStatus",Value:next.Status},
    {Key:"map",Value:next.Map},{Key:"stateJson",Value:string(next.State)},
    {Key:"updatedAt",Value:time.Now().UTC()},
   }},
   {Key:"$inc",Value:bson.D{{Key:"worldVersion",Value:int64(1)}}},
  },
 )
 if err!=nil{return false,ErrUnavailable}
 return result.ModifiedCount==1,nil
}
