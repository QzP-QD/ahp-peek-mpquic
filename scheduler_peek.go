package quic

import (
	"bufio"
	"fmt"
	"github.com/gammazero/deque"
	"github.com/lucas-clemente/quic-go/ackhandler"
	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/wire"
	"gonum.org/v1/gonum/mat"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const dimension = 6

type Teststatic struct {
	username string
}

var t *Teststatic

func init() {
	t = new(Teststatic)
}

//方便链式操作
func WithConfig() *Teststatic {
	return t.WithConfig()
}
func (c *Teststatic) WithConfig() *Teststatic {
	return c
}

//写
func (c *Teststatic) SetConfig(name string) {
	c.username = name
}

//读
func (c *Teststatic) GetConfig() string {
	return c.username
}

type PeekID uint8

type PeekMemory struct {
	*deque.Deque
}

type PeekEvent struct {
	PathID       protocol.PathID
	peekID       PeekID
	PacketNumber protocol.PacketNumber
	decideTime   time.Time
	PacketLength protocol.ByteCount
	state        [dimension]float64
	Tref         int64
}

func PeekNewMemory() *PeekMemory {
	return &PeekMemory{
		Deque: &deque.Deque{},
	}
}

func PeekNewEvent(pathID protocol.PathID, peekID PeekID, packetnumber protocol.PacketNumber, state [dimension]float64, pktlen protocol.ByteCount, Tref int64, decideTime time.Time) *PeekEvent {
	return &PeekEvent{
		PathID:       pathID,
		peekID:       peekID,
		PacketNumber: packetnumber,
		state:        state,
		Tref:         Tref,
		decideTime:   decideTime,
		PacketLength: pktlen,
	}
}

type peekstrategy struct {
	peekID       PeekID
	strategyname string
	pdeploy      float64
	qdeploy      float64
	theta        *mat.Dense
}

var peek0 = &peekstrategy{
	peekID:       0,
	strategyname: "tx", // minRTT：在当前minRTT路径传输
}

var peek1 = &peekstrategy{
	peekID:       1,
	strategyname: "wt", // 等待快速路径
}

const recordNum = 2000

// 记录阶段的执行次数统计
var recordnumtx = recordNum
var recordnumwt = recordNum

// state代表目前的状态
const initstate = "initstate"
const recordtx = "recordtx"
const recordwt = "recordwt"
const learning = "learning"
const deployment = "deployment"

var state string = initstate

const txpath string = "/home/mininet/peekaboo/output/outputtx.txt"
const wtpath string = "/home/mininet/peekaboo/output/outputwt.txt"
const inputpath string = "/home/mininet/peekaboo/input/input.txt"
const deployedtx string = "/home/mininet/peekaboo/output/deployedtx.txt"
const deployedwt string = "/home/mininet/peekaboo/output/deployedwt.txt"
const deployedtxbak string = "/home/mininet/peekaboo/output/deployedtxbak.txt"
const deployedwtbak string = "/home/mininet/peekaboo/output/deployedwtbak.txt"
const checkqpth string = "/home/mininet/peekaboo/input/checkq.txt"

var flagfile string = "/home/mininet/peekaboo/flag1"

const pathinfo string = "/home/mininet/peekaboo/pathinfo.txt"

const cleaninputpath string = "/home/mininet/peekaboo/shellscript/input.sh"
const cleancheckqpth string = "/home/mininet/peekaboo/shellscript/checkq.sh"

const cleantxpath string = "/home/mininet/peekaboo/shellscript/outputtx.sh"
const cleanwtpath string = "/home/mininet/peekaboo/shellscript/outputwt.sh"

const cleandeployedtx string = "/home/mininet/peekaboo/shellscript/deployedtx.sh"
const cleandeployedwt string = "/home/mininet/peekaboo/shellscript/deployedwt.sh"

var featureCache [dimension]float64
var TrefCache int64

const wt = "wt"
const tx = "tx"
const normal = "normal"

var mediaparams = map[string]float64{
	"rtt":  47.955748,
	"loss": 11.4959,
	"bth":  40.5483,
}
var sessparams = map[string]float64{
	"rtt":  47.9557,
	"loss": 40.5483,
	"bth":  11.4959,
}
var backparams = map[string]float64{
	"rtt":  11.4959,
	"loss": 47.9557,
	"bth":  40.5484,
}

//var videoparams map[string]float64
//var audioparams map[string]float64
//var fileparams map[string]float64

type wtcache struct {
	feature    [dimension]float64
	decideTime time.Time
	Tref       int64
}

func WtNewCache(feature [dimension]float64, Tref int64) *wtcache {
	return &wtcache{
		feature:    feature,
		Tref:       Tref,
		decideTime: time.Now(),
	}
}

var wtcaches deque.Deque = deque.Deque{}

// peekabood调度器
/*
	path: 调度器选择的路径
	flag: 当前做的决策（tx，wt，normal）
*/
func (sch *scheduler) selectPeekaboo(s *session, hasRetransmission bool, hasStreamRetransmission bool, fromPth *path) (*path, string) {
	var selectedpath *path
	var flag string = normal
	if state == initstate {
		fmt.Println("Init Peek>>>>")
		// 创建新的输出文件
		var f *os.File
		f, _ = os.Create(txpath)
		f.Close()

		f, _ = os.Create(wtpath)
		f.Close()

		f, _ = os.Create(deployedtx)
		f.Close()

		f, _ = os.Create(deployedwt)
		f.Close()

		f, _ = os.Create(deployedtxbak)
		f.Close()
		f, _ = os.Create(deployedwtbak)
		f.Close()
		f, _ = os.Create(checkqpth)
		f.Close()

		// 状态置为 recordtx
		state = recordwt
	}
	// TODO:筛选路径
	bestPath, secondPath := sch.selectPathAHP(s, hasRetransmission, hasStreamRetransmission, fromPth)

	// 当最快路径不存在时的判断逻辑
	if bestPath == nil {
		if secondPath != nil {
			return secondPath, flag
		}
		if s.paths[protocol.InitialPathID].SendingAllowed() || hasRetransmission {
			return s.paths[protocol.InitialPathID], flag
		} else {
			return nil, flag
		}
	}
	// 最快路径存在且可用时，直接在最快路径上发送
	if bestPath.SendingAllowed() {
		sch.waiting = 0
		return bestPath, flag
	}
	if secondPath == nil {
		if s.paths[protocol.InitialPathID].SendingAllowed() || hasRetransmission {
			return s.paths[protocol.InitialPathID], flag
		} else {
			return nil, flag
		}
	}

	if hasRetransmission && secondPath.SendingAllowed() {
		return secondPath, flag
	}
	if hasRetransmission {
		return s.paths[protocol.InitialPathID], flag
	}
	/*
		peekaboo的决策逻辑：只有在最快路径不可用，且次优路径可用的情况下，才会触发
	*/
	//	获取当前的feature
	cwndBest := float64(bestPath.sentPacketHandler.GetCongestionWindow())
	cwndSecond := float64(secondPath.sentPacketHandler.GetCongestionWindow())
	BSend := uint64(0)
	for streamid := range s.streamsMap.openStreams {
		tmpbsend, _ := s.flowControlManager.SendWindowSize(protocol.StreamID(streamid))
		BSend = BSend + uint64(tmpbsend)
	}
	//BSend, _ := s.flowControlManager.SendWindowSize(protocol.StreamID(5))
	//inflightf := float64(bestPath.sentPacketHandler.GetBytesInFlight())
	//inflights := float64(secondPath.sentPacketHandler.GetBytesInFlight())
	llowerRTT := bestPath.rttStats.LatestRTT().Milliseconds()
	lsecondLowerRTT := secondPath.rttStats.LatestRTT().Milliseconds()
	bthf := (bestPath.bth*1024*1024/8)*float64(bestPath.rttStats.LatestRTT().Seconds()) - (float64(bestPath.sentPacketHandler.GetBytesInFlight()))
	bths := (secondPath.bth*1024*1024/8)*float64(secondPath.rttStats.LatestRTT().Seconds()) - (float64(secondPath.sentPacketHandler.GetBytesInFlight()))

	//fmt.Println(bestPath.bth)

	var feature [dimension]float64
	feature[0] = cwndBest / float64(llowerRTT)
	feature[1] = cwndSecond / float64(lsecondLowerRTT)
	//feature[2] = bthf / float64(llowerRTT)
	//feature[3] = bths / float64(lsecondLowerRTT)
	feature[2] = bthf / float64(llowerRTT)
	feature[3] = bths / float64(lsecondLowerRTT)
	feature[4] = float64(BSend) / float64(llowerRTT)
	feature[5] = float64(BSend) / float64(lsecondLowerRTT)

	//FIXME: Tref计算
	var Tref int64
	Tref = int64(math.Max(float64(2*llowerRTT), float64((lsecondLowerRTT))))

	// 记录feature到缓存，为保存PeekEvent做数据准备
	for i := 0; i < dimension; i++ {
		featureCache[i] = feature[i]
	}
	TrefCache = Tref

	if state == recordwt {
		//	执行wt策略，记录到wtcaches中
		selectedpath = nil
		flag = wt
	} else if state == recordtx {
		// 执行tx策略
		selectedpath = secondPath
		flag = tx
	} else if state == learning {
		// 离线学习中，没有其他选择，直接在secondPath上传输
		selectedpath = secondPath
	} else if state == deployment {
		// 部署之后的，peekaboo决策逻辑
		xArray := mat.NewDense(dimension, 1, nil)
		for i := 0; i < dimension; i++ {
			xArray.Set(i, 0, feature[i])
		}
		ucb0 := mat.NewDense(1, 1, nil)
		ucb0.Product(peek0.theta.T(), xArray)

		ucb1 := mat.NewDense(1, 1, nil)
		ucb1.Product(peek1.theta.T(), xArray)

		if ucb0.At(0, 0) > ucb1.At(0, 0) {
			if rand.Intn(100) < int(peek0.pdeploy*100) {
				selectedpath = secondPath
				// 采用tx策略
				flag = tx
			} else {
				selectedpath = nil
				//采用wt策略
				flag = wt
			}
		} else {
			if rand.Intn(100) < int(peek1.pdeploy*100) {
				selectedpath = nil
				//采用wt策略
				flag = wt
			} else {
				selectedpath = secondPath
				// 采用tx策略
				flag = tx
			}
		}
	}
	// 若采用wt，则在队列中进行缓存
	if flag == wt {
		newwtcache := WtNewCache(feature, Tref)
		wtcaches.PushBack(newwtcache)
	}

	//if selectedpath != nil{
	//	fmt.Println(selectedpath.pathID)
	//}
	return selectedpath, flag
}

// TODO: 后台监控 q差值 的线程
func monitorThread() {
	// 只在 deployment阶段进行判定，若不处于deployment阶段，则表示正在重新采集数据进行训练，不需要再评估分数
	time.Sleep(time.Duration(6) * time.Second)

	for state == deployment {
		// 将数据备份，python的读和golang的写同时进行导致err
		input, err := ioutil.ReadFile(deployedtx)
		if err != nil {
			panic(err)
		}
		f1, _ := os.Create(deployedtxbak)
		f1.Close()
		f2, _ := os.Create(deployedwtbak)
		f2.Close()

		err = ioutil.WriteFile(deployedtxbak, input, 0644)
		if err != nil {
			panic(err)
		}
		// cat /dev/null > file_name
		removefile := exec.Command("bash", cleandeployedtx)
		err = removefile.Run()
		if err != nil {
			panic(err)
		}

		input, err = ioutil.ReadFile(deployedwt)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(deployedwtbak, input, 0644)
		if err != nil {
			panic(err)
		}

		removefile = exec.Command("bash", cleandeployedwt)
		err = removefile.Run()
		if err != nil {
			panic(err)
		}

		//执行python脚本，从脚本获取返回值标识当前的q是否超限
		args := []string{"/home/mininet/peekaboo/evaluateqdeploy.py", strconv.FormatFloat(peek0.qdeploy, 'f', 5, 64), strconv.FormatFloat(peek1.qdeploy, 'f', 5, 64)}
		_, err = exec.Command("python", args...).Output()
		if err != nil {
			panic(err)
		}
		content, err1 := os.ReadFile(checkqpth)
		if err1 != nil {
			panic(err)
		}

		err = os.Remove(deployedwtbak)
		err = os.Remove(deployedtxbak)

		qflag, _ := strconv.ParseFloat(string(content), 64)

		removefile = exec.Command("bash", cleancheckqpth)
		err = removefile.Run()
		if err != nil {
			panic(err)
		}

		if qflag == -1 {
			// 需要重新训练
			fmt.Println("Need to retrain Peekaboo!")
			state = initstate
			break
		} else {
			time.Sleep(time.Duration(1) * time.Second)
		}
	}

	removefile := exec.Command("bash", cleantxpath)
	err := removefile.Run()
	if err != nil {
		panic(err)
	}

	removefile = exec.Command("bash", cleanwtpath)
	err = removefile.Run()
	if err != nil {
		panic(err)
	}

	removefile = exec.Command("bash", cleaninputpath)
	err = removefile.Run()
	if err != nil {
		panic(err)
	}

	fmt.Println("Stop Monitoring, Retrain Peek")
}

// 开始learning阶段；训练结果部署到当前peeka调度器上
func offlineTrainPeek() {
	// 调用python脚本
	args := []string{"/home/mininet/peekaboo/learningphase.py"}
	_, err := exec.Command("python", args...).Output()
	if err != nil {
		panic(err)
	}

	fmt.Println("Learning Finised!")
	// 读取文件内容
	// input文件格式：peekID、feature[0-5]、p
	var strtx string
	var strwt string
	f, err := os.Open(inputpath)
	if err != nil {
		panic(any(err))
	}
	buf := bufio.NewReader(f)
	for {
		line, erri := buf.ReadBytes('\n')
		tmpstr := string(line)

		if erri != nil {
			if erri == io.EOF {
				break
			} else {
				panic(any(erri))
			}
		}

		if tmpstr[0] == '0' {
			strtx = tmpstr
		} else if tmpstr[0] == '1' {
			strwt = tmpstr
		}
	}

	peek0.theta = mat.NewDense(dimension, 1, nil)
	peek1.theta = mat.NewDense(dimension, 1, nil)
	strtxarray := strings.Split(strtx, ",")
	strwtarray := strings.Split(strwt, ",")
	for i := 1; i <= 6; i++ {
		tmp0, _ := strconv.ParseFloat(strtxarray[i], 64)
		tmp1, _ := strconv.ParseFloat(strwtarray[i], 64)

		peek0.theta.Set(i-1, 0, tmp0)
		peek1.theta.Set(i-1, 0, tmp1)
	}

	peek0.pdeploy, _ = strconv.ParseFloat(strtxarray[7], 64)
	peek1.pdeploy, _ = strconv.ParseFloat(strwtarray[7], 64)

	peek0.qdeploy, _ = strconv.ParseFloat(strtxarray[8], 64)
	peek1.qdeploy, _ = strconv.ParseFloat(strwtarray[8], 64)

	fmt.Println("peek0's Q-deploy ", peek0.qdeploy)
	fmt.Println("peek1's Q-deploy ", peek1.qdeploy)

	state = deployment

	//	TODO：开启监控
	fmt.Println("Start Monitoring......")
	go monitorThread()
}

// 获取最快和第二快路径
func (sch *scheduler) selectPathPeeka(s *session, hasRetransmission bool, hasStreamRetransmission bool, fromPth *path) (*path, *path) {
	// XXX Avoid using PathID 0 if there is more than 1 path
	if len(s.paths) <= 1 {
		if !hasRetransmission && !s.paths[protocol.InitialPathID].SendingAllowed() {
			return nil, nil
		}
		return s.paths[protocol.InitialPathID], nil
	}

	// FIXME Only works at the beginning... Cope with new paths during the connection
	if hasRetransmission && hasStreamRetransmission && fromPth.rttStats.SmoothedRTT() == 0 {
		// Is there any other path with a lower number of packet sent?
		currentQuota := sch.quotas[fromPth.pathID]
		for pathID, pth := range s.paths {
			if pathID == protocol.InitialPathID || pathID == fromPth.pathID {
				continue
			}
			// The congestion window was checked when duplicating the packet
			if sch.quotas[pathID] < currentQuota {
				return pth, nil
			}
		}
	}

	var bestPath *path
	var secondBestPath *path
	var lowerRTT time.Duration
	var currentRTT time.Duration
	var secondLowerRTT time.Duration
	bestPathID := protocol.PathID(255)

pathLoop:
	for pathID, pth := range s.paths {
		// If this path is potentially failed, do not consider it for sending
		if pth.potentiallyFailed.Get() {
			continue pathLoop
		}

		// XXX Prevent using initial pathID if multiple paths
		if pathID == protocol.InitialPathID {
			continue pathLoop
		}

		currentRTT = pth.rttStats.SmoothedRTT()

		// Prefer staying single-path if not blocked by current path
		// Don't consider this sample if the smoothed RTT is 0
		if lowerRTT != 0 && currentRTT == 0 {
			continue pathLoop
		}

		// Case if we have multiple paths unprobed
		if currentRTT == 0 {
			currentQuota, ok := sch.quotas[pathID]
			if !ok {
				sch.quotas[pathID] = 0
				currentQuota = 0
			}
			lowerQuota, _ := sch.quotas[bestPathID]
			if bestPath != nil && currentQuota > lowerQuota {
				continue pathLoop
			}
		}

		if currentRTT >= lowerRTT {
			if (secondLowerRTT == 0 || currentRTT < secondLowerRTT) && pth.SendingAllowed() {
				// Update second best available path
				secondLowerRTT = currentRTT
				secondBestPath = pth
			}
			if currentRTT != 0 && lowerRTT != 0 && bestPath != nil {
				continue pathLoop
			}
		}

		// Update
		lowerRTT = currentRTT
		bestPath = pth
		bestPathID = pathID

	}

	return bestPath, secondBestPath
}

//TODO:使用AHP算法筛选出最优和次优路径
type SortBy []*path

func (a SortBy) Len() int      { return len(a) }
func (a SortBy) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortBy) Less(i, j int) bool {
	return a[i].curscore > a[j].curscore
}

