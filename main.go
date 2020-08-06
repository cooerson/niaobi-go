package main //niaobi.org by 鸟神

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/beanstalkd/go-beanstalk"
	"github.com/didip/tollbooth"
	"github.com/go-xorm/xorm"
	"github.com/gogf/gf/os/gtimer"
	"github.com/iris-contrib/middleware/jwt"
	"github.com/iris-contrib/middleware/tollboothic"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/hero"
	"github.com/kataras/iris/v12/middleware/recover"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"

	"reqing.org/niaobi-go/config"
	"reqing.org/niaobi-go/controller"
	"reqing.org/niaobi-go/db"
	"reqing.org/niaobi-go/model"
)

var (
	rmbExr  float64
	pq      *xorm.Engine
	txLocks *db.TransLocks
)

type rmbExrRes struct {
	RmbExr float64 `json:"rmbExr"`
}

func main() {
	//-----初始化配置-----
	config.Load()
	rmbExr = config.Public.Exr.RmbExr
	//init locks
	txLocks = new(db.TransLocks)
	txLocks.Locks = make(map[string]bool)

	//-----同步数据库字段-----
	pq, _ = xorm.NewEngine("postgres", config.PQInfo)
	db.SyncDB(pq)

	//-----绑定model-----
	model.BindForm()

	//-----中间件-----
	jwt := jwt.New(jwt.Config{
		ContextKey:    config.JWTIrisIDKey,
		SigningMethod: jwt.SigningMethodHS256,
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(config.JWTSecret), nil
		},
		Expiration: true,
	})

	crs := func(ctx iris.Context) {
		if config.Public.Debug {
			ctx.Header("Access-Control-Allow-Origin", "*")
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Headers", "Access-Control-Allow-Origin,X-HTTP-Method-Override,Content-Type")
			ctx.Next()
		}
	}

	//限制请求次数每秒3次
	limiter := tollbooth.NewLimiter(3, nil)

	//-----定时任务-----
	startTimer()
	jobReqCheck()

	//-----路由-----
	app := iris.New()
	app.Use(recover.New())
	app.Use(dbHandler)
	app.Use(tollboothic.LimitHandler(limiter))

	//自定义路由规则
	app.Macros().Get("string").RegisterFunc("range", func(minLength, maxLength int) func(string) bool {
		return func(paramValue string) bool {
			return len(paramValue) >= minLength && len(paramValue) <= maxLength
		}
	})

	//人民币兑换鸟币汇率（1人民币=多少鸟币），仅供定价时参考，不可直接使用"RmbExr*人民币兑其他某个法币货币的汇率"来计算"其他某个法币货币兑鸟币的汇率"
	//在系统中正确的计算其他货币兑鸟币的汇率的方法：如港币，应该是港币当前的M2与鸟币创世时的港币M2的比值
	//注意，备用方案config.Public.Exr.RmbExr是管理员手动维护的数据，见config.toml
	app.Get("/exr/rmb", crs, func(ctx context.Context) {
		res := rmbExrRes{RmbExr: rmbExr}
		ctx.JSON(&res)
	})

	app.Post("/login", crs, hero.Handler(controller.Login))       //登录
	app.Post("/register", crs, hero.Handler(controller.Register)) //注册

	coin := app.Party("coin", crs)
	{
		coin.Use(jwt.Serve)
		{
			coin.Put("/updateProfile", hero.Handler(controller.UpdateProfile))             //修改个人资料
			coin.Put("/updatePwd", hero.Handler(controller.UpdatePwd))                     //修改密码
			coin.Put("/updateAvatar", picSizeHandler, controller.UpdateAvatar)             //修改头像
			coin.Get("/profile/{name:string range(1,20) else 400}", controller.GetProfile) //获取某用户资料
			coin.Get("/info", exrHandler, controller.GetMyActivity)                        //获取自己的动态
			//todo 找回密码
			//todo dashboard控制台，展示交易和鸟币等信息
		}
	}

	skill := app.Party("skill", crs)
	{
		skill.Use(jwt.Serve)
		{
			skill.Post("/new", picsSizeHandler, transHandler, hero.Handler(controller.NewSkill))    //添加技能
			skill.Put("/update", transHandler, hero.Handler(controller.UpdateSkill))                //更新技能
			skill.Put("/open/{id:uint64 else 400}/{open:bool}", transHandler, controller.OpenSkill) //上架或下架技能。open参数：1、t、true等表示上架技能，0、f、false等表示下架技能
			skill.Delete("/delete/{id:uint64 else 400}", transHandler, controller.DeleteSkill)      //删除技能，软删除
			//todo 搜索技能
		}
	}

	trans := app.Party("tx", crs)
	{
		trans.Use(jwt.Serve)
		{
			trans.Post("/pay", transHandler, hero.Handler(controller.NewPay))              //支付
			trans.Post("/req", hero.Handler(controller.NewReq))                            //发送兑现请求
			trans.Post("/repay", transHandler, hero.Handler(controller.NewRepay))          //接受兑现请求
			trans.Put("/reject/{req:uint64 else 400}", transHandler, controller.RejectReq) //拒绝兑现请求
			trans.Put("/uncash/{req:uint64 else 400}", controller.UnCash)                  //标记未兑现请求
			trans.Put("/redo/{req:uint64 else 400}", controller.Redo)                      //重新执行请求（拒绝后）
			trans.Put("/done/{req:uint64 else 400}", controller.Done)                      //标记完成交易
		}
	}

	img := app.Party("img", crs)
	{
		img.Use(jwt.Serve)
		{
			img.Get("/exist/{hash:string range(64,64) else 400}", controller.CheckPicHash) //检查图片是否存在
			img.Post("/new", picSizeHandler, controller.NewPic)                            //上传图片
		}
	}

	//认证失败
	app.OnErrorCode(iris.StatusUnauthorized, func(ctx context.Context) {
		var e = new(model.CommonError)
		e.FinalError(ctx, iris.StatusUnauthorized, config.Public.Err.E1010)
	})
	//路由错误
	app.OnErrorCode(iris.StatusNotFound, func(ctx context.Context) {
		var e = new(model.CommonError)
		e.FinalError(ctx, iris.StatusNotFound, config.Public.Err.E1011)
	})
	//上传文件过大
	app.OnErrorCode(iris.StatusRequestEntityTooLarge, func(ctx context.Context) {
		var e = new(model.CommonError)
		e.FinalError(ctx, iris.StatusRequestEntityTooLarge, config.Public.Err.E1014)
	})

	app.HandleDir("/", "./view/api_test.html")

	app.Run(iris.Addr("localhost:3001"))
}

