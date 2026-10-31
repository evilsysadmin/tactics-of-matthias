package tacticsrules

// Deterministic, PURE port of Chronicles' party deployment footprint.
// The server must supply the map, enemy roster and authored occupied content.
// This planner never persists a run, rolls initiative or moves an enemy.
import (
 "errors"
 "math"
 "sort"
 "strings"
 "fmt"
)
var ErrInvalidDeployment=errors.New("invalid authoritative combat deployment")
var ErrNoFormation=errors.New("no reachable connected formation for living party")
const maxDeployRadius=6
const maxDeployCandidates=24

var partyGridOrder=[]string{"matthias","rook","bishop","knight"}
var roleTargets=map[string]struct{forward,left float64}{
 "matthias":{0.7,0.7},
 "rook":{0.7,-0.7},
 "bishop":{-0.7,0.7},
 "knight":{-0.7,-0.7},
}
type deployCandidate struct{
 Cell
 distance int
 forward,left float64
}
func manhattan(a,b Cell)int{return absDiff(a.X,b.X)+absDiff(a.Y,b.Y)}
func cellKey(c Cell)string{return fmt.Sprintf("%d:%d",c.X,c.Y)}
func deploymentBasis(direction int)(forward,left Cell){
 forward=Cell{X:directions[direction].dx,Y:directions[direction].dy}
 return forward,Cell{X:forward.Y,Y:-forward.X}
}
func validDeployment(board Snapshot,direction int,blocked []Cell)(
 []string,map[Cell]bool,map[Cell]bool,error,
){
 if direction<0||direction>3||board.Phase!="explore"||
  board.TurnPhase=="enemy"||len(board.Initiative)!=0||
  !validGrid(board.Grid)||len(board.Party)==0||len(board.Party)>4||
  len(board.Enemies)>256||len(blocked)>1024||
  !traversable(tile(board,board.Anchor))||
  len(board.PartyPositions)!=0{return nil,nil,nil,ErrInvalidDeployment}
 living:=map[string]bool{}
 known:=map[string]bool{}
 for _,member:=range board.Party{
  if member.ID==""||member.HP<0||known[member.ID]{
   return nil,nil,nil,ErrInvalidDeployment
  }
  if _,ok:=roleTargets[member.ID];!ok{return nil,nil,nil,ErrInvalidDeployment}
  known[member.ID]=true
  living[member.ID]=member.HP>0
 }
 alive:=[]string{}
 for _,id:=range partyGridOrder{if living[id]{alive=append(alive,id)}}
 if len(alive)==0{return nil,nil,nil,ErrNoFormation}
 enemies:=make(map[Cell]bool)
 enemyIDs:=map[string]bool{}
 for _,enemy:=range board.Enemies{
  if enemy.ID==""||enemy.HP<0||enemyIDs[enemy.ID]{
   return nil,nil,nil,ErrInvalidDeployment
  }
  enemyIDs[enemy.ID]=true
  if enemy.HP>0{
   if !traversable(tile(board,enemy.Cell))||enemies[enemy.Cell]{
    return nil,nil,nil,ErrInvalidDeployment
   }
   enemies[enemy.Cell]=true
  }
 }
 if enemies[board.Anchor]{return nil,nil,nil,ErrInvalidDeployment}
 occupied:=make(map[Cell]bool,len(blocked)+len(enemies))
 for cell:=range enemies{occupied[cell]=true}
 for _,cell:=range blocked{
  if !traversable(tile(board,cell)){return nil,nil,nil,ErrInvalidDeployment}
  occupied[cell]=true
 }
 return alive,occupied,enemies,nil
}
func deployCandidates(board Snapshot,occupied,enemies map[Cell]bool,direction int)[]deployCandidate{
 forward,left:=deploymentBasis(direction)
 origin:=board.Anchor
 seen:=map[Cell]bool{origin:true}
 frontier:=[]Cell{origin}
 candidates:=[]deployCandidate{}
 // Matches Chronicles: BFS can route around an authored visible object but
 // cannot go THROUGH an enemy or a wall. Blocking exits X is intentional
 // to match the server's movement rules (stricter than the legacy JS).
 steps:=[]Cell{{X:1},{X:-1},{Y:1},{Y:-1}}
 for radius:=0;radius<=maxDeployRadius&&len(frontier)>0;radius++{
  next:=[]Cell{}
  for _,cell:=range frontier{
   if !occupied[cell]&&traversable(tile(board,cell)){
    dx,dy:=cell.X-origin.X,cell.Y-origin.Y
    candidates=append(candidates,deployCandidate{
     Cell:cell,distance:radius,
     forward:float64(dx*forward.X+dy*forward.Y),
     left:float64(dx*left.X+dy*left.Y),
    })
   }
   for _,step:=range steps{
    nearby:=Cell{X:cell.X+step.X,Y:cell.Y+step.Y}
    if seen[nearby]{continue}
    seen[nearby]=true
    if !traversable(tile(board,nearby))||enemies[nearby]{continue}
    next=append(next,nearby)
   }
  }
  frontier=next
 }
 sort.SliceStable(candidates,func(i,j int)bool{
  a,b:=candidates[i],candidates[j]
  if a.distance!=b.distance{return a.distance<b.distance}
  if a.forward!=b.forward{return a.forward>b.forward}
  if a.left!=b.left{return a.left>b.left}
  if a.Y!=b.Y{return a.Y<b.Y}
  return a.X<b.X
 })
 if len(candidates)>maxDeployCandidates{candidates=candidates[:maxDeployCandidates]}
 return candidates
}
func connectedCluster(cells []deployCandidate)bool{
 if len(cells)==0{return false}
 remaining:=make(map[Cell]bool,len(cells))
 for _,cell:=range cells{remaining[cell.Cell]=true}
 queue:=[]Cell{cells[0].Cell}
 delete(remaining,cells[0].Cell)
 for head:=0;head<len(queue);head++{
  current:=queue[head]
  for _,step:=range []Cell{{X:1},{X:-1},{Y:1},{Y:-1}}{
   neighbor:=Cell{X:current.X+step.X,Y:current.Y+step.Y}
   if remaining[neighbor]{
    delete(remaining,neighbor)
    queue=append(queue,neighbor)
   }
  }
 }
 return len(remaining)==0
}
func clusterTie(cells []deployCandidate)string{
 keys:=make([]string,0,len(cells))
 for _,c:=range cells{keys=append(keys,cellKey(c.Cell))}
 sort.Strings(keys)
 return strings.Join(keys,"|")
}
func clusterScore(cells []deployCandidate,origin Cell)float64{
 minX,maxX,minY,maxY:=cells[0].X,cells[0].X,cells[0].Y,cells[0].Y
 sum,maxDist,pairwise:=0,0,0
 containsOrigin:=false
 for i,a:=range cells{
  if a.X<minX{minX=a.X};if a.X>maxX{maxX=a.X}
  if a.Y<minY{minY=a.Y};if a.Y>maxY{maxY=a.Y}
  dist:=a.distance
  if dist>maxDist{maxDist=dist}
  sum+=dist
  if a.Cell==origin{containsOrigin=true}
  for j:=i+1;j<len(cells);j++{pairwise+=manhattan(a.Cell,cells[j].Cell)}
 }
 spanX,spanY:=maxX-minX,maxY-minY
 penalty:=0
 if !containsOrigin{penalty=80}
 return float64(penalty+maxDist*30+sum*4+
  (spanX+1)*(spanY+1)*5+(spanX+spanY)*3+pairwise)
}
func bestConnectedCluster(candidates []deployCandidate,size int,origin Cell)[]deployCandidate{
 if size<=0||len(candidates)<size{return nil}
 var best []deployCandidate
 bestScore,bestTie:=math.Inf(1),""
 picked:=make([]deployCandidate,0,size)
 var search func(int)
 search=func(start int){
  if len(picked)==size{
   if !connectedCluster(picked){return}
   score,tie:=clusterScore(picked,origin),clusterTie(picked)
   if score<bestScore||(score==bestScore&&(best==nil||tie<bestTie)){
    best=append([]deployCandidate(nil),picked...)
    bestScore,bestTie=score,tie
   }
   return
  }
  need:=size-len(picked)
  for index:=start;index<=len(candidates)-need;index++{
   picked=append(picked,candidates[index])
   search(index+1)
   picked=picked[:len(picked)-1]
  }
 }
 search(0)
 return best
}
func assignmentCost(id string,cell deployCandidate)float64{
 target:=roleTargets[id]
 return math.Abs(cell.forward-target.forward)*3+
  math.Abs(cell.left-target.left)*2+
  float64(cell.distance)*0.15
}
func bestAssignment(ids []string,cells []deployCandidate)map[string]Cell{
 used:=make([]bool,len(cells))
 current:=make(map[string]Cell,len(ids))
 best:=map[string]Cell(nil)
 bestScore,bestTie:=math.Inf(1),""
 var search func(int,float64,[]string)
 search=func(index int,cost float64,ties []string){
  if index==len(ids){
   tie:=strings.Join(ties,"|")
   if cost<bestScore||(cost==bestScore&&(best==nil||tie<bestTie)){
    best=make(map[string]Cell,len(current))
    for id,cell:=range current{best[id]=cell}
    bestScore,bestTie=cost,tie
   }
   return
  }
  id:=ids[index]
  for i,cell:=range cells{
   if used[i]{continue}
   used[i]=true
   current[id]=cell.Cell
   search(index+1,cost+assignmentCost(id,cell),append(ties,id+":"+cellKey(cell.Cell)))
   delete(current,id)
   used[i]=false
  }
 }
 search(0,0,nil)
 return best
}