func (sch *scheduler) selectPathAHP(s *session, hasRetransmission bool, hasStreamRetransmission bool, fromPth *path) (*path, *path) {
	_, err := os.Stat(flagfile)

	// 版本号太小，读取最新的链路状态
	if err == nil || os.IsExist(err) {
		////同步版本号
		//sch.pathversion = version
		os.Remove(flagfile)

		fmt.Println("update pth status")
		infofile, err1 := os.Open(pathinfo)
		if err1 != nil {
			panic(any(err1))
		}
		defer infofile.Close()
		reader := bufio.NewReader(infofile)
		for {
			str, err2 := reader.ReadString('\n')
			if err2 == io.EOF {
				break
			}
			stringSlice := strings.Split(str, " ")

			ip := stringSlice[0]
			ip = strings.Replace(ip, "\n", "", -1)

			loss, _ := strconv.ParseFloat(stringSlice[1], 64)

			bthstr := strings.Replace(stringSlice[2], "\n", "", -1)
			bth, _ := strconv.ParseFloat(bthstr, 64)

			s.paths[s.addrs[ip]].loss = loss
			s.paths[s.addrs[ip]].bth = bth

		}

		//更新各个链路的loss得分
		for _, pathID := range s.addrs {
			pth := s.paths[pathID]
			totallossrate := float64(0)

			for _, pathID1 := range s.addrs {
				tmppth := s.paths[pathID1]
				totallossrate = totallossrate + pth.loss/tmppth.loss
			}
			pth.lossscore = 1 / totallossrate
		}
		//sch.pathversion += 1
		//index指针归零
		sch.countindex = 0
	}
	//计算链路的bth得分
	for _, pathID := range s.addrs {
		pth := s.paths[pathID]
		totalbthrate := float64(0)

		pthbth1 := (pth.bth*1024*1024/8)*pth.rttStats.LatestRTT().Seconds() - (float64(pth.sentPacketHandler.GetBytesInFlight()))
		for _, pathID1 := range s.addrs {
			tmppth := s.paths[pathID1]
			pthbth2 := (tmppth.bth*1024*1024/8)*tmppth.rttStats.LatestRTT().Seconds() - (float64(tmppth.sentPacketHandler.GetBytesInFlight()))
			totalbthrate = totalbthrate + pthbth2/pthbth1
		}

		pth.bthscore = 1 / totalbthrate
	}

	//计算链路的rtt得分
	for _, pathID := range s.addrs {
		pth := s.paths[pathID]
		totalrttrate := float64(0)

		for _, pathID1 := range s.addrs {
			tmppth := s.paths[pathID1]
			if pth.rttStats.SmoothedRTT().Milliseconds() == 0 || tmppth.rttStats.SmoothedRTT().Milliseconds() == 0 {
				totalrttrate = totalrttrate + 1
			} else {
				totalrttrate = totalrttrate + float64(pth.rttStats.SmoothedRTT().Milliseconds()/tmppth.rttStats.SmoothedRTT().Milliseconds())
			}
		}

		pth.rttscore = 1 / totalrttrate
	}

	//根据业务类型代入算式计算得分
	var params map[string]float64
	if s.missiontype == "media" {
		params = mediaparams
	} else if s.missiontype == "sess" {
		params = sessparams
	} else if s.missiontype == "back" {
		params = backparams
	}

	var pthlist []*path
pathLoop:
	for pathID, pth := range s.paths {
		if pth.potentiallyFailed.Get() {
			continue pathLoop
		}

		if pathID == protocol.InitialPathID {
			continue pathLoop
		}

		pth.sentPacketHandler.GetBytesInFlight()
		pth.curscore = pth.rttscore*params["rtt"] + pth.lossscore*params["loss"] + (pth.bthscore)*params["bth"]
		pthlist = append(pthlist, pth)
	}
	if len(pthlist) == 0 {
		//fmt.Println("shit")
		return s.paths[0], nil
	}
	//TODO: 将选择的路径错开
	sort.Sort(SortBy(pthlist))
	//var mysize = int64(len(pthlist))
	//var curindex = sch.countindex % mysize
	pth1 := pthlist[0]
	var pth2 *path = nil
	if len(pthlist) > 1 {
		for _, pth := range pthlist {
			//if id != 0 && pth != pth1 && pth.SendingAllowed() {
			if pth.SendingAllowed() {
				pth2 = pth
				break
			}
		}
	}

	sch.countindex = sch.countindex + 1

	return pth1, pth2
}

