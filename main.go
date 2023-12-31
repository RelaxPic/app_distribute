package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"strconv"
)

const rc = "rc"
const beta = "beta"
const aplha = "alpha"

type AppInfo struct {
	VersionName string `json:"version_name"`
	VersionCode int    `json:"version_code"`
	Release     bool   `json:"release"`
	timestamp   int64  `json:"timestamp"`
}

// 存储正式版和预览版的最新版本
type ReleaseInfo struct {
	appid   int  `json:"appid"`
	release bool `json:"release"`
}

func main() {
	port := flag.Int("port", 8080, "port")
	var dbHost = flag.String("db_host", "localhost", "数据库host")
	var dbDatabase = flag.String("db_database", "logs", "数据库名字")
	var dbUser = flag.String("db_user", "logs", "数据库用户")
	var dbPassword = flag.String("db_passwd", "logs", "数据库用户密码")
	var dbPort = flag.Int("db_port", 3306, "数据库端口")
	var filePath = flag.String("file_dir", "~/app/", "文件目录")
	flag.Parse()
	initDB(*dbHost, *dbDatabase, *dbUser, *dbPassword, *dbPort)
	en := gin.Default()
	en.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, "pong")
	})
	v1 := en.Group("/v1/apps")
	{
		// 查询最大的版本号
		v1.GET("/official", func(context *gin.Context) {
			findApp(context, true)
		})
		v1.GET("/preview", func(context *gin.Context) {
			findApp(context, false)
		})
		// 下载正式版本和预览版本
		v1.GET("/official/latest", func(context *gin.Context) {
			handlerDownload(context, filePath, true)
		})
		v1.GET("/preview/latest", func(context *gin.Context) {
			handlerDownload(context, filePath, false)
		})
		// 设置正式版和与预览版最新的版本
		v1.PUT("/:appid/official", func(context *gin.Context) {
			setLatest(context, true)
		})
		v1.PUT("/:appid/preview", func(context *gin.Context) {
			setLatest(context, false)
		})
		// 上传apk
		v1.POST("apps/:versionName/:versionCode", func(c *gin.Context) {
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, err.Error())
				return
			}
			versionName := c.Param("versionName")
			versionCode := c.Param("versionCode")
			fmt.Printf("versionname" + versionName)
			fmt.Printf("versioncode" + versionCode)
			c.JSON(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
		})
	}

	err := en.Run(fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
		return
	}
}

func setLatest(context *gin.Context, release bool) {
	getDb := GetDb()
	param := context.Param("appid")
	var query = "release is true"
	if !release {
		query = "release is false"
	}
	var re = &ReleaseInfo{}
	getDb.Model(&ReleaseInfo{}).Where(query).Find(re)
	if re.appid == 0 {
		getDb.Model(&ReleaseInfo{}).Where("release is true").Update("appid", param)
	} else {
		num, err := strconv.Atoi(param)
		if err != nil {
			context.Writer.Write([]byte("appid is not number"))
			return
		}
		getDb.Save(&ReleaseInfo{appid: (num), release: true})
	}
}

func findApp(context *gin.Context, release bool) {
	getDb := GetDb()
	info := AppInfo{}
	var query = "release is true"
	if !release {
		query = "release is false"
	}
	tx := getDb.Model(&AppInfo{}).Where(query).Order("version_code desc").Limit(1).Find(info)
	if tx.Error != nil {
		fmt.Println("查询最新的版本失败" + tx.Error.Error())
		context.JSON(http.StatusInternalServerError, tx.Error.Error())
		return
	}
	context.JSON(http.StatusOK, info)
}

func handlerDownload(context *gin.Context, filePath *string, release bool) {
	getDb := GetDb()
	var appInfo AppInfo
	var query = ""
	if release {
		query = "release is true"
	} else {
		query = "release is false"
	}
	getDb.Model(&AppInfo{}).Where(query).Order("version_code desc").Limit(1).Find(&appInfo)
	if appInfo.VersionName != "" {

		open, err := os.Open(*filePath + appInfo.VersionName + ".apk")
		if err != nil {
			fmt.Println("打开文件失败" + err.Error())
			context.Writer.WriteHeader(http.StatusNotFound)
			context.Writer.Write([]byte("not found"))
			return
		}
		all, err := io.ReadAll(open)
		if err != nil {
			fmt.Println("读取文件失败" + err.Error())
			context.Writer.WriteHeader(http.StatusNotFound)
			context.Writer.Write([]byte("not found"))
			return
		}
		_, err = context.Writer.Write(all)
		if err != nil {
			fmt.Println("写入文件失败" + err.Error())
		}
	} else {
		context.Writer.WriteHeader(http.StatusNotFound)
	}
}