//-----中间件-----
func dbHandler(ctx context.Context) {
	ctx.Values().Set(config.PQIrisIDKey, pq)
	ctx.Next()
}

//检查头像大小
func exrHandler(ctx context.Context) {
	ctx.Values().Set(config.RMBExrIrisKey, rmbExr)
	ctx.Next()
}

//交易锁
func transHandler(ctx context.Context) {
	ctx.Values().Set(config.TxLocksIrisKey, txLocks)
	ctx.Next()
}

//检查单张图片大小
func picSizeHandler(ctx context.Context) {
	if ctx.GetContentLength() > config.Public.Pic.MaxUploadPic {
		ctx.StatusCode(iris.StatusRequestEntityTooLarge)
		return
	}
	ctx.Next()
}

//检查多图上传的大小
func picsSizeHandler(ctx context.Context) {
	if ctx.GetContentLength() > config.Public.Pic.MaxUploadPics {
		ctx.StatusCode(iris.StatusRequestEntityTooLarge)
		return
	}
	ctx.Next()
}

//-----定时任务-----
func startTimer() {
	c := cron.New()

	//启动的时候执行一次，以后每隔5小时执行一次
	job1 := jobRMBExr{}
	job1.Run()
	c.AddJob("@every 5h", job1)
	// job2 := jobReqCheck{}
	// job2.Run()
	// c.AddJob("@every 5s", job2)

	c.Start()
}

