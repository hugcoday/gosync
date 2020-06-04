package gosync

import (
	"encoding/gob"
	// "fmt"
	"net"
)

// md5 string
type md5s string

// host返回的结果
type ret struct {
	Status bool
	ErrStr string
}

// host返回的结果
type hostRet struct {
	hostIP
	ret
}

// 将host的sync结果push到channel
func putRetCh(host hostIP, errStr string, retCh chan hostRet) {
	var re ret
	if errStr != "" {
		re = ret{false, errStr}
	} else {
		re = ret{true, ""}
	}
	retCh <- hostRet{host, re}
}

// 启动监控进程, 和各目标host建立连接
func TravHosts(hosts []string, fileMd5List []string, flMd5 md5s, mg *Message, diffCh chan diffInfo, retCh chan hostRet, taskID string) {

	var port = ":38999"
	for _, host := range hosts {
		conn, cnErr := net.Dial("tcp", host+port)
		// 建立连接失败, 即此目标host同步失败
		if cnErr != nil {
			putRetCh(hostIP(host), cnErr.Error(), retCh)
			continue
		}
		go hdRetConn(conn, fileMd5List, flMd5, mg, diffCh, retCh, taskID)
	}
}

// 发送源host的文件列表, 接收目标host的请求列表, 接收目标host的sync结果
// flMd5: md5 of fileMd5List
func hdRetConn(conn net.Conn, fileMd5List []string, flMd5 md5s, mg *Message, diffCh chan diffInfo, retCh chan hostRet, taskID string) {
	defer conn.Close()
	// 包装conn
	gbc := initGobConn(conn)

	// 发送fileMd5List
	var fileMd5ListMg Message
	fileMd5ListMg.TaskID = taskID
	fileMd5ListMg.MgID = RandId()
	fileMd5ListMg.MgType = "allFilesMd5List"
	fileMd5ListMg.MgString = string(flMd5)
	fileMd5ListMg.MgStrings = fileMd5List
	fileMd5ListMg.DstPath = mg.DstPath
	fileMd5ListMg.Del = mg.Del
	fileMd5ListMg.Overwrt = mg.Overwrt
	err := gbc.gobConnWt(fileMd5ListMg)
	// 如果encode失败, 则此conn对应的目标host同步失败
	if err != nil {
		// lg.Printf("%s\t%s\n", conn.RemoteAddr().String(), err)
		DubugInfor(conn.RemoteAddr().String(), "\t", err)
		putRetCh(hostIP(conn.RemoteAddr().String()), err.Error(), retCh)
		return
	}

	// 设置超时器, 1min
	fresher := make(chan struct{})
	ender := make(chan struct{})
	stop := make(chan struct{})
	go setTimer(fresher, ender, stop, 60)

	// 用于接收目标host发来的信息
	var hostMg Message
	dataRecCh := make(chan Message)
	go dataReciver(gbc.dec, dataRecCh)

	var diffFile diffInfo
	var diffFlag int // 是否是在"diffOfFilesMd5List"返回之前返回了结果
ENDCONN:
	for {
		select {
		case <-stop:
			// 超时失败
			// err = fmt.Errorf("%s", "timeout 60s")
			putRetCh(hostIP(conn.RemoteAddr().String()), "timeout 60s", retCh)
			if diffFlag != 1 {
				diffFile.files = nil
				diffCh <- diffFile
			}
			break ENDCONN
		case hostMg = <-dataRecCh:
			switch hostMg.MgType {
			case "result":
				var errStr string
				if hostMg.B {
					err = nil
					errStr = ""
				} else {
					// err = fmt.Errorf("%s", hostMg.MgString)
					errStr = hostMg.MgString
				}
				DubugInfor("the result will be pushed to retCh.")
				putRetCh(hostIP(conn.RemoteAddr().String()), errStr, retCh)
				if diffFlag != 1 {
					diffFile.files = nil
					diffCh <- diffFile
				}
				ender <- struct{}{}
				break ENDCONN
			case "diffOfFilesMd5List":
				diffFile.files = hostMg.MgStrings
				diffFile.hostIP = hostIP(conn.RemoteAddr().String())
				diffFile.md5s = md5s(hostMg.MgString)
				diffCh <- diffFile
				diffFlag = 1
				fresher <- struct{}{}
			case "live": // heartbeat
				fresher <- struct{}{}
			}
		}
	}
}

func dataReciver(dec *gob.Decoder, dataRecCh chan Message) {
	var hostMessage Message
	for {
		err := dec.Decode(&hostMessage)
		if err != nil {
			// lg.Println(err)
			break
		}
		dataRecCh <- hostMessage
	}
}