// PlanPartyDeployment returns ONLY living party positions. It runs the
// Chronicles 6-step/24-candidate BFS, connected-group and weighted role
// assignment. Unknown/dead members cannot materialize during deployment.
//
// No valid reachable connected footprint => error; never teleport to cells
// across a wall, overlap a unit, or fake a missing formation as success.
func PlanPartyDeployment(board Snapshot,direction int,authoredBlocked []Cell)(
 map[string]Cell,error,
){
 alive,occupied,enemies,err:=validDeployment(board,direction,authoredBlocked)
 if err!=nil{return nil,err}
 candidates:=deployCandidates(board,occupied,enemies,direction)
 var cluster []deployCandidate
 slots:=len(partyGridOrder)
 for cluster==nil&&slots>=len(alive){
  cluster=bestConnectedCluster(candidates,slots,board.Anchor)
  slots--
 }
 if cluster==nil{
  cluster=bestConnectedCluster(candidates,len(alive),board.Anchor)
 }
 if cluster==nil{return nil,ErrNoFormation}
 // Normal 4-slot deployments are exactly the Chronicles rule. When a
 // cramped map offers fewer than four contiguous cells, include EVERY
 // living character instead of assigning a fallback cell to a dead role.
 ids:=make([]string,0,len(cluster))
 for _,id:=range partyGridOrder{
  if len(ids)==len(cluster){break}
  ids=append(ids,id)
 }
 for _,id:=range alive{
  present:=false
  for _,slot:=range ids{if slot==id{present=true;break}}
  if !present{
   for i:=len(ids)-1;i>=0;i--{
    if !isLivingID(ids[i],alive){ids[i]=id;break}
   }
  }
 }
 assigned:=bestAssignment(ids,cluster)
 output:=make(map[string]Cell,len(alive))
 for _,id:=range alive{
  cell,ok:=assigned[id]
  if !ok{return nil,ErrNoFormation}
  output[id]=cell
 }
 return output,nil
}
func isLivingID(id string,alive []string)bool{
 for _,live:=range alive{if live==id{return true}}
 return false
}
