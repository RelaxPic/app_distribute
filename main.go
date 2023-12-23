package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

const rc = "rc"
const beta = "beta"
const aplha = "alpha"

type AppInfo struct {
	VersionName string `json:"version_name"`
	VersionCode int    `json:"version_code"`
}

func main() {
	port := flag.Int("port", 8080, "port")
	var dbHost = flag.String("db_host", "localhost", "数据库host")
	var dbDatabase = flag.String("db_database", "logs", "数据库名字")
	var dbUser = flag.String("db_user", "logs", "数据库用户")
	var dbPassword = flag.String("db_passwd", "logs", "数据库用户密码")
	var dbPort = flag.Int("db_port", 3306, "数据库端口")
	flag.Parse()
	initDB(*dbHost, *dbDatabase, *dbUser, *dbPassword, *dbPort)
	en := gin.Default()
	en.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, "pong")
	})
	v1 := en.Group("/v1/apps")
	{
		v1.GET("/official", func(context *gin.Context) {})
		v1.GET("/preview", func(context *gin.Context) {})
		v1.GET("/official/latest", func(context *gin.Context) {})
		v1.GET("/preview/latest", func(context *gin.Context) {})
		v1.PUT("/:appid/official", func(context *gin.Context) {})
		v1.PUT("/:appid/preview", func(context *gin.Context) {})
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
