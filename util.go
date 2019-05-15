package main

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func EnsureDir(path string) error {
	//处理属性
	if !Exists(path){
		err := os.MkdirAll(path, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func Exists(path string)  bool{
	_, err := os.Stat(path)
	//没有就创建该目录
	return err == nil
}

func ParseCSV(data string) []string {
	splitted := strings.SplitN(data, ",", -1)

	data_tmp := make([]string, len(splitted))

	for i, val := range splitted {
		data_tmp[i] = strings.TrimSpace(val)
	}

	return data_tmp
}

func Min(x int64, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func RandInt64(min, max int64) int64 {
	if min >= max || min == 0 || max == 0 {
		return max
	}
	return rand.Int63n(max-min) + min
}


func GetIp() map[string]bool{
	ret := make(map[string]bool)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ret
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ret[ipnet.IP.String()] = true
			}
		}
	}
	return ret
}


/****************
mongodb objectId
 */

var (
	objIdMutex sync.Mutex
	pid        = int32(os.Getpid())
	machine    = getMachineHash()
	increment  = getRandomNumber()
)

func NextObjectId() string {
	objIdMutex.Lock()
	defer objIdMutex.Unlock()
	timestamp := time.Now().Unix()
	increment++
	array := []byte{
		byte(timestamp >> 0x18),
		byte(timestamp >> 0x10),
		byte(timestamp >> 8),
		byte(timestamp),
		byte(machine >> 0x10),
		byte(machine >> 8),
		byte(machine),
		byte(pid >> 8),
		byte(pid),
		byte(increment >> 0x10),
		byte(increment >> 8),
		byte(increment),
	}
	return hex.EncodeToString(array)
}

func getMachineHash() int32 {
	machineName, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	buf := md5.Sum([]byte(machineName))
	return (int32(buf[0])<<0x10 + int32(buf[1])<<8) + int32(buf[2])
}

func getRandomNumber() int32 {
	rand.Seed(time.Now().UnixNano())
	return rand.Int31()
}