func (sch *scheduler) selectPathAHPnew(s *session, hasRetransmission bool, hasStreamRetransmission bool, fromPth *path) (*path, *path) {
	_, err := os.Stat(flagfile)

	// 版本号太小，读取最新的链路状态
	if err == nil || os.IsExist(err) {
		////同步版本号
		//sch.pathversion = version
		os.Remove(flagfile)

		fmt.Println("update pth status")
		infofile, err1 := os.Open(pathinfo)
		if err1 != nil {
			panic(any(err1))
		}
		defer infofile.Close()
		reader := bufio.NewReader(infofile)
		for {
			str, err2 := reader.ReadString('\n')
			if err2 == io.EOF {
				break
			}
			stringSlice := strings.Split(str, " ")

			ip := stringSlice[0]
			ip = strings.Replace(ip, "\n", "", -1)

			loss, _ := strconv.ParseFloat(stringSlice[1], 64)

			bthstr := strings.Replace(stringSlice[2], "\n", "", -1)
			bth, _ := strconv.ParseFloat(bthstr, 64)

			s.paths[s.addrs[ip]].loss = loss
			s.paths[s.addrs[ip]].bth = bth

		}

		//更新各个链路的loss\bth\rtt得分
		for _, pathID := range s.addrs {
			pth := s.paths[pathID]
			totallossrate := float64(0)
			totalbthrate := float64(0)
			totalrttrate := float64(0)

			for _, pathID1 := range s.addrs {
				tmppth := s.paths[pathID1]
				totallossrate = totallossrate + pth.loss/tmppth.loss
				totalbthrate = totalbthrate + tmppth.bth/pth.bth
				totalrttrate = totalrttrate + pth.rtt/tmppth.rtt
			}

			pth.lossscore = 1 / totallossrate
			pth.bthscore = 1 / totalbthrate
			pth.rttscore = 1 / totalrttrate
		}

		sch.countindex = 0
	}

	//根据业务类型代入算式计算得分
	var params map[string]float64
	if s.missiontype == "media" {
		params = mediaparams
	} else if s.missiontype == "sess" {
		params = sessparams
	} else if s.missiontype == "back" {
		params = backparams
	}

	var pthlist []*path
pathLoop:
	for pathID, pth := range s.paths {
		if pth.potentiallyFailed.Get() {
			continue pathLoop
		}

		if pathID == protocol.InitialPathID {
			continue pathLoop
		}

		pth.sentPacketHandler.GetBytesInFlight()
		pth.curscore = pth.rttscore*params["rtt"] + pth.lossscore*params["loss"] + (pth.bthscore)*params["bth"]
		pthlist = append(pthlist, pth)
	}
	if len(pthlist) == 0 {
		//fmt.Println("shit")
		return s.paths[0], nil
	}
	//TODO: 将选择的路径错开
	sort.Sort(SortBy(pthlist))
	//var mysize = int64(len(pthlist))
	//var curindex = sch.countindex % mysize
	pth1 := pthlist[0]
	var pth2 *path = nil
	if len(pthlist) > 1 {
		for _, pth := range pthlist {
			//if id != 0 && pth != pth1 && pth.SendingAllowed() {
			if pth.SendingAllowed() {
				pth2 = pth
				break
			}
		}
	}

	sch.countindex = sch.countindex + 1

	return pth1, pth2
}

