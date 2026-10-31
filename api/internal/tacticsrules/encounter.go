package tacticsrules

// Pure server-side engagement detection ported from
// chroniclesEnemyHasSightline and chroniclesEngagedEnemies. It decides which
// live enemies can SEE the exploration party, not who may attack or take a
// turn. Caller must supply validated authored enemy ranges from server data.
import "errors"

var ErrInvalidEncounter = errors.New("invalid authoritative Tactics encounter")

// Per-enemy authored visibility/attack reach. Nil engagement range is
// distinct from 0: Chronicles defaults it to attack reach. Tactics rejects
// invalid configured ranges instead of repairing corrupted fresh content.
type EnemyEngagement struct {
 AttackReach int
 EngageRange *int
}

func absDiff(a,b int)int{
 if a>=b{return a-b}
 return b-a
}

// EnemySightline uses the original Bresenham grid ray, including solid
// diagonal corner occlusion. It is deliberately DIFFERENT from ranged-attack
// line geometry: non-45-degree visible enemies may still be out of firing
// line. Only '#' blocks vision as in Chronicles.
func EnemySightline(grid []string,from,to Cell)bool{
 if !validGrid(grid)||from.X<0||to.X<0||from.Y<0||to.Y<0||
  from.Y>=len(grid)||to.Y>=len(grid)||
  from.X>=len(grid[0])||to.X>=len(grid[0]){return false}
 if grid[from.Y][from.X]=='#'||grid[to.Y][to.X]=='#'{return false}
 x,y:=from.X,from.Y
 dx,dy:=absDiff(to.X,x),absDiff(to.Y,y)
 sx,sy:=-1,-1
 if x<to.X{sx=1}
 if y<to.Y{sy=1}
 errorTerm:=dx-dy
 for x!=to.X||y!=to.Y{
  beforeX,beforeY:=x,y
  twice:=2*errorTerm
  if twice > -dy{errorTerm-=dy;x+=sx}
  if twice < dx{errorTerm+=dx;y+=sy}
  if (x!=to.X||y!=to.Y)&&grid[y][x]=='#'{return false}
  if x!=beforeX&&y!=beforeY&&
   grid[beforeY][x]=='#'&&grid[y][beforeX]=='#'{return false}
 }
 return true
}

// EngagedEnemies validates the authoritative exploration snapshot and each
// living enemy's server-authored reach. Dead enemies cannot enter initiative;
// no caller may send engagement stats through the HTTP Action schema.
//
// This returns a subset, in the original roster order. The encounter starter
// must still deploy the party and roll AGI+1d8 for THIS subset; this is not a
// complete resolved or persisted turn.
func EngagedEnemies(board Snapshot,authored map[string]EnemyEngagement)([]string,error){
 if board.Phase!="explore"||len(board.Initiative)!=0||
  board.TurnPhase=="enemy"||!traversable(tile(board,board.Anchor)){
  return nil,ErrInvalidEncounter
 }
 if err:=checkRoster(board);err!=nil{return nil,ErrInvalidEncounter}
 partyAlive:=false
 for _,member:=range board.Party{if member.HP>0{partyAlive=true;break}}
 if !partyAlive{return nil,ErrInvalidEncounter}
 selected:=make([]string,0,len(board.Enemies))
 for _,enemy:=range board.Enemies{
  if enemy.HP==0{continue}
  if !traversable(tile(board,enemy.Cell))||
   same(enemy.Cell,board.Anchor){return nil,ErrInvalidEncounter}
  profile,ok:=authored[enemy.ID]
  if !ok||profile.AttackReach<1||profile.AttackReach>16{
   return nil,ErrInvalidEncounter
  }
  effective:=profile.AttackReach
  if profile.EngageRange!=nil{
   if *profile.EngageRange<1||*profile.EngageRange>64{
    return nil,ErrInvalidEncounter
   }
   if *profile.EngageRange>effective{effective=*profile.EngageRange}
  }
  if absDiff(enemy.Cell.X,board.Anchor.X)+absDiff(enemy.Cell.Y,board.Anchor.Y)<=effective&&
   EnemySightline(board.Grid,enemy.Cell,board.Anchor){
   selected=append(selected,enemy.ID)
  }
 }
 return selected,nil
}
