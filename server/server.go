package server

import (
	"embed"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

//go:embed frontend/dist/*
var FS embed.FS

func Run() {
	var port = "27149"
	gin.SetMode(gin.DebugMode) //设置模式 ReleaseMode生产模式,DebugMode开发模式
	router := gin.Default()

	// router.GET("/", func(c *gin.Context) {
	// 	c.Writer.Write([]byte("abcd"))
	// })
	// 静态文件路由
	staticFiles, _ := fs.Sub(FS, "frontend/dist")
	router.StaticFS("/static", http.FS(staticFiles))

	router.POST("/api/v1/texts", TextsController)
	router.GET("/api/v1/addresses", AddressesController)
	router.GET("/uploads/:path", UploadsController)
	router.GET("/api/v1/qrcodes", QrcodesController)
	router.POST("/api/v1/files", FilesController)

	// 没有路由时,走这最后一个路由
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path               // 获取用户访问路径
		if strings.HasPrefix(path, "/static/") { // 以static开头的,说明用户想访问的是静态文件
			reader, err := staticFiles.Open("index.html") // 打开index.html
			if err != nil {
				log.Fatal(err)
			}
			defer reader.Close() // 读完index.html后关闭文件
			stat, err := reader.Stat()
			if err != nil {
				log.Fatal(err)
			}
			c.DataFromReader(http.StatusOK, stat.Size(), "text/html", reader, nil)
		} else {
			// 不是以static开头的,说明用户想访问的是动态文件
			c.Status(http.StatusNotFound)
		}
	})
	router.Run(":" + port)
}

//  实现接口1: 上传文件
/*
	思路:
	1,获取go执行文件(.exe文件)所在目录
	2,在该目录创建 uploads 目录
	3,将文本保存为一个文件
	4,返回该文件的下载路径
*/
func TextsController(c *gin.Context) {
	var json struct {
		Raw string
	}
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		// os.Getwd会输出实际的工作目录
		// os.Executable会输出一个临时文件的路径，毕竟os.Executable就是要返回当前运行的程序路径，
		// 所以会返回一个go run生成的临时文件路径
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		// filepath.Dir()函数用于返回指定路径中除最后一个元素以外的所有元素
		/*
			Dir返回路径除去最后一个路径元素的部分，即该路径最后一个元素所在的目录。在使用Split去掉最后一个元素后，会简化路径并去掉末尾的斜杠。如果路径是空字符串，会返回"."；
			如果路径由1到多个斜杠后跟0到多个非斜杠字符组成，会返回"/"；其他任何情况下都不会返回以斜杠结尾的路径。
			Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加斜杠。
			结果是经过简化的，所有的空字符串元素会被忽略。
		*/
		dir := filepath.Dir(exe)
		if err != nil {
			log.Fatal(err)
		}
		// uuid是谷歌开发的生成16字节UUID的模块
		filename := uuid.New().String()
		uploads := filepath.Join(dir, "uploads")
		// os.Mkdir	 创建目录
		// 初次创建dir时成功，再次创建dir时，如果path已经是一个目录，Mkdir会报错
		// os.MkdirAll  创建多级目录,如果path已经是一个目录，MkdirAll什么也不做，并返回nil
		// 必须分成两步：先创建文件夹、再修改权限
		err = os.MkdirAll(uploads, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		fullpath := path.Join("uploads", filename+".txt")
		err = ioutil.WriteFile(filepath.Join(dir, fullpath), []byte(json.Raw), 0644)
		if err != nil {
			log.Fatal(err)
		}

		/*
			exe:  "c:\\Users\\young\\Desktop\\lorcademo\\demo3\\__debug_bin.exe"
			dir: "c:\\Users\\young\\Desktop\\lorcademo\\demo3"
			filename: "79a89ddf-5025-4c75-9716-6405e01b37c2"
			uploads: "c:\\Users\\young\\Desktop\\lorcademo\\demo3\\uploads"
			fullpath: "uploads/79a89ddf-5025-4c75-9716-6405e01b37c2.txt"
		*/
		c.JSON(http.StatusOK, gin.H{"url": "/" + fullpath})
	}

	/*
		    获取当前目录
			os.Getwd()

			创建文件
			f1, _ := os.Create("./1.txt")
			defer f1.Close()

			以读写方式打开文件，如果不存在则创建文件，等同于上面os.Create
			f4, _ := os.OpenFile("./4.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			defer f4.Close()

			用os.path.join()连接两个文件名地址的时候，就比如
			os.path.join("D:\","test.txt")      \\结果是D:\test.txt

			删除指定目录下所有文件
			os.Remove("abc/d/e/f")

			删除指定目录
			os.RemoveAll("abc")

			重命名文件
			os.Rename("./2.txt", "./2_new.txt")
	*/

}

