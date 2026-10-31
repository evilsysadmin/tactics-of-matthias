package tacticsmongo

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "net"
 "net/url"
 "os"
 "sync"
 "testing"
 "time"

 "github.com/evilsysadmin/tactics-of-matthias/api/internal/tacticsapi"
 "go.mongodb.org/mongo-driver/v2/mongo"
 "go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestNewRejectsAbsentDatabase(t *testing.T){
 if _,err:=New(context.Background(),nil,time.Second);err==nil{t.Fatal("nil database accepted")}
}
func dbFixture(t *testing.T)(*Store,*mongo.Database,string){
 t.Helper()
 uri:=os.Getenv("TACTICS_MONGO_INTEGRATION_URI")
 if uri==""{t.Skip("ephemeral Mongo CI not configured")}
 u,err:=url.Parse(uri);if err!=nil{t.Fatal(err)}
 host,port,err:=net.SplitHostPort(u.Host)
 if err!=nil || u.Scheme!="mongodb" || port!="27017" || (host!="localhost" && host!="127.0.0.1"){
  t.Fatal("refuse remote database in tests")
 }
 client,err:=mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(3*time.Second))
 if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ =client.Disconnect(context.Background())})
 db:=client.Database("shadowops_tactics_ci")
 store,err:=New(context.Background(),db,5*time.Second)
 if err!=nil{t.Fatal(err)}
 random:=make([]byte,16);if _,err:=rand.Read(random);err!=nil{t.Fatal(err)}
 id:="ci-"+hex.EncodeToString(random)
 t.Cleanup(func(){
  ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
  _,_=db.Collection(CollectionName).DeleteOne(ctx,map[string]string{"_id":id})
 })
 return store,db,id
}
func seed(t *testing.T,db *mongo.Database,id string)tacticsapi.Run{
 t.Helper()
 run:=tacticsapi.Run{ID:id,Owner:"alice-uuid",MapID:"crypt",WorldVersion:0,Status:"active",
  Map:tacticsapi.Map{ID:"crypt",Title:"Crypt",Grid:[]string{"####","#..#","####"}},
  State:json.RawMessage(`{"x":1,"y":1}`),
 }
 _,err:=db.Collection(CollectionName).InsertOne(context.Background(),document{
  ID:run.ID,Owner:run.Owner,MapID:run.MapID,WorldVersion:run.WorldVersion,Status:run.Status,
  Map:run.Map,StateJSON:string(run.State),
 })
 if err!=nil{t.Fatal(err)}
 return run
}
func TestOwnerScopedLoadAndCAS(t *testing.T){
 store,db,id:=dbFixture(t);initial:=seed(t,db,id)
 _,found,err:=store.Load(context.Background(),"bob-uuid",id)
 if found || err!=nil{t.Fatalf("cross-owner read: %v %v",found,err)}
 got,found,err:=store.Load(context.Background(),initial.Owner,id)
 if !found || err!=nil || string(got.State)!=string(initial.State){t.Fatalf("read failed: %+v %v",got,err)}
 next:=got;next.WorldVersion++;next.State=json.RawMessage(`{"x":2}`)
 ok,err:=store.CompareAndSwap(context.Background(),"bob-uuid",id,0,next)
 if ok || err==nil{t.Fatalf("wrong owner accepted %v %v",ok,err)}
 ok,err=store.CompareAndSwap(context.Background(),initial.Owner,id,0,next)
 if !ok || err!=nil{t.Fatalf("CAS failed %v %v",ok,err)}
 ok,err=store.CompareAndSwap(context.Background(),initial.Owner,id,0,next)
 if ok || err!=nil{t.Fatalf("replayed action accepted %v %v",ok,err)}
 got,found,err=store.Load(context.Background(),initial.Owner,id)
 if err!=nil || !found || got.WorldVersion!=1 || string(got.State)!=string(next.State){
  t.Fatalf("wrong persisted run: %+v %v",got,err)
 }
 terminal:=got;terminal.Status="defeated";terminal.WorldVersion++
 ok,err=store.CompareAndSwap(context.Background(),initial.Owner,id,1,terminal)
 if !ok || err!=nil{t.Fatalf("could not finish run %v %v",ok,err)}
 revived:=terminal;revived.Status="active";revived.WorldVersion++
 ok,err=store.CompareAndSwap(context.Background(),initial.Owner,id,2,revived)
 if ok || err!=nil{t.Fatalf("revived terminal run %v %v",ok,err)}
}
func TestConcurrentCASAcrossIndependentStores(t *testing.T){
 store,db,id:=dbFixture(t);initial:=seed(t,db,id)
 other,err:=New(context.Background(),db,5*time.Second);if err!=nil{t.Fatal(err)}
 next:=initial;next.WorldVersion=1;next.State=json.RawMessage(`{"turn":1}`)
 var wg sync.WaitGroup;var mu sync.Mutex;winners:=0
 for i:=0;i<20;i++{
  wg.Add(1)
  go func(i int){
   defer wg.Done();s:=store;if i%2==1{s=other}
   ok,err:=s.CompareAndSwap(context.Background(),initial.Owner,id,0,next)
   if err!=nil{t.Errorf("CAS: %v",err);return}
   if ok{mu.Lock();winners++;mu.Unlock()}
  }(i)
 }
 wg.Wait();if winners!=1{t.Fatalf("expected single winner got %d",winners)}
 got,found,err:=store.Load(context.Background(),initial.Owner,id)
 if !found || err!=nil || got.WorldVersion!=1{t.Fatalf("invalid final version %+v %v",got,err)}
}
func TestInvalidWritesAndForeignDatabases(t *testing.T){
 store,db,id:=dbFixture(t);initial:=seed(t,db,id)
 for _,name:=range []string{"chronicles","chess_study","shadowops_identity_staging","shadowops_tactics"}{
  if _,err:=New(context.Background(),db.Client().Database(name),time.Second);err==nil{
   t.Fatalf("foreign database %s accepted",name)
  }
 }
 for _,next:=range []tacticsapi.Run{
  {ID:id,Owner:"bob-uuid",MapID:"crypt",WorldVersion:1,Status:"active",Map:initial.Map,State:initial.State},
  {ID:"wrong",Owner:initial.Owner,MapID:"crypt",WorldVersion:1,Status:"active",Map:initial.Map,State:initial.State},
  {ID:id,Owner:initial.Owner,MapID:"crypt",WorldVersion:5,Status:"active",Map:initial.Map,State:initial.State},
  {ID:id,Owner:initial.Owner,MapID:"crypt",WorldVersion:1,Status:"active",Map:initial.Map,State:json.RawMessage("{")},
 }{
  ok,err:=store.CompareAndSwap(context.Background(),initial.Owner,id,0,next)
  if ok || err==nil{t.Fatalf("invalid write accepted %v %v",ok,err)}
 }
}
