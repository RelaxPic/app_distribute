package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/philippgille/gokv/file"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const apps = "apps"

type AppInfo struct {
	VersionName string `json:"versionName"`
	VersionCode int    `json:"versionCode"`
	Release     bool   `json:"release"`
	Timestamp   int64  `json:"timestamp"`
	Latest      bool   `json:"latest"`
}

type ExternalAppInfo struct {
	VersionName  string `json:"versionName"`
	VersionCode  int    `json:"versionCode"`
	Release      bool   `json:"release"`
	Latest       bool   `json:"latest"`
	DownloadLink string `json:"downloadLink"`
}

//go:embed ui/dist
var ui embed.FS
var dbPath = ""
var filePath = ""
var domain = ""

func main() {
	port := flag.Int("port", 8089, "port")
	var filePaths = flag.String("upload_path", "~/app/", "文件目录")
	var dbPathS = flag.String("db_path", "~/db/", "数据库目录")
	var domainS = flag.String("app_domain", "https://test.com", "域名")
	var usernameS = flag.String("usernameS", "admin", "账号")
	var passwordS = flag.String("passwordS", "admin1995", "密码")
	flag.Parse()
	dbPath = *dbPathS
	filePath = *filePaths
	domain = *domainS
	en := gin.Default()
	auth := gin.BasicAuth(gin.Accounts{
		*usernameS: *passwordS,
	})
	en.MaxMultipartMemory = 1024 * 1024 * 40
	en.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, "pong")
	})
	en.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	v1 := en.Group("/v1/app")
	{
		// 获取最新正式版/预览般的信息
		v1.GET("/release", auth, findApp(true))
		v1.GET("/preview", auth, findApp(false))
		// 下载指定版本号应用
		v1.GET("/download/:versionName", handlerDownload(false))
		// 下载应用正式版本/预览版
		v1.GET("/download/release", handlerDownload(true))
		v1.GET("/download/preview", handlerDownload(false))
		// 设置指定版本为release
		v1.PATCH("/:versionName/latest", auth, setLatest())
		v1.POST("/:versionName/:versionCode", auth, uploadAPP())
	}
	err := en.Run(fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
		return
	}
}

func uploadAPP() func(*gin.Context) {
	return func(c *gin.Context) {
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
		c.JSON(http.StatusCreated, "ok")
	}
}

func setLatest() func(ctx *gin.Context) {
	return func(context *gin.Context) {
		store, err := createStore()
		if err != nil {
			resoJSONError(context, err)
			return
		}
		infos := []AppInfo{}
		found, err := store.Get(apps, &infos)
		if err != nil {
			resoJSONError(context, err)
			return
		}
		if !found {
			context.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		var find = false
		versionName := context.Param("versionName")
		var info AppInfo
		for i := range infos {
			if infos[i].VersionName == versionName {
				infos[i].Latest = true
				info = infos[i]
				find = true
				break
			}
		}
		for i := range infos {
			if infos[i].Release == info.Release {
				if infos[i].VersionName == versionName {
					infos[i].Latest = true
					find = true
				} else {
					infos[i].Latest = false
				}
			}
		}
		if find {
			store.Set(apps, infos)
			context.JSON(http.StatusCreated, gin.H{})
		} else {
			context.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		}
	}
}

func resoJSONError(context *gin.Context, err error) {
	context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func findApp(release bool) func(*gin.Context) {
	return func(context *gin.Context) {
		store, err := createStore()
		if err != nil {
			fmt.Println("创建数据库失败" + err.Error())
			context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer store.Close()
		infos := []AppInfo{}
		found, err := store.Get("apps", &infos)
		if err != nil {
			fmt.Println("查询数据库失败" + err.Error())
			context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found {
			context.JSON(http.StatusNotFound, gin.H{"error": "找不到应用apk"})
			return
		}
		var info = AppInfo{VersionCode: 0}
		var exist = false
		for i := range infos {
			if infos[i].Release == release {
				if infos[i].Latest {
					info = infos[i]
					exist = true
					break
				}
			}
		}
		if exist {
			var extAppInfo = ExternalAppInfo{
				VersionName:  info.VersionName,
				VersionCode:  info.VersionCode,
				Release:      info.Release,
				Latest:       info.Latest,
				DownloadLink: domain + info.VersionName + ".apk",
			}
			context.JSON(http.StatusOK, extAppInfo)
		} else {
			context.JSON(http.StatusNotFound, gin.H{})
		}
	}
}
func handlerDownload(relase bool) func(*gin.Context) {
	return func(c *gin.Context) {
		var downloadFilePath = ""
		var fileName = ""
		if relase {
			store, err := createStore()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			infos := []AppInfo{}
			found, err := store.Get("apps", &infos)
			if err != nil {
				fmt.Println("查询数据库失败" + err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if !found {
				c.JSON(http.StatusNotFound, gin.H{"error": "找不到应用apk"})
				return
			}
			var info = AppInfo{VersionCode: 0}
			var exist = false
			for i := range infos {
				if infos[i].Release && infos[i].Latest {
					if infos[i].Latest {
						info = infos[i]
						exist = true
						break
					}
				}
			}
			if !exist {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "not found"})
				return
			}
			fileName = info.VersionName + ".apk"
			downloadFilePath = filePath + fileName
		} else {
			param := c.Param("versionName")
			fileName = param + ".apk"
			downloadFilePath = filePath + fileName
		}

		file, err := os.Open(downloadFilePath)
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
		c.Header("Content-Type", "application/vnd.android.package-archive")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", "慢图浏览_"+fileName))
		c.File(filePath)
	}
}

func createUIHanlder() http.Handler {
	fileFs, err := fs.Sub(ui, "ui/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(fileFs))
	return fileServer
}

func createStore() (*file.Store, error) {
	_, err2 := os.Stat(dbPath)
	if err2 != nil && os.IsNotExist(err2) {
		file, err3 := os.Create(dbPath)
		if err3 != nil {
			return nil, err3
		}
		defer file.Close()
	}
	options := file.Options{
		Directory: dbPath,
	}
	store, err := file.NewStore(options)
	if err != nil {
		return nil, err
	}
	return &store, nil
}