func (sch *scheduler) selectPathAHPonly(s *session, hasRetransmission bool, hasStreamRetransmission bool, fromPth *path) *path {
	_, err := os.Stat(flagfile)

	// 版本号太小，读取最新的链路状态
	if err == nil || os.IsExist(err) {
		////同步版本号
		//sch.pathversion = version
		os.Remove(flagfile)

		fmt.Println("update pth status")
		infofile, err1 := os.Open(pathinfo)
		if err1 != nil {
			panic(any(err1))
		}
		defer infofile.Close()
		reader := bufio.NewReader(infofile)
		for {
			str, err2 := reader.ReadString('\n')
			if err2 == io.EOF {
				break
			}
			stringSlice := strings.Split(str, " ")

			ip := stringSlice[0]
			ip = strings.Replace(ip, "\n", "", -1)

			loss, _ := strconv.ParseFloat(stringSlice[1], 64)

			bthstr := strings.Replace(stringSlice[2], "\n", "", -1)
			bth, _ := strconv.ParseFloat(bthstr, 64)
			//fmt.Println(bth)

			s.paths[s.addrs[ip]].loss = loss
			s.paths[s.addrs[ip]].bth = bth

		}

		//更新各个链路的loss得分
		for _, pathID := range s.addrs {
			pth := s.paths[pathID]
			totallossrate := float64(0)

			for _, pathID1 := range s.addrs {
				tmppth := s.paths[pathID1]
				totallossrate = totallossrate + pth.loss/tmppth.loss
			}
			pth.lossscore = 1 / totallossrate
		}
		//sch.pathversion += 1
		//index指针归零
		sch.countindex = 0
	}
	//计算链路的bth得分
	for _, pathID := range s.addrs {
		pth := s.paths[pathID]
		totalbthrate := float64(0)

		pthbth1 := (pth.bth*1024*1024/8)*pth.rttStats.LatestRTT().Seconds() - (float64(pth.sentPacketHandler.GetBytesInFlight()))
		for _, pathID1 := range s.addrs {
			tmppth := s.paths[pathID1]
			pthbth2 := (tmppth.bth*1024*1024/8)*tmppth.rttStats.LatestRTT().Seconds() - (float64(tmppth.sentPacketHandler.GetBytesInFlight()))
			totalbthrate = totalbthrate + pthbth2/pthbth1
		}

		pth.bthscore = 1 / totalbthrate
	}

	//计算链路的rtt得分
	for _, pathID := range s.addrs {
		pth := s.paths[pathID]
		totalrttrate := float64(0)

		for _, pathID1 := range s.addrs {
			tmppth := s.paths[pathID1]
			if pth.rttStats.SmoothedRTT().Milliseconds() == 0 || tmppth.rttStats.SmoothedRTT().Milliseconds() == 0 {
				totalrttrate = totalrttrate + 1
			} else {
				totalrttrate = totalrttrate + float64(pth.rttStats.SmoothedRTT().Milliseconds()/tmppth.rttStats.SmoothedRTT().Milliseconds())
			}
		}

		pth.rttscore = 1 / totalrttrate
	}

	//根据业务类型代入算式计算得分
	var params map[string]float64
	if s.missiontype == "media" {
		params = mediaparams
	} else if s.missiontype == "sess" {
		params = sessparams
	} else if s.missiontype == "back" {
		params = backparams
	}

	var pthlist []*path
pathLoop:
	for pathID, pth := range s.paths {
		if pth.potentiallyFailed.Get() {
			continue pathLoop
		}

		if pathID == protocol.InitialPathID {
			continue pathLoop
		}

		pth.sentPacketHandler.GetBytesInFlight()
		pth.curscore = pth.rttscore*params["rtt"] + pth.lossscore*params["loss"] + (pth.bthscore)*params["bth"]
		pthlist = append(pthlist, pth)
	}
	if len(pthlist) == 0 {
		//fmt.Println("shit")
		return s.paths[0]
	}

	sort.Sort(SortBy(pthlist))

	pth1 := pthlist[0]

	sch.countindex = sch.countindex + 1

	return pth1
}

