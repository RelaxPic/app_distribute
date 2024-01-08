package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/philippgille/gokv/bbolt"
	"github.com/philippgille/gokv/encoding"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const apps = "apps"

type AppInfo struct {
	VersionName string `json:"version_name"`
	VersionCode int    `json:"version_code"`
	Release     bool   `json:"release"`
	Timestamp   int64  `json:"timestamp"`
	Latest      bool   `json:"Latest"`
}

//go:embed ui/dist
var ui embed.FS
var dbPath = ""
var filePath = ""

func main() {
	port := flag.Int("port", 8089, "port")
	var filePaths = flag.String("upload_path", "/home/song/app/", "文件目录")
	var dbPathS = flag.String("db_path", "/home/song/db/", "数据库目录")
	flag.Parse()

	dbPath = *dbPathS
	filePath = *filePaths
	en := gin.Default()
	en.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, "pong")
	})
	en.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	v1 := en.Group("/v1/app")
	{
		// 查询正式版本和预览版本最大的版本号
		v1.GET("/official", func(context *gin.Context) {
			findApp(context, true)
		})
		v1.GET("/preview", func(context *gin.Context) {
			findApp(context, false)
		})
		// 下载正式版本和预览版本 最新的版本
		v1.GET("/:versionName", func(context *gin.Context) {
			handlerDownload(context)
		})
		// 设置正式版和与预览版最新的版本号码
		v1.PUT("/:versionName/official", func(context *gin.Context) {
			setLatest(context, true)
		})
		v1.PUT("/:versionName/preview", func(context *gin.Context) {
			setLatest(context, false)
		})
		// 上传apk
		v1.POST("/:versionName/:versionCode", func(c *gin.Context) {
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, err.Error())
				return
			}
			open, err := file.Open()
			if err != nil {
				c.JSON(http.StatusBadRequest, err.Error())
				return
			}
			versionName := c.Param("versionName")
			versionCode := c.Param("versionCode")

			// 创建目标文件
			destFile := filepath.Join(filePath, filepath.Base(versionName+".apk"))
			dest, err := os.Create(destFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			defer dest.Close()
			// 复制文件内容
			_, err = io.Copy(dest, open)
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			store, err := createStore()
			if err != nil {
				fmt.Println("创建数据库失败" + err.Error())
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			defer store.Close()
			infos := []AppInfo{}
			_, err = store.Get("apps", &infos)
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			versionC, err := strconv.Atoi(versionCode)
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
			isRelease := !ContainsLetter(versionName)
			// 可能存在
			var find = false
			for i := range infos {
				if infos[i].VersionName == versionName {
					infos[i].VersionCode = versionC
					find = true
					break
				}
			}
			if !find {
				infos = append(infos, AppInfo{VersionName: versionName, VersionCode: versionC, Release: isRelease, Timestamp: time.Now().Unix()})
			}
			store.Set("apps", infos)
			c.JSON(http.StatusOK, "ok")
		})
	}

	err := en.Run(fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
		return
	}
}

func ContainsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func setLatest(context *gin.Context, release bool) {
	store, err := createStore()
	if err != nil {
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	infos := []AppInfo{}
	found, err := store.Get(apps, &infos)
	if err != nil {
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		context.JSON(http.StatusNotFound, "not found")
		return
	}
	var find = false
	for i := range infos {
		if infos[i].Release == release && infos[i].VersionName == context.Param("versionName") {
			infos[i].Latest = true
			find = true
		} else {
			infos[i].Latest = false
		}
	}
	if find {
		store.Set(apps, infos)
		context.JSON(http.StatusOK, "ok")
	} else {
		context.JSON(http.StatusNotFound, "not found")
	}
}

func findApp(context *gin.Context, release bool) {
	store, err := createStore()
	if err != nil {
		fmt.Println("创建数据库失败" + err.Error())
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()
	infos := []AppInfo{}
	found, err := store.Get("apps", &infos)
	if err != nil {
		fmt.Println("查询数据库失败" + err.Error())
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		context.JSON(http.StatusNotFound, "not found")
		return
	}
	var info = AppInfo{VersionCode: 0}
	for i := range infos {
		if infos[i].Release == release {
			if infos[i].Latest {
				info = infos[i]
			}
		}
	}
	context.JSON(http.StatusOK, info)
}
func handlerDownload(c *gin.Context) {
	param := c.Param("versionName")
	filePath := "/path/to/file/"
	fileName := filePath + param + ".apk"

	file, err := os.Open(fileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	fileSize := fileInfo.Size()
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		ranges, err := parseRange(rangeHeader, fileSize)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(ranges) > 0 {
			start := ranges[0].start
			end := ranges[0].end

			// 设置响应头
			c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
			c.Status(http.StatusPartialContent)

			// 设置读取位置
			_, err = file.Seek(start, io.SeekStart)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// 创建部分读取的响应体
			reader := io.LimitReader(file, end-start+1)
			// 将部分文件内容写入响应体
			_, err = io.Copy(c.Writer, reader)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			return
		}
	}

	// 如果没有范围请求，发送完整文件
	c.Header("Content-Type", "application/vnd.android.package-archive")
	c.File(fileName)
}

// 解析范围请求头
func parseRange(rangeHeader string, fileSize int64) ([]Range, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, errors.New("Invalid Range header")
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeList := strings.Split(rangeStr, ",")

	ranges := make([]Range, 0, len(rangeList))

	for _, rangeItem := range rangeList {
		rangeParts := strings.Split(rangeItem, "-")
		if len(rangeParts) != 2 {
			return nil, errors.New("Invalid Range header")
		}

		start, err := strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil || start < 0 {
			return nil, errors.New("Invalid Range header")
		}

		var end int64
		if rangeParts[1] == "" {
			end = fileSize - 1
		} else {
			end, err = strconv.ParseInt(rangeParts[1], 10, 64)
			if err != nil || end < 0 || end >= fileSize {
				return nil, errors.New("Invalid Range header")
			}
		}

		if start > end {
			return nil, errors.New("Invalid Range header")
		}

		ranges = append(ranges, Range{start, end})
	}

	return ranges, nil
}

// 范围结构体
type Range struct {
	start int64
	end   int64
}

func createUIHanlder() http.Handler {
	fileFs, err := fs.Sub(ui, "ui/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(fileFs))
	return fileServer
}

func createStore() (*bbolt.Store, error) {
	path := dbPath + "app.db"
	_, err2 := os.Stat(path)
	if err2 != nil && os.IsNotExist(err2) {
		file, err3 := os.Create(path)
		if err3 != nil {
			return nil, err3
		}
		defer file.Close()
	}
	options := bbolt.Options{
		Path:  path,
		Codec: encoding.JSON,
	}
	store, err := bbolt.NewStore(options)
	if err != nil {
		return nil, err
	}
	return &store, nil
}