type jobRMBExr struct {
}

func (jobRMBExr) Run() {
	fmt.Println("[timer]Running RmbExrJob...")

	//注意：不同国家需要使用各自国家的M2
	//每隔一段时间，自动更新人民币m2，此处数据来自新浪财经。官方来源 http://www.pbc.gov.cn/diaochatongjisi/116219/116319/3750274/3750284/index.html
	// resp, err := http.Get("http://money.finance.sina.com.cn/mac/api/jsonp.php/SINAREMOTECALLCALLBACK/MacPage_Service.get_pagedata?cate=fininfo&event=1&from=0&num=1&condition")

	// /**
	// http响应失败时，resp变量将为 空值，而 err变量将是 非空值。
	// 当得到一个重定向的错误时，两个变量都将是 非空值。这意味着最后依然会内存泄露。
	// 防止内存泄漏的正确写法:
	// */
	// if resp != nil {
	// 	defer resp.Body.Close()
	// }
	// if err != nil {
	// 	rmbExr = config.Public.Exr.RmbExr
	// 	return
	// }

	// //流式读取数据
	// str := bytes.NewBufferString("")
	// err = util.ReadReader(resp.Body, func(block []byte) {
	// 	str.WriteString(exbytes.ToString(block))
	// })
	// if err != nil {
	// 	rmbExr = config.Public.Exr.RmbExr
	// 	return
	// }

	// //格式整理
	// s := str.String()
	// idx := strings.LastIndex(s, "data:")
	// if idx < 1 {
	// 	rmbExr = config.Public.Exr.RmbExr
	// 	return
	// }
	// newstr := s[idx:]
	// newstr = exstrings.SubString(newstr, 6, len(newstr)-10)
	// util.LogDebug(newstr)

	// var arr []string
	// json.Unmarshal([]byte(newstr), &arr)
	// if err != nil {
	// 	rmbExr = config.Public.Exr.RmbExr
	// 	return
	// }
	// m2Now, err := strconv.ParseFloat(arr[1], 64)
	// if err != nil {
	// 	rmbExr = config.Public.Exr.RmbExr
	// 	return
	// }
	// rmbExr = m2Now / config.Public.Exr.RmbM2Init
	// if rmbExr < config.Public.Exr.RmbExr {
	rmbExr = config.Public.Exr.RmbExr
	// }
}

//超时未接受的兑现请求处理
func jobReqCheck() {
	//每隔20毫秒循环一次，一分钟可以查询3000条数据，记录读取超时时间为200毫秒
	interval := 20 * time.Millisecond
	timeOut := 200 * time.Millisecond
	gtimer.Add(interval, func() {
		conn, _ := beanstalk.Dial("tcp", config.BeanstalkURI)
		tubeSet := beanstalk.NewTubeSet(conn, config.BeanstalkTubeReq)
		jobID, body, err := tubeSet.Reserve(timeOut)
		if err != nil {
			defer conn.Close()
			return
		}

		req := db.Req{}
		err = json.Unmarshal(body, &req)
		if err != nil {
			conn.Delete(jobID)
			defer conn.Close()
			return
		}

		reqNow := db.Req{}
		pq.ID(req.ID).Cols("state").Get(&reqNow)
		if reqNow.State != 10 {
			conn.Delete(jobID)
			defer conn.Close()
			return
		}

		//数据库
		news1 := db.News{Owner: req.Bearer, Desc: config.Public.Req.B22, Amount: int64(req.Amount), Buddy: req.Issuer, Table: config.NewsTableReq, SourceID: req.ID}
		news2 := db.News{Owner: req.Issuer, Desc: config.Public.Req.I22, Amount: int64(req.Amount), Buddy: req.Bearer, Table: config.NewsTableReq, SourceID: req.ID}
		pq.Insert(&news1, &news2)
		pq.ID(req.ID).Update(&db.Req{State: 22})

		conn.Delete(jobID)
		defer conn.Close()
	})
}
