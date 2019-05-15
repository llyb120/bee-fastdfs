package main

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	config  *Config
	proxies []*httputil.ReverseProxy
	syncing []bool
)

func StartFileServer(path string) {
	config = NewConfig(path)
	//反向代理初始化
	proxies = make([]*httputil.ReverseProxy, len(config.Peers))
	syncing = make([]bool, len(config.Peers))
	for i, v := range config.Peers {
		if i == config.Index {
			continue
		}
		remote, err := url.Parse("http://" + v)
		if err != nil {
			continue
		}
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Host = remote.Host
				//req.URL.Path = remote.Path + reg.ReplaceAllString(req.URL.Path, "")
				req.URL.Scheme = remote.Scheme
				req.Host = remote.Host
				//req.Header.Set("Server", remote.Host)
				//req.Header.Set("Host", remote.Host)
				req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			},
			//ModifyResponse: responseFactory(value, remote, reg),
		}
		proxies[i] = proxy
		syncing[i] = false
	}
	go InitDb()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		log.Fatal("cannot listen %d", config.Port)
	}

	var mux = http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		log.Printf(request.URL.Path)
		switch request.URL.Path {
		case "/upload":
			upload(writer, request)
			return
		case "/syncList":
			syncList(writer,request)
			return

		default:
			arr := strings.Split(request.URL.Path, "/")
			if len(arr) == 3 && strings.Index(arr[1], "group") == 0 {
				group := arr[1][5:]
				g, err := strconv.Atoi(group)
				if err != nil {
					goto NOT_FOUND
				}
				download(g, arr[2], writer, request)
				return
			}
		}

	NOT_FOUND:
		{
			http.NotFound(writer, request)
		}

	})
	//mux.HandleFunc("/api/upload", upload)
	//mux.HandleFunc("/api/download", download)

	log.Printf("%+v", config)

	go http.Serve(listener, mux)
	log.Printf("server is listening on %d \n", config.Port)

	go SyncLoop()

	//wait main thread
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}

func download(group int, filename string, w http.ResponseWriter, r *http.Request) {
	fmt.Printf("fff")
	val, err := ldb.Get([]byte(filename), nil)
	if err != nil {
		//goto NOT_FOUND
		//如果没找到，反向代理到该去的地方
		if group == config.Index {
			fileNotFound(w,r)
			return
		}
		proxies[group].ServeHTTP(w, r)
	}

	serveFile(path.Join(config.Dir, string(val)), w, r)
}

func upload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		err := r.ParseMultipartForm(4096)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m := r.MultipartForm
		files := m.File["file"]
		if len(files) == 0 {
			http.Error(w, "no file to upload", http.StatusInternalServerError)
			return
		}

		var id = NextObjectId()

		for i, v := range files {
			if i == 1 {
				break
			}
			file, err := v.Open()
			defer file.Close()

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			//create save dir
			dir := "/" + time.Now().Format("2006-01-02")
			target := config.Dir + dir
			if err = EnsureDir(target); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			//create destination file making sure the path is writeable.
			ext := path.Ext(v.Filename)
			if ext == "" {
				ext = ".unknown"
			}
			filename := "/" + NextObjectId() + ext
			dst, err := os.Create(target + filename)
			defer dst.Close()

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			//copy the uploaded file to the destination file
			if _, err := io.Copy(dst, file); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			fullPath := dir + filename
			err = ldb.Put([]byte(id+ext), []byte(fullPath), nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			go AddFileLog(id, fullPath)

			//真实的ID
			rid := fmt.Sprintf("/group%d/%s%s", config.Index, id, ext)

			w.WriteHeader(200)
			_, _ = w.Write([]byte(rid))
		}

		//var resp success
		//resp.State = 200
		//resp.Id = id
		//bs, err := json.Marshal(resp)
		//if err != nil {
		//	http.Error(w, err.Error(), http.StatusInternalServerError)
		//	return
		//}
		//w.Header().Set("Content-Type", "application/json; charset=utf-8")
		//w.Header().Set("Content-Length", strconv.Itoa(len(bs)))
		//_, _ = w.Write(bs)

	default:
		http.NotFound(w, r)
		//w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func serveFile(p string, w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "404 Not Found : Error while opening the file.", 404)
		return
	}

	defer f.Close()

	// Checking if the opened handle is really a file
	statinfo, err := f.Stat()
	if err != nil {
		fileNotFound(w,r)
		return
	}

	if statinfo.IsDir() { // If it's a directory, open it !
		fileNotFound(w,r)
		return
	}

	if (statinfo.Mode() &^ 07777) == os.ModeSocket { // If it's a socket, forbid it !
		http.Error(w, "403 Forbidden : you can't access this resource.", 403)
		return
	}

	// Manages If-Modified-Since and add Last-Modified (taken from Golang code)
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && statinfo.ModTime().Unix() <= t.Unix() {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Last-Modified", statinfo.ModTime().Format(http.TimeFormat))
	// Fetching file's mimetype and giving it to the browser
	if mimetype := mime.TypeByExtension(path.Ext(p)); mimetype != "" {
		w.Header().Set("Content-Type", mimetype)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Manage gzip/zlib compression
	output_writer := w.(io.Writer)

	is_compressed_reply := false

	if r.Header.Get("Accept-Encoding") != "" {
		encodings := ParseCSV(r.Header.Get("Accept-Encoding"))

		for _, val := range encodings {
			if val == "gzip" {
				w.Header().Set("Content-Encoding", "gzip")
				output_writer = gzip.NewWriter(w)

				is_compressed_reply = true

				break
			} else if val == "deflate" {
				w.Header().Set("Content-Encoding", "deflate")
				output_writer = zlib.NewWriter(w)

				is_compressed_reply = true

				break
			}
		}
	}

	if !is_compressed_reply {
		// Add Content-Length
		w.Header().Set("Content-Length", strconv.FormatInt(statinfo.Size(), 10))
	}

	// Stream data out !
	buf := make([]byte, Min(4096, statinfo.Size()))
	n := 0
	for err == nil {
		n, err = f.Read(buf)
		_, _ = output_writer.Write(buf[0:n])
	}

	// Closes current compressors
	switch output_writer.(type) {
	case *gzip.Writer:
		err = output_writer.(*gzip.Writer).Close()
		if err != nil {
			fileError(w,r)
		}
	case *zlib.Writer:
		err = output_writer.(*zlib.Writer).Close()
		if err != nil {
			fileError(w,r)
		}
	}

}

func syncList(w http.ResponseWriter, r *http.Request)  {
	group := r.URL.Query().Get("fromGroupId")
	lastId := r.URL.Query().Get("lastId")

	li, err := GetNeedToSyncList(group, lastId)
	if err != nil {
		fileNotFound(w,r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bs, err := json.Marshal(li)
	if err != nil {
		fileNotFound(w,r)
		return
	}
	w.WriteHeader(200)
	w.Write(bs)
}

func fileNotFound(w http.ResponseWriter, r *http.Request)  {
	http.NotFound(w, r)
}
func fileError(w http.ResponseWriter, r *http.Request)  {
	http.Error(w, "error file", http.StatusInternalServerError)
}
