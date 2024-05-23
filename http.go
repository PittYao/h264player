package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/deepch/vdk/av"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// 路由
func ServeHTTP() {
	httpRouter := gin.Default()

	httpRouter.Use(Cors())
	httpRouter.GET("/ping", pong)

	// 流处理
	stream := httpRouter.Group("/stream")
	{
		stream.GET("/player/:uuid", StreamPlayer)
		stream.POST("/receiver/:uuid", StreamWebRTC)
		stream.GET("/codec/:uuid", StreamCodec)
		stream.POST("/register", StreamRegister)
	}

	// 静态文件代理
	httpRouter.StaticFS("/web", http.Dir("web/static"))

	// 启动web服务
	err := httpRouter.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatalln("启动web服务失败 ", err)
	}
}

// StreamPlayer stream player
func StreamPlayer(c *gin.Context) {
	_, all := Config.List()
	sort.Strings(all)
	c.HTML(http.StatusOK, "player.tmpl", gin.H{
		"port":     Config.Server.HTTPPort,
		"suuid":    c.Param("uuid"),
		"suuidMap": all,
		"version":  time.Now().String(),
	})
}

// StreamCodec stream codec
func StreamCodec(c *gin.Context) {
	if Config.Ext(c.Param("uuid")) {
		Config.RunIFNotRun(c.Param("uuid"))
		codecs := Config.CoGe(c.Param("uuid"))
		if codecs == nil {
			return
		}
		var tmpCodec []JCodec
		for _, codec := range codecs {
			if codec.Type() != av.H264 && codec.Type() != av.PCM_ALAW && codec.Type() != av.PCM_MULAW && codec.Type() != av.OPUS {
				log.Println("Codec Not Supported WebRTC ignore this track", codec.Type())
				continue
			}
			if codec.Type().IsVideo() {
				tmpCodec = append(tmpCodec, JCodec{Type: "video"})
			} else {
				tmpCodec = append(tmpCodec, JCodec{Type: "audio"})
			}
		}
		b, err := json.Marshal(tmpCodec)
		if err == nil {
			_, err = c.Writer.Write(b)
			if err != nil {
				log.Println("Write Codec Info error", err)
				return
			}
		}
	}
}

// StreamWebRTC stream video over WebRTC
func StreamWebRTC(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")
	var ssuid = ""
	var data = ""
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		ssuid = c.PostForm("suuid")
		data = c.PostForm("data")
	} else if strings.Contains(contentType, "application/json") {
		var receiverDTO ReceiverDTO
		if err := c.ShouldBindJSON(&receiverDTO); err != nil {
			log.Println(err)
			c.JSON(200, "传递参数异常")
			return
		}
		ssuid = receiverDTO.Suuid
		data = receiverDTO.Data
	}

	if !Config.Ext(ssuid) {
		log.Println("Stream Not Found")
		return
	}
	Config.RunIFNotRun(c.PostForm("suuid"))
	codecs := Config.CoGe(c.PostForm("suuid"))
	if codecs == nil {
		log.Println("Stream Codec Not Found")
		return
	}
	var AudioOnly bool
	if len(codecs) == 1 && codecs[0].Type().IsAudio() {
		AudioOnly = true
	}
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{ICEServers: Config.GetICEServers(), ICEUsername: Config.GetICEUsername(), ICECredential: Config.GetICECredential(), PortMin: Config.GetWebRTCPortMin(), PortMax: Config.GetWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, data)
	if err != nil {
		log.Println("WriteHeader", err)
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		log.Println("Write", err)
		return
	}
	go func() {
		cid, ch := Config.ClAd(c.PostForm("suuid"))
		defer Config.ClDe(c.PostForm("suuid"), cid)
		defer muxerWebRTC.Close()
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				log.Println("noVideo")
				return
			case pck := <-ch:
				if pck.IsKeyFrame || AudioOnly {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart && !AudioOnly {
					continue
				}
				err = muxerWebRTC.WritePacket(pck)
				if err != nil {
					log.Println("WritePacket", err)
					return
				}
			}
		}
	}()
}

// StreamRegister register
func StreamRegister(c *gin.Context) {
	var rtspUrlDTO RtspUrlDTO
	if err := c.ShouldBindJSON(&rtspUrlDTO); err != nil {
		log.Println(err)
		c.JSON(200, "rtspUrl地址异常")
		return
	}

	var responseDTO ResponseDTO
	log.Println("注册rtspUrl:", rtspUrlDTO.RtspUrl)

	// 为url生成唯一的id
	uuid := PseudoUUID()
	streamST := StreamST{
		URL:          rtspUrlDTO.RtspUrl,
		OnDemand:     true,
		Cl:           make(map[string]Viewer),
		DisableAudio: rtspUrlDTO.DisableAudio,
	}

	// 添加到配置中
	Config.Streams[uuid] = streamST
	RtspMap[rtspUrlDTO.RtspUrl] = uuid
	log.Println("配置流:", Config.Streams[uuid])

	c.JSON(200, responseDTO.SuccessWithData("注册成功，等待播放", uuid))
	return
}

func pong(c *gin.Context) {
	c.JSON(http.StatusOK, Success("pong"))
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		origin := c.Request.Header.Get("Origin") //请求头部
		if origin != "" {
			//接收客户端发送的origin （重要！）
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			//服务器支持的所有跨域请求的方法
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE")
			//允许跨域设置可以返回其他子段，可以自定义字段
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,Content-Type")
			// 允许浏览器（客户端）可以解析的头部 （重要）
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers")
			//设置缓存时间
			c.Header("Access-Control-Max-Age", "172800")
			//允许客户端传递校验信息比如 cookie (重要)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		//允许类型校验
		if method == "OPTIONS" {
			c.JSON(http.StatusOK, "ok!")
		}
		// -ldflags="-H windowsgui"

		c.Next()
	}
}

func PseudoUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return
}