// 记录tx决策的同时，将wt决策缓存记录到event，清空wtcaches ;记为与tx同一个pkt和pathID
func (sch *scheduler) storePeekAction(s *session, pathID protocol.PathID, pkt *ackhandler.Packet, flag string) {
	var packetNumber protocol.PacketNumber = pkt.PacketNumber
	if sch.peekmemories[pathID] == nil {
		sch.peekmemories[pathID] = PeekNewMemory()
	}
	if flag == tx {
		state := featureCache
		event := PeekNewEvent(pathID, 0, packetNumber, state, pkt.Length, TrefCache, time.Now())

		sch.peekmemories[pathID].PushBack(event)
	}

	// 将wt决策的缓存也记录到memory中
	for {
		if wtcaches.Len() == 0 {
			break
		}
		var Frontdata = wtcaches.Front().(*wtcache)
		wtcaches.PopFront()
		state := Frontdata.feature
		// 虽然wt决策没有选择路径进行传输，暂时设置为与当前相同的发送路径和数据包
		event := PeekNewEvent(pathID, 1, packetNumber, state, pkt.Length, Frontdata.Tref, Frontdata.decideTime)
		sch.peekmemories[pathID].PushBack(event)
	}
}

// 处理ack帧
//	输出文件的格式：调度策略、state[0~5]，Tref，Telap，瞬时reward, packetNumber
func (sch *scheduler) receivedACKForPeeka(s *session, ackFrame *wire.AckFrame) {
	var pathID = ackFrame.PathID

	//确认的最大PacketNumber
	var ack = ackFrame.LargestAcked
	// Calculation of ACK number received without loss
	if len(ackFrame.AckRanges) > 0 {
		ack = ackFrame.AckRanges[len(ackFrame.AckRanges)-1].Last
	}

	if sch.peekmemories[pathID] == nil {
		return
	}

	var filemap map[PeekID]*os.File
	filemap = make(map[PeekID]*os.File)
	var err error
	filemap[0], err = os.OpenFile(txpath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(any(err))
	}
	// 会不会多线程出问题？
	defer filemap[0].Close()
	filemap[1], err = os.OpenFile(wtpath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(any(err))
	}
	defer filemap[1].Close()

	var writermap map[PeekID]*bufio.Writer
	writermap = make(map[PeekID]*bufio.Writer)
	writermap[0] = bufio.NewWriter(filemap[0])
	writermap[1] = bufio.NewWriter(filemap[1])

	var deployfile map[PeekID]*os.File
	deployfile = make(map[PeekID]*os.File)

	deployfile[0], err = os.OpenFile(deployedtx, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(any(err))
	}
	defer deployfile[0].Close()

	deployfile[1], err = os.OpenFile(deployedwt, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		panic(any(err))
	}
	defer deployfile[1].Close()

	var deploywriter map[PeekID]*bufio.Writer
	deploywriter = make(map[PeekID]*bufio.Writer)
	deploywriter[0] = bufio.NewWriter(deployfile[0])
	deploywriter[1] = bufio.NewWriter(deployfile[1])

	//	遍历已保存的peekaboo事件
	for {
		if sch.peekmemories[pathID].Len() == 0 {
			break
		}

		var FrontData = sch.peekmemories[pathID].Front().(*PeekEvent)
		sch.peekmemories[pathID].PopFront()
		if FrontData.PacketNumber > ack {
			// 当前ack的包已经确认过，跳过
			break
		}

		// 将FrontData记录到文件
		duration := time.Since(FrontData.decideTime).Milliseconds()

		pktlen := FrontData.PacketLength
		//FIXME：计算奖励使用的是Tack，和Telap的区分？
		currentReward := float64(pktlen) / float64(duration)

		var peekid = FrontData.peekID
		var slice []string
		slice = append(slice, strconv.FormatInt(int64(peekid), 10))
		for _, num := range FrontData.state {
			slice = append(slice, strconv.FormatFloat(num, 'f', 5, 64))
		}
		slice = append(slice, strconv.FormatInt(FrontData.Tref, 10))
		slice = append(slice, strconv.FormatInt(time.Since(FrontData.decideTime).Milliseconds(), 10))
		slice = append(slice, strconv.FormatFloat(currentReward, 'f', 5, 64))
		slice = append(slice, strconv.FormatInt(int64(FrontData.PacketNumber), 10))
		outputstr := strings.Join(slice, ",")
		outputstr += "\n" // 换行符

		//FIXME：对于不同state下的记录逻辑，如learning阶段也需要进行记录决策历史以实现q值的更新比较
		if state == recordwt {
			writermap[peekid].WriteString(outputstr)
			writermap[peekid].Flush()

			recordnumwt--
			if recordnumwt <= 0 {
				fmt.Println("Start recording tx")
				state = recordtx
			}
		} else if state == recordtx {
			writermap[peekid].WriteString(outputstr)
			writermap[peekid].Flush()

			recordnumtx--
			if recordnumtx <= 0 {
				state = learning

				recordnumwt = recordNum / 2
				recordnumtx = recordNum / 2
				fmt.Println("Start Training!!!!")
				go offlineTrainPeek()
				fmt.Println("Training Peekaboo Finished! Deploy!")
			}
		} else if state == learning {
			// 在learning和deployment阶段，将数据存储到不同的路径下
			deploywriter[peekid].WriteString(outputstr)
			deploywriter[peekid].Flush()
		} else if state == deployment {
			deploywriter[peekid].WriteString(outputstr)
			deploywriter[peekid].Flush()
		}
	}
}