// 实现接口2: 获取局域网IP

/*
思路:
	1,获取电脑在各个局域网的IP地址
	2,转为JSON写入HTTP响应
*/
func AddressesController(c *gin.Context) {
	// InterfaceAddrs获取本地IP
	addrs, _ := net.InterfaceAddrs()
	var result []string
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		// address.类型断言,断言net是一个地址ip 是一个(*net.IPNet)类型
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				result = append(result, ipnet.IP.String())
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"addresses": result})
}

//
/* 文件下载
GET/uploads/:path
思路:
	1,将网络路径:path变成本地绝对路径
	2,读取本地文件,写到HTTP响应里
*/
func UploadsController(c *gin.Context) {
	if path := c.Param("path"); path != "" {
		target := filepath.Join(GetUploadsDir(), path)
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary") // 内容的编码:二进制
		c.Header("Content-Disposition", "attachment; filename="+path)
		c.Header("Content-Type", "application/octet-stream")
		c.File(target) // 给前端发送一个文件
	} else {
		c.Status(http.StatusNotFound)
	}
}

func GetUploadsDir() (uploads string) {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Dir(exe)
	if err != nil {
		log.Fatal(err)
	}
	uploads = filepath.Join(dir, "uploads")
	return
}

// 添加二维码接口
/*
GET /api/v1/qrcodes
思路:
	1. 获取文本内容
	2. 将文本转为图片 (用库 qrcode)
	3. 将图片写入HTTP响应
	x = http://ip1;27149/upload/... .txt
	http://ip1:27149/static/downloads?url=x

	GET /api/v1/qrcodes?content=http%3A%2F%2F192.168.244.1%3A27149%2Fstatic%2Fdownloads%3Ftype%3Dtext%26url%3Dhttp%3A%2F%2F192.168.244.1%3A27149%252Fuploads%252Fdfdd8bf3-1b64-40b7-ab12-ace74752f26e.txt"
*/
func QrcodesController(c *gin.Context) {
	if content := c.Query("content"); content != "" {
		png, err := qrcode.Encode(content, qrcode.Medium, 256)
		if err != nil {
			log.Fatal(err)
		}
		c.Data(http.StatusOK, "image/png", png)
	} else {
		c.Status(http.StatusBadRequest)
	}
}

// 上传文件
/*
POST /api/v1/files
跟POST /abi/v1/texts类似
区别在于保存文件而不是保存文本
思路:
1. 获取go执行文件所在目录
2. 在该目录创建uploads目录
3. 将文件保存为另一个文件看起来(多此一举)
4. 返回后者的下载路径
*/
func FilesController(c *gin.Context) {
	file, err := c.FormFile("raw")
	if err != nil {
		log.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Dir(exe)
	if err != nil {
		log.Fatal(err)
	}
	filename := uuid.New().String()
	uploads := filepath.Join(dir, "uploads")
	err = os.MkdirAll(uploads, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	fullpath := path.Join("uploads", filename+filepath.Ext(file.Filename))
	fileErr := c.SaveUploadedFile(file, filepath.Join(dir, fullpath))
	if fileErr != nil {
		log.Fatal(fileErr)
	}
	c.JSON(http.StatusOK, gin.H{"url": "/" + fullpath})
}