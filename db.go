package main

import (
	"database/sql"
	"fileserver/asset"
	_ "github.com/mattn/go-sqlite3"
	"github.com/syndtr/goleveldb/leveldb"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

//func a()  {
//	_, err := leveldb.OpenFile("path/to/db", nil)
//	if err != nil {
//
//	}
//
//}
var (
	ldb *leveldb.DB
	sqldb *sql.DB
)

func InitDb()  {
	//如果不对存在数据库，则先创建
	dbpath := path.Join(config.Dir,"db/data.db")
	if !Exists(dbpath) {
		bs,err := asset.Asset("static/data.db")
		if err != nil {
			log.Fatal("can not read data.db")
		}
		err = ioutil.WriteFile(dbpath, bs, os.ModeAppend)
		if err != nil {
			log.Fatal("can not create data.db")
		}
	}
	var err error
	sqldb, err = sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatal("open sql db error")
	}
	sqldb.SetMaxOpenConns(2000)
	sqldb.SetMaxIdleConns(1000)
	sqldb.Ping()

	ldb, err = leveldb.OpenFile(config.Dir + "/db/", nil)
	if err != nil {
		log.Fatal("cannnot open level db at" + config.Dir + "/db/")
	}

	//i := 0
	//for i < 100 {
	//	ldb.Put([]byte(NextObjectId()), []byte(strconv.Itoa(i)), nil )
	//	i += 1
	//}
	//
	////iter := ldb.NewIterator(nil, nil)
	//iter := ldb.NewIterator(nil, nil)
	//iter.Seek([]byte("5ccc05621913af072c4b867c"))
	//for iter.Next() {
	//	log.Println(string(iter.Key()) + " " + string(iter.Value()))
	//}
	////for ok := iter.Seek([]byte("10")); ok; ok = iter.Next() {
	//	// Use key/value.
	////}
	//iter.Release()
	//err = iter.Error()
}

func CloseDb()  {
	if ldb != nil {
		_ = ldb.Close()
	}
	if sqldb != nil {
		_ = sqldb.Close()
	}
}

func AddFileLog(id string, path string) error {
	stmt, err := sqldb.Prepare("INSERT INTO t_log(id, path, add_time, action, file_id) values(null ,?,?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(path, time.Now().Unix(), 1, id)
	if err != nil {
		return err
	}
	return nil
}

func GetLastSyncFileId(group int) (fileId int, err error) {
	ret, err := sqldb.Query(("select last_file_id from t_sync_log where group_id = ? limit 1"), group);
	if err != nil {
		return
	}
	for ret.Next(){
		ret.Scan(&fileId)
	}
	return
}

func GetNeedToSyncList(group string, lastId string) (li []SyncFileItem, err error)  {
	ret, err := sqldb.Query(("select action, file_id, path from t_log where id > ? order by add_time asc"), lastId);
	if err != nil {
		return
	}
	for ret.Next() {
		var syncFileItem SyncFileItem
		ret.Scan(&syncFileItem.Action, &syncFileItem.FileId, &syncFileItem.Path)
		li = append(li, syncFileItem)
	}

	return

}

